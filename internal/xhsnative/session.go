package xhsnative

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

type SessionManager struct {
	headless   bool
	binPath    string
	cookiePath string
}

type QRCodeLoginResult struct {
	AlreadyLoggedIn bool   `json:"already_logged_in"`
	SavedCookies    bool   `json:"saved_cookies"`
	QRCodeHTMLPath  string `json:"qrcode_html_path"`
	QRCodeDataURL   string `json:"qrcode_data_url,omitempty"`
	CookiePath      string `json:"cookie_path"`
}

func NewSessionManager(headless bool, binPath, cookiePath string) *SessionManager {
	return &SessionManager{
		headless:   headless,
		binPath:    binPath,
		cookiePath: cookiePath,
	}
}

func (m *SessionManager) CheckLoginStatus(ctx context.Context) (bool, error) {
	if err := m.applyCookiePath(); err != nil {
		return false, err
	}
	b, err := newBrowserInstance(m.headless, m.binPath, m.cookiePath)
	if err != nil {
		return false, err
	}
	defer b.Close()
	page := b.NewPage()
	defer page.Close()
	return xiaohongshu.NewLogin(page).CheckLoginStatus(ctx)
}

func (m *SessionManager) LoginWithQRCode(ctx context.Context, htmlPath string, wait time.Duration, onQRCodeWritten func(string)) (QRCodeLoginResult, error) {
	return m.LoginWithQRCodeData(ctx, htmlPath, wait, func(path, _ string) {
		if onQRCodeWritten != nil {
			onQRCodeWritten(path)
		}
	})
}

func (m *SessionManager) LoginWithQRCodeData(ctx context.Context, htmlPath string, wait time.Duration, onQRCodeWritten func(string, string)) (QRCodeLoginResult, error) {
	return m.LoginWithQRCodeDataOptions(ctx, htmlPath, wait, false, onQRCodeWritten)
}

func (m *SessionManager) LoginWithQRCodeDataOptions(ctx context.Context, htmlPath string, wait time.Duration, forceScan bool, onQRCodeWritten func(string, string)) (QRCodeLoginResult, error) {
	if err := m.applyCookiePath(); err != nil {
		return QRCodeLoginResult{}, err
	}
	if wait <= 0 {
		wait = 4 * time.Minute
	}
	if htmlPath == "" {
		htmlPath = filepath.Join("data", "login-qrcode.html")
	}

	b, err := newBrowserInstance(m.headless, m.binPath, m.cookiePath)
	if err != nil {
		return QRCodeLoginResult{}, err
	}
	defer b.Close()
	page := b.NewPage()
	defer page.Close()

	login := xiaohongshu.NewLogin(page)
	qrcode, loggedIn, err := login.FetchQrcodeImage(ctx)
	if err != nil {
		return QRCodeLoginResult{}, err
	}
	if !loggedIn {
		qrcode = captureQRCodeDataURL(ctx, page, qrcode)
	}
	result := QRCodeLoginResult{
		AlreadyLoggedIn: loggedIn,
		QRCodeHTMLPath:  htmlPath,
		QRCodeDataURL:   qrcode,
		CookiePath:      m.resolvedCookiePath(),
	}
	if loggedIn && !forceScan {
		if err := m.saveCookies(page); err != nil {
			return QRCodeLoginResult{}, err
		}
		result.SavedCookies = true
		return result, nil
	}
	if loggedIn && forceScan && qrcode == "" {
		return result, fmt.Errorf("小红书没有生成扫码二维码，请在后端打开的小红书页面完成验证后重试")
	}
	if err := WriteQRCodeHTML(htmlPath, qrcode); err != nil {
		return QRCodeLoginResult{}, err
	}
	if onQRCodeWritten != nil {
		onQRCodeWritten(htmlPath, qrcode)
	}

	waitCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	if !waitForQRCodeLogin(waitCtx, page) {
		return result, fmt.Errorf("login timed out after %s; open %s and scan the QR code before timeout", wait, htmlPath)
	}
	if err := m.saveCookies(page); err != nil {
		return QRCodeLoginResult{}, err
	}
	result.SavedCookies = true
	return result, nil
}

func captureQRCodeDataURL(ctx context.Context, page *rod.Page, fallback string) string {
	el := findQRCodeElement(ctx, page)
	if el == nil {
		return fallback
	}
	bin, err := el.Screenshot(proto.PageCaptureScreenshotFormatPng, 0)
	if err != nil || len(bin) == 0 {
		return fallback
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(bin)
}

func findQRCodeElement(ctx context.Context, page *rod.Page) *rod.Element {
	pp := page.Context(ctx)
	selectors := []string{
		".login-container .qrcode-img",
		".login-container img",
		"[class*='qrcode'] img",
		"[class*='qr-code'] img",
		"[class*='qrCode'] img",
		"[class*='qrcode'] canvas",
		"[class*='qr-code'] canvas",
		"[class*='qrCode'] canvas",
		".qrcode-img",
		"img[src*='qrcode']",
	}
	for _, selector := range selectors {
		exists, el, err := pp.Has(selector)
		if err == nil && exists && el != nil {
			return el
		}
	}
	return nil
}

func waitForQRCodeLogin(ctx context.Context, page *rod.Page) bool {
	pp := page.Context(ctx)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			qrExists, _, _ := pp.Has(".login-container .qrcode-img")
			loggedIn, _, _ := pp.Has(".main-container .user .link-wrapper .channel")
			if loggedIn && !qrExists {
				return true
			}
		}
	}
}

func waitForQRCodeResolved(ctx context.Context, page *rod.Page) bool {
	pp := page.Context(ctx)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if findQRCodeElement(ctx, page) == nil {
				return true
			}
			loggedIn, _, _ := pp.Has(".main-container .user .link-wrapper .channel")
			if loggedIn {
				return true
			}
		}
	}
}

func WriteQRCodeHTML(path, qrcodeDataURL string) error {
	if path == "" {
		return fmt.Errorf("html path is required")
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	content := `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>小红书登录二维码</title>
  <style>
    body { font-family: sans-serif; margin: 40px; }
    img { width: 280px; height: 280px; }
  </style>
</head>
<body>
  <h1>小红书登录二维码</h1>
  <p>请使用小红书 App 扫码登录。登录成功后，命令会保存 cookies。</p>
  <img alt="小红书登录二维码" src="` + html.EscapeString(qrcodeDataURL) + `">
</body>
</html>
`
	return os.WriteFile(path, []byte(content), 0o644)
}

func (m *SessionManager) applyCookiePath() error {
	path := m.resolvedCookiePath()
	if path == "" {
		return nil
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.Setenv("COOKIES_PATH", path)
}

func (m *SessionManager) resolvedCookiePath() string {
	return resolvedCookiePath(m.cookiePath)
}

func (m *SessionManager) saveCookies(page *rod.Page) error {
	return savePageCookies(page, m.resolvedCookiePath())
}

func savePageCookies(page *rod.Page, path string) error {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return err
	}
	data, err := json.Marshal(cks)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o600)
}
