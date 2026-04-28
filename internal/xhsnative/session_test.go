package xhsnative

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteQRCodeHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "qrcode.html")
	if err := WriteQRCodeHTML(path, `data:image/png;base64,abc"`); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "小红书登录二维码") {
		t.Fatalf("expected title in html: %s", content)
	}
	if strings.Contains(content, `abc"`) {
		t.Fatalf("expected qrcode data URL to be escaped: %s", content)
	}
	if !strings.Contains(content, "abc&#34;") {
		t.Fatalf("expected escaped qrcode data URL: %s", content)
	}
}

func TestResolvedCookiePathDefaultsToDataCookies(t *testing.T) {
	t.Setenv("COOKIES_PATH", "")
	manager := NewSessionManager(true, "", "")
	if got := manager.resolvedCookiePath(); got != filepath.Join("data", "cookies.json") {
		t.Fatalf("unexpected cookie path %q", got)
	}
}
