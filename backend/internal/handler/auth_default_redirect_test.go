package handler

import "testing"

func TestOAuthDefaultRedirectsUseHome(t *testing.T) {
	redirects := map[string]string{
		"email":    emailOAuthDefaultRedirect,
		"linuxdo":  linuxDoOAuthDefaultRedirectTo,
		"dingtalk": dingTalkOAuthDefaultRedirectTo,
		"oidc":     oidcOAuthDefaultRedirectTo,
		"wechat":   wechatOAuthDefaultRedirectTo,
	}

	for provider, redirect := range redirects {
		if redirect != "/home" {
			t.Errorf("%s OAuth default redirect = %q, want /home", provider, redirect)
		}
	}
}
