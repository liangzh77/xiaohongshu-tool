package xhsnative

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/sirupsen/logrus"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

type browserInstance struct {
	browser  *rod.Browser
	launcher *launcher.Launcher
}

func newBrowserInstance(headless bool, binPath, cookiePath string) (*browserInstance, error) {
	l := launcher.New().
		Headless(headless).
		Leakless(false).
		NoSandbox(true).
		Set("user-agent", defaultUserAgent)

	if binPath != "" {
		l = l.Bin(binPath)
	}
	if proxy := os.Getenv("XHS_PROXY"); proxy != "" {
		l = l.Proxy(proxy)
	}

	url, err := l.Launch()
	if err != nil {
		return nil, err
	}
	b := rod.New().ControlURL(url)
	if err := b.Connect(); err != nil {
		l.Cleanup()
		return nil, err
	}

	if err := loadCookies(b, cookiePath); err != nil {
		logrus.Warnf("failed to load cookies: %v", err)
	}
	return &browserInstance{browser: b, launcher: l}, nil
}

func (b *browserInstance) Close() {
	if b == nil {
		return
	}
	if b.browser != nil {
		_ = b.browser.Close()
	}
	if b.launcher != nil {
		b.launcher.Cleanup()
	}
}

func (b *browserInstance) NewPage() *rod.Page {
	return stealth.MustPage(b.browser)
}

func loadCookies(b *rod.Browser, cookiePath string) error {
	path := resolvedCookiePath(cookiePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var cookies []*proto.NetworkCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return err
	}
	b.MustSetCookies(cookies...)
	return nil
}

func resolvedCookiePath(cookiePath string) string {
	if cookiePath != "" {
		return cookiePath
	}
	if env := os.Getenv("COOKIES_PATH"); env != "" {
		return env
	}
	return filepath.Join("data", "cookies.json")
}
