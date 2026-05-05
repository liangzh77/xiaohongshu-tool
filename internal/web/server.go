package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"xiaohongshu-tool/internal/analyzer"
	"xiaohongshu-tool/internal/collector"
	"xiaohongshu-tool/internal/draftgen"
	"xiaohongshu-tool/internal/keydist"
	"xiaohongshu-tool/internal/reviewer"
	"xiaohongshu-tool/internal/scorer"
	"xiaohongshu-tool/internal/storage"
	"xiaohongshu-tool/internal/xhsnative"
)

type Config struct {
	DB             *storage.DB
	DefaultLimit   int
	CollectorCmd   string
	LLMBaseURL     string
	LLMModel       string
	KeyName        string
	KeyDistBaseURL string
}

type Server struct {
	db           *storage.DB
	defaultLimit int
	collectorCmd string
	llmBaseURL   string
	llmModel     string
	keyName      string
	mu           sync.RWMutex
	keyConfig    KeyConfig
	loginState   LoginState
	xhsCollector *xhsnative.Collector
}

type KeyConfig struct {
	BaseURL       string   `json:"base_url"`
	Username      string   `json:"username"`
	Password      string   `json:"password,omitempty"`
	KeyName       string   `json:"key_name"`
	AvailableKeys []string `json:"available_keys,omitempty"`
}

type LoginState struct {
	Status          string `json:"status"`
	Message         string `json:"message"`
	QRCodeDataURL   string `json:"qrcode_data_url,omitempty"`
	QRCodeHTMLPath  string `json:"qrcode_html_path,omitempty"`
	CookiePath      string `json:"cookie_path,omitempty"`
	StartedAt       string `json:"started_at,omitempty"`
	FinishedAt      string `json:"finished_at,omitempty"`
	SavedCookies    bool   `json:"saved_cookies"`
	AlreadyLoggedIn bool   `json:"already_logged_in"`
}

type StateResponse struct {
	Targets    []storage.Target              `json:"targets"`
	Runs       []storage.Run                 `json:"runs"`
	RunDetails []storage.RunDetail           `json:"run_details"`
	Items      []storage.StoredItem          `json:"items"`
	Analyses   []storage.NoteAnalysis        `json:"analyses"`
	Candidates []storage.TopicCandidate      `json:"candidates"`
	Drafts     []storage.GeneratedDraft      `json:"drafts"`
	Publishes  []storage.PublishRecord       `json:"publishes"`
	Snapshots  []storage.PerformanceSnapshot `json:"snapshots"`
	Reports    []storage.PerformanceReport   `json:"reports"`
	Config     PublicConfig                  `json:"config"`
}

type PublicConfig struct {
	LLMBaseURL        string   `json:"llm_base_url"`
	LLMModel          string   `json:"llm_model"`
	KeyName           string   `json:"key_name"`
	KeyDistConfigured bool     `json:"key_dist_configured"`
	KeyDistBaseURL    string   `json:"key_dist_base_url"`
	KeyDistUsername   string   `json:"key_dist_username"`
	AvailableKeys     []string `json:"available_keys"`
}

func NewServer(cfg Config) *Server {
	limit := cfg.DefaultLimit
	if limit <= 0 {
		limit = 20
	}
	llmBaseURL := firstNonEmpty(cfg.LLMBaseURL, os.Getenv("XHS_LLM_BASE_URL"), "https://api.openai.com/v1")
	llmModel := firstNonEmpty(cfg.LLMModel, os.Getenv("XHS_LLM_MODEL"))
	keyName := firstNonEmpty(cfg.KeyName, os.Getenv("XHS_LLM_KEY_NAME"), "OPENAI_API_KEY")
	return &Server{
		db:           cfg.DB,
		defaultLimit: limit,
		collectorCmd: cfg.CollectorCmd,
		llmBaseURL:   llmBaseURL,
		llmModel:     llmModel,
		keyName:      keyName,
		keyConfig: KeyConfig{
			BaseURL: cfg.KeyDistBaseURL,
			KeyName: keyName,
		},
		xhsCollector: xhsnative.NewCollector(false, os.Getenv("ROD_BROWSER_BIN")),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/favicon.ico", s.handleFavicon)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/targets", s.handleTargets)
	mux.HandleFunc("/api/targets/delete", s.handleTargetDelete)
	mux.HandleFunc("/api/collect/once", s.handleCollectOnce)
	mux.HandleFunc("/api/analyze/batch", s.handleAnalyzeBatch)
	mux.HandleFunc("/api/score/batch", s.handleScoreBatch)
	mux.HandleFunc("/api/draft/batch", s.handleDraftBatch)
	mux.HandleFunc("/api/publish", s.handlePublish)
	mux.HandleFunc("/api/review/snapshot", s.handleReviewSnapshot)
	mux.HandleFunc("/api/review/score", s.handleReviewScore)
	mux.HandleFunc("/api/key-config", s.handleKeyConfig)
	mux.HandleFunc("/api/key-config/test", s.handleKeyConfigTest)
	mux.HandleFunc("/api/xhs-login/qrcode", s.handleXHSLoginQRCode)
	mux.HandleFunc("/api/xhs-login/status", s.handleXHSLoginStatus)
	mux.HandleFunc("/api/xhs-login/logout", s.handleXHSLogout)
	return withSecurityHeaders(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="7" fill="#1967d2"/><path d="M9 10h14v3H9zm0 6h14v3H9zm0 6h9v3H9z" fill="white"/></svg>`))
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	limit := queryLimit(r, s.defaultLimit)
	ctx := r.Context()
	targets, err := s.db.ListTargets(ctx, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	runs, err := s.db.ListRuns(ctx, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	runDetails, err := s.db.ListRunDetails(ctx, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	items, err := s.db.ListItems(ctx, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	analyses, err := s.db.ListNoteAnalyses(ctx, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	candidates, err := s.db.ListTopicCandidates(ctx, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	drafts, err := s.db.ListGeneratedDrafts(ctx, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	publishes, err := s.db.ListPublishRecords(ctx, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	snapshots, err := s.db.ListPerformanceSnapshots(ctx, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	reports, err := s.db.ListPerformanceReports(ctx, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, StateResponse{
		Targets:    targets,
		Runs:       runs,
		RunDetails: runDetails,
		Items:      items,
		Analyses:   analyses,
		Candidates: candidates,
		Drafts:     drafts,
		Publishes:  publishes,
		Snapshots:  snapshots,
		Reports:    reports,
		Config:     s.publicConfig(),
	})
}

func (s *Server) handleTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		ID       int64  `json:"id"`
		Kind     string `json:"kind"`
		Name     string `json:"name"`
		URL      string `json:"url"`
		Keyword  string `json:"keyword"`
		Interval int    `json:"interval"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ID > 0 {
		err := s.db.UpdateTarget(r.Context(), storage.Target{
			ID:                 req.ID,
			Kind:               req.Kind,
			Name:               req.Name,
			URL:                req.URL,
			Keyword:            req.Keyword,
			MinIntervalSeconds: req.Interval,
			Enabled:            true,
		})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, map[string]any{"id": req.ID})
		return
	}
	id, err := s.db.AddTarget(r.Context(), storage.Target{
		Kind:               req.Kind,
		Name:               req.Name,
		URL:                req.URL,
		Keyword:            req.Keyword,
		MinIntervalSeconds: req.Interval,
		Enabled:            true,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"id": id})
}

func (s *Server) handleTargetDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.db.DeleteTarget(r.Context(), req.ID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"id": req.ID, "deleted": true})
}

func (s *Server) handleCollectOnce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		TargetIDs []int64 `json:"target_ids"`
		ItemLimit int     `json:"item_limit"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.TargetIDs) == 0 {
		writeError(w, fmt.Errorf("请选择采集目标"))
		return
	}
	targets, err := s.selectedTargets(r.Context(), req.TargetIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(targets) == 0 {
		writeError(w, fmt.Errorf("没有找到可用采集目标"))
		return
	}
	summary, err := s.runLocalBrowserCollection(r.Context(), targets, req.ItemLimit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, summary)
}

func (s *Server) runLocalBrowserCollection(ctx context.Context, targets []storage.Target, itemLimit int) (collector.Summary, error) {
	if itemLimit <= 0 {
		itemLimit = 3
	}
	if itemLimit > 20 {
		itemLimit = 20
	}
	summary := collector.Summary{Runs: make([]collector.RunSummary, 0, len(targets))}
	for _, target := range targets {
		startedAt := time.Now()
		runID, err := s.db.StartRun(ctx, target.ID, "local_browser_search", startedAt)
		if err != nil {
			return summary, err
		}
		keyword := firstNonEmpty(target.Keyword, target.Name)
		if keyword == "" {
			_ = s.db.FinishRun(ctx, runID, "failed", "关键词为空", time.Now())
			continue
		}
		progressLogs := []string{}
		updateProgress := func(line string) {
			progressLogs = append(progressLogs, line)
			_ = s.db.UpdateRunMessage(context.Background(), runID, strings.Join(progressLogs, "\n"))
		}
		result, err := s.xhsCollector.SearchThenOpenDetails(ctx, xhsnative.NaturalSearchOptions{
			Keyword:      keyword,
			Limit:        itemLimit,
			LoadComments: true,
			DelayMin:     2 * time.Second,
			DelayMax:     5 * time.Second,
			Exists: func(ctx context.Context, externalID string) (bool, error) {
				return s.db.ItemHasCompleteDetailByExternalID(ctx, externalID)
			},
			Log: updateProgress,
		})
		logs := result.Logs
		if err != nil {
			if len(result.Items) > 0 {
				saved, saveErr := s.db.SaveItemsForRun(ctx, runID, target.ID, result.Items, time.Now())
				if saveErr != nil {
					logs = append(logs, "暂停前保存失败："+saveErr.Error())
				} else {
					logs = append(logs, fmt.Sprintf("暂停前已保存 %d 条内容", len(saved)))
				}
			}
			logs = append(logs, "采集失败："+err.Error())
			status := "failed"
			if xhsnative.IsQRCodeLoginRequired(err) {
				status = "needs_login"
				logs = append(logs, "已暂停：等待前端扫码登录成功后自动重试本次采集请求")
			}
			_ = s.db.FinishRun(ctx, runID, status, strings.Join(logs, "\n"), time.Now())
			if xhsnative.IsQRCodeLoginRequired(err) {
				return summary, err
			}
			continue
		}
		saved, err := s.db.SaveItemsForRun(ctx, runID, target.ID, result.Items, time.Now())
		if err != nil {
			logs = append(logs, "保存失败："+err.Error())
			_ = s.db.FinishRun(ctx, runID, "failed", strings.Join(logs, "\n"), time.Now())
			continue
		}
		titles := make([]string, 0, len(saved))
		for _, item := range saved {
			titles = append(titles, item.Title)
		}
		logs = append(logs, fmt.Sprintf("collected_items=%d skipped_existing=%d detail_failed=%d", len(saved), result.Skipped, result.Failed))
		_ = s.db.FinishRun(ctx, runID, "succeeded", strings.Join(logs, "\n"), time.Now())
		summary.TargetCount++
		summary.ItemCount += len(saved)
		summary.Runs = append(summary.Runs, collector.RunSummary{
			RunID:      runID,
			TargetID:   target.ID,
			TargetName: target.Name,
			ItemCount:  len(saved),
			Titles:     titles,
		})
	}
	return summary, nil
}

func (s *Server) handleXHSLoginQRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Force bool `json:"force"`
		Auto  bool `json:"auto"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	startedAt := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	if s.loginState.Status == "starting" || s.loginState.Status == "waiting" {
		state := s.loginState
		s.mu.Unlock()
		writeJSON(w, state)
		return
	}
	s.loginState = LoginState{Status: "starting", Message: "正在生成登录二维码", StartedAt: startedAt, CookiePath: "data/cookies.json"}
	s.mu.Unlock()

	qrReady := make(chan struct{}, 1)
	if req.Force {
		go s.runXHSPersistentPageQRCodeLogin(qrReady, req.Auto)
	} else {
		go s.runXHSQRCodeLogin(qrReady, false)
	}

	select {
	case <-qrReady:
	case <-time.After(45 * time.Second):
	}
	writeJSON(w, s.currentLoginState())
}

func (s *Server) handleXHSLoginStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, s.currentLoginState())
}

func (s *Server) handleXHSLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if err := os.Remove("data/cookies.json"); err != nil && !os.IsNotExist(err) {
		writeError(w, err)
		return
	}
	if s.xhsCollector != nil {
		s.xhsCollector.Close()
	}
	s.xhsCollector = xhsnative.NewCollector(false, os.Getenv("ROD_BROWSER_BIN"))
	s.mu.Lock()
	s.loginState = LoginState{Status: "idle", Message: "已登出，cookies 已清除", CookiePath: "data/cookies.json"}
	s.mu.Unlock()
	writeJSON(w, s.currentLoginState())
}

func (s *Server) runXHSPersistentPageQRCodeLogin(qrReady chan<- struct{}, fromAuthBlock bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	var qrcode string
	var err error
	if fromAuthBlock {
		qrcode, err = s.xhsCollector.CurrentQRCodeDataURL(ctx)
	}
	if qrcode == "" {
		if loggedIn, checkErr := s.xhsCollector.CheckPersistentLoginStatus(ctx); checkErr == nil && loggedIn {
			s.mu.Lock()
			s.loginState.Status = "succeeded"
			s.loginState.Message = "当前小红书登录态可用，没有生成二维码"
			s.loginState.QRCodeDataURL = ""
			s.loginState.CookiePath = "data/cookies.json"
			s.loginState.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			s.mu.Unlock()
			select {
			case qrReady <- struct{}{}:
			default:
			}
			return
		}
		qrcode, err = s.xhsCollector.OpenLoginQRCodeDataURL(ctx)
	}
	if err != nil {
		s.mu.Lock()
		s.loginState.Status = "failed"
		s.loginState.Message = err.Error()
		s.loginState.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		s.mu.Unlock()
		select {
		case qrReady <- struct{}{}:
		default:
		}
		return
	}
	s.mu.Lock()
	s.loginState.Status = "waiting"
	s.loginState.Message = "请使用小红书 App 扫码完成当前页面验证"
	s.loginState.QRCodeDataURL = qrcode
	s.loginState.QRCodeHTMLPath = ""
	s.loginState.CookiePath = "data/cookies.json"
	s.mu.Unlock()
	select {
	case qrReady <- struct{}{}:
	default:
	}

	err = s.xhsCollector.WaitForCurrentQRCodeResolved(ctx, "data/cookies.json")
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loginState.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		s.loginState.Status = "failed"
		s.loginState.Message = err.Error()
		return
	}
	s.loginState.Status = "succeeded"
	s.loginState.Message = "扫码验证完成，cookies 已保存"
	s.loginState.CookiePath = "data/cookies.json"
	s.loginState.SavedCookies = true
	select {
	case qrReady <- struct{}{}:
	default:
	}
}

func (s *Server) runXHSQRCodeLogin(qrReady chan<- struct{}, forceScan bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	manager := xhsnative.NewSessionManager(true, "", "data/cookies.json")
	result, err := manager.LoginWithQRCodeDataOptions(ctx, "data/login-qrcode.html", 4*time.Minute, forceScan, func(path, qrcode string) {
		s.mu.Lock()
		s.loginState.Status = "waiting"
		s.loginState.Message = "请使用小红书 App 扫码登录"
		s.loginState.QRCodeHTMLPath = path
		s.loginState.QRCodeDataURL = qrcode
		s.loginState.CookiePath = "data/cookies.json"
		s.mu.Unlock()
		select {
		case qrReady <- struct{}{}:
		default:
		}
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loginState.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		if s.loginState.Status == "starting" {
			select {
			case qrReady <- struct{}{}:
			default:
			}
		}
		s.loginState.Status = "failed"
		s.loginState.Message = err.Error()
		return
	}
	s.loginState.Status = "succeeded"
	s.loginState.Message = "登录成功，cookies 已保存"
	s.loginState.QRCodeDataURL = result.QRCodeDataURL
	s.loginState.QRCodeHTMLPath = result.QRCodeHTMLPath
	s.loginState.CookiePath = result.CookiePath
	s.loginState.SavedCookies = result.SavedCookies
	s.loginState.AlreadyLoggedIn = result.AlreadyLoggedIn
	if forceScan && s.xhsCollector != nil {
		s.xhsCollector.Close()
		s.xhsCollector = xhsnative.NewCollector(false, os.Getenv("ROD_BROWSER_BIN"))
	}
	select {
	case qrReady <- struct{}{}:
	default:
	}
}

func (s *Server) currentLoginState() LoginState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.loginState.Status == "" {
		return LoginState{Status: "idle", Message: "未开始登录", CookiePath: "data/cookies.json"}
	}
	state := s.loginState
	state.StartedAt = storage.FormatBeijingTime(state.StartedAt)
	state.FinishedAt = storage.FormatBeijingTime(state.FinishedAt)
	return state
}

func (s *Server) selectedTargets(ctx context.Context, ids []int64) ([]storage.Target, error) {
	all, err := s.db.ListTargets(ctx, 1000)
	if err != nil {
		return nil, err
	}
	want := make(map[int64]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	targets := make([]storage.Target, 0, len(ids))
	for _, target := range all {
		if want[target.ID] {
			targets = append(targets, target)
		}
	}
	return targets, nil
}

func (s *Server) collectorCommand(itemLimit int) string {
	if itemLimit <= 0 {
		itemLimit = 3
	}
	if itemLimit > 50 {
		itemLimit = 50
	}
	command := firstNonEmpty(s.collectorCmd, "go run ./cmd/xhs-native-collector")
	return fmt.Sprintf("%s --limit %d --details=true", command, itemLimit)
}

func (s *Server) handleAnalyzeBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req actionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	items, err := s.db.ListItems(r.Context(), requestLimit(req.Limit, s.defaultLimit))
	if err != nil {
		writeError(w, err)
		return
	}
	apiKey, err := s.apiKeyForEngine(r.Context(), req.Engine)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	analyses := make([]storage.NoteAnalysis, 0, len(items))
	for _, item := range items {
		analysis, err := s.analyze(ctx, item, req.Engine, apiKey)
		if err != nil {
			writeError(w, err)
			return
		}
		id, err := s.db.SaveNoteAnalysis(r.Context(), analysis)
		if err != nil {
			writeError(w, err)
			return
		}
		analysis.ID = id
		analyses = append(analyses, analysis)
	}
	writeJSON(w, analyses)
}

func (s *Server) handleScoreBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req actionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	analyses, err := s.db.ListNoteAnalyses(r.Context(), requestLimit(req.Limit, s.defaultLimit))
	if err != nil {
		writeError(w, err)
		return
	}
	apiKey, err := s.apiKeyForEngine(r.Context(), req.Engine)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	candidates := make([]storage.TopicCandidate, 0, len(analyses))
	for _, analysis := range analyses {
		candidate, err := s.score(ctx, analysis, req.Engine, apiKey)
		if err != nil {
			writeError(w, err)
			return
		}
		id, err := s.db.SaveTopicCandidate(r.Context(), candidate)
		if err != nil {
			writeError(w, err)
			return
		}
		candidate.ID = id
		candidates = append(candidates, candidate)
	}
	writeJSON(w, candidates)
}

func (s *Server) handleDraftBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req actionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	candidates, err := s.db.ListTopicCandidates(r.Context(), requestLimit(req.Limit, s.defaultLimit))
	if err != nil {
		writeError(w, err)
		return
	}
	apiKey, err := s.apiKeyForEngine(r.Context(), req.Engine)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	drafts := make([]storage.GeneratedDraft, 0, len(candidates))
	for _, candidate := range candidates {
		draft, err := s.draft(ctx, candidate, req.Engine, apiKey)
		if err != nil {
			writeError(w, err)
			return
		}
		id, err := s.db.SaveGeneratedDraft(r.Context(), draft)
		if err != nil {
			writeError(w, err)
			return
		}
		draft.ID = id
		drafts = append(drafts, draft)
	}
	writeJSON(w, drafts)
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req storage.PublishRecord
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.PublishedAt == "" {
		req.PublishedAt = time.Now().Format(time.RFC3339)
	}
	id, err := s.db.SavePublishRecord(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	req.ID = id
	writeJSON(w, req)
}

func (s *Server) handleReviewSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req storage.PerformanceSnapshot
	if !decodeJSON(w, r, &req) {
		return
	}
	id, err := s.db.SavePerformanceSnapshot(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	req.ID = id
	writeJSON(w, req)
}

func (s *Server) handleReviewScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Limit int `json:"limit"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	snapshots, err := s.db.ListPerformanceSnapshots(r.Context(), requestLimit(req.Limit, s.defaultLimit))
	if err != nil {
		writeError(w, err)
		return
	}
	ruleReviewer := reviewer.NewRuleReviewer()
	reports := make([]storage.PerformanceReport, 0, len(snapshots))
	for _, snapshot := range snapshots {
		report := ruleReviewer.Review(snapshot)
		id, err := s.db.SavePerformanceReport(r.Context(), report)
		if err != nil {
			writeError(w, err)
			return
		}
		report.ID = id
		reports = append(reports, report)
	}
	writeJSON(w, reports)
}

func (s *Server) handleKeyConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req KeyConfig
	if !decodeJSON(w, r, &req) {
		return
	}
	s.mu.Lock()
	current := s.keyConfig
	if req.Password == "" {
		req.Password = current.Password
	}
	if req.BaseURL == "" {
		req.BaseURL = current.BaseURL
	}
	if req.Username == "" {
		req.Username = current.Username
	}
	if len(req.AvailableKeys) == 0 {
		req.AvailableKeys = current.AvailableKeys
	}
	if req.KeyName != "" {
		s.keyName = req.KeyName
	}
	s.keyConfig = req
	s.mu.Unlock()
	writeJSON(w, s.publicConfig())
}

func (s *Server) handleKeyConfigTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req KeyConfig
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.BaseURL == "" || req.Username == "" || req.Password == "" {
		writeError(w, fmt.Errorf("密钥分发服务、用户名、密码都必须填写"))
		return
	}
	token, err := keydist.Client{BaseURL: req.BaseURL}.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	client := keydist.Client{BaseURL: req.BaseURL}
	keys, err := client.ListKeys(r.Context(), token)
	if err != nil {
		writeError(w, err)
		return
	}
	keyNames := make([]string, 0, len(keys))
	for _, key := range keys {
		keyNames = append(keyNames, key.KeyName)
	}
	keyName := firstNonEmpty(req.KeyName, s.keyName)
	if len(keyNames) > 0 && !containsString(keyNames, keyName) {
		keyName = keyNames[0]
	}
	value, err := client.GetKey(r.Context(), token, keyName)
	if err != nil {
		writeError(w, err)
		return
	}
	req.KeyName = keyName
	req.AvailableKeys = keyNames
	s.mu.Lock()
	s.keyConfig = req
	s.keyName = keyName
	s.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true, "key_name": keyName, "available_keys": keyNames, "key_present": value != ""})
}

type actionRequest struct {
	Engine string `json:"engine"`
	Limit  int    `json:"limit"`
}

func (s *Server) analyze(ctx context.Context, item storage.StoredItem, engine, apiKey string) (storage.NoteAnalysis, error) {
	switch normalizedEngine(engine) {
	case "rule":
		return analyzer.NewRuleAnalyzer().Analyze(item), nil
	case "llm":
		return analyzer.LLMAnalyzer{BaseURL: s.llmBaseURL, APIKey: apiKey, Model: s.llmModel}.Analyze(ctx, item)
	default:
		return storage.NoteAnalysis{}, fmt.Errorf("unsupported analyzer engine %q", engine)
	}
}

func (s *Server) score(ctx context.Context, analysis storage.NoteAnalysis, engine, apiKey string) (storage.TopicCandidate, error) {
	switch normalizedEngine(engine) {
	case "rule":
		return scorer.NewRuleScorer().Score(analysis), nil
	case "llm":
		return scorer.LLMScorer{BaseURL: s.llmBaseURL, APIKey: apiKey, Model: s.llmModel}.Score(ctx, analysis)
	default:
		return storage.TopicCandidate{}, fmt.Errorf("unsupported scoring engine %q", engine)
	}
}

func (s *Server) draft(ctx context.Context, candidate storage.TopicCandidate, engine, apiKey string) (storage.GeneratedDraft, error) {
	switch normalizedEngine(engine) {
	case "rule":
		return draftgen.NewRuleGenerator().Generate(candidate), nil
	case "llm":
		return draftgen.LLMGenerator{BaseURL: s.llmBaseURL, APIKey: apiKey, Model: s.llmModel}.Generate(ctx, candidate)
	default:
		return storage.GeneratedDraft{}, fmt.Errorf("unsupported draft engine %q", engine)
	}
}

func (s *Server) apiKeyForEngine(ctx context.Context, engine string) (string, error) {
	if normalizedEngine(engine) != "llm" {
		return "", nil
	}
	if s.llmModel == "" {
		return "", fmt.Errorf("LLM model is required; set XHS_LLM_MODEL before starting the server")
	}
	s.mu.RLock()
	cfg := s.keyConfig
	s.mu.RUnlock()
	if cfg.BaseURL == "" || cfg.Username == "" || cfg.Password == "" {
		return "", fmt.Errorf("请先在页面填写并测试密钥分发服务、用户名、密码")
	}
	keyName := firstNonEmpty(cfg.KeyName, s.keyName)
	client := keydist.Client{BaseURL: cfg.BaseURL}
	token, err := client.Login(ctx, cfg.Username, cfg.Password)
	if err != nil {
		return "", err
	}
	return client.GetKey(ctx, token, keyName)
}

func (s *Server) publicConfig() PublicConfig {
	s.mu.RLock()
	cfg := s.keyConfig
	s.mu.RUnlock()
	return PublicConfig{
		LLMBaseURL:        s.llmBaseURL,
		LLMModel:          s.llmModel,
		KeyName:           firstNonEmpty(cfg.KeyName, s.keyName),
		KeyDistConfigured: cfg.BaseURL != "" && cfg.Username != "" && cfg.Password != "",
		KeyDistBaseURL:    cfg.BaseURL,
		KeyDistUsername:   cfg.Username,
		AvailableKeys:     s.availableKeysFromConfig(cfg),
	}
}

func (s *Server) availableKeysFromConfig(cfg KeyConfig) []string {
	if len(cfg.AvailableKeys) > 0 {
		return append([]string(nil), cfg.AvailableKeys...)
	}
	if cfg.KeyName != "" {
		return []string{cfg.KeyName}
	}
	return nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeError(w, err)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func queryLimit(r *http.Request, fallback int) int {
	value, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return requestLimit(value, fallback)
}

func requestLimit(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizedEngine(engine string) string {
	if strings.TrimSpace(engine) == "" {
		return "rule"
	}
	return strings.TrimSpace(engine)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
