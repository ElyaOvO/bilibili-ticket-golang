package cluster_service

import (
	"net/http"
	"net/url"
	"testing"

	"bilibili-ticket-golang/cluster/domain"
	"bilibili-ticket-golang/cmd/gui/store/cookiejar"
)

func TestRestoreAccountCookiesDoesNotReplayStaleFlatCookies(t *testing.T) {
	jar := cookiejar.New(nil)
	restoreAccountCookies(jar, domain.Credentials{
		Cookies: map[string]string{"SESSDATA": "stale-flat"},
		CookieJar: []domain.HTTPCookie{
			{Name: "SESSDATA", Value: "fresh", Domain: ".bilibili.com", Path: "/"},
			{Name: "SESSDATA", Value: "stale-host", Domain: "www.bilibili.com", Path: "/"},
		},
	})

	u, _ := url.Parse("https://www.bilibili.com/")
	cookies := jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Name != "SESSDATA" || cookies[0].Value != "fresh" {
		t.Fatalf("structured cookie jar was not authoritative: %#v", cookies)
	}
	jar.SetCookies(u, []*http.Cookie{{Name: "SESSDATA", Value: "refreshed", Domain: ".bilibili.com", Path: "/"}})
	cookies = jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Value != "refreshed" {
		t.Fatalf("refreshed cookie did not replace the previous value: %#v", cookies)
	}
}

func TestRestoreAccountCookiesSupportsLegacyFlatCookies(t *testing.T) {
	jar := cookiejar.New(nil)
	restoreAccountCookies(jar, domain.Credentials{Cookies: map[string]string{"SESSDATA": "legacy"}})

	u, _ := url.Parse("https://www.bilibili.com/")
	cookies := jar.Cookies(u)
	if len(cookies) != 1 || cookies[0].Name != "SESSDATA" || cookies[0].Value != "legacy" {
		t.Fatalf("legacy flat cookies were not restored: %#v", cookies)
	}
}
