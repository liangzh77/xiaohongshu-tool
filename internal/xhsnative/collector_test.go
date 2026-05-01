package xhsnative

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsQRCodeLoginRequired(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "sentinel",
			err:  ErrQRCodeLoginRequired,
			want: true,
		},
		{
			name: "wrapped sentinel",
			err:  fmt.Errorf("暂停：%w", ErrQRCodeLoginRequired),
			want: true,
		},
		{
			name: "sorry page",
			err:  errors.New("笔记不可访问：Sorry, This Page Isn't Available Right Now."),
			want: true,
		},
		{
			name: "app scan page",
			err:  errors.New("请打开小红书App扫码查看"),
			want: true,
		},
		{
			name: "login expired",
			err:  errors.New("小红书登录态已失效，请先登录"),
			want: true,
		},
		{
			name: "ordinary detail failure",
			err:  errors.New("timeout waiting for selector"),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsQRCodeLoginRequired(tc.err); got != tc.want {
				t.Fatalf("IsQRCodeLoginRequired() = %v, want %v", got, tc.want)
			}
		})
	}
}
