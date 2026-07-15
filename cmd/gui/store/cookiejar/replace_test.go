package cookiejar

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestReplaceCookiesRemovesOldScopes(t *testing.T) {
	jar := New(nil)
	www, _ := url.Parse("https://www.bilibili.com/")
	show, _ := url.Parse("https://show.bilibili.com/")
	passport, _ := url.Parse("https://passport.bilibili.com/x/passport-login/web/cookie/refresh")
	example, _ := url.Parse("https://www.example.com/")
	jar.SetCookies(www, []*http.Cookie{{Name: "SESSDATA", Value: "old-host", Path: "/"}})
	jar.SetCookies(www, []*http.Cookie{{Name: "SESSDATA", Value: "old-domain", Domain: ".bilibili.com", Path: "/"}})
	jar.SetCookies(show, []*http.Cookie{{Name: "SESSDATA", Value: "old-show", Path: "/account"}})
	jar.SetCookies(passport, []*http.Cookie{{Name: "SESSDATA", Value: "old-passport", Path: "/"}})
	jar.SetCookies(example, []*http.Cookie{{Name: "SESSDATA", Value: "other-site", Path: "/"}})

	jar.ReplaceCookies(passport, []*http.Cookie{{Name: "SESSDATA", Value: "new", Domain: ".bilibili.com", Path: "/"}})

	cookies := jar.Cookies(www)
	if len(cookies) != 1 || cookies[0].Name != "SESSDATA" || cookies[0].Value != "new" {
		t.Fatalf("new cookie did not replace all old scopes: %#v", cookies)
	}
	for _, entry := range jar.AllEntries() {
		if entry.Name == "SESSDATA" && strings.HasSuffix(entry.Domain, "bilibili.com") && entry.Value != "new" {
			t.Fatalf("stale subdomain cookie was retained: %#v", entry)
		}
	}
	if cookies := jar.Cookies(example); len(cookies) != 1 || cookies[0].Value != "other-site" {
		t.Fatalf("same-name cookie from another site was removed: %#v", cookies)
	}
}

func TestReplaceCookiesKeepsUnrelatedCookies(t *testing.T) {
	jar := New(nil)
	u, _ := url.Parse("https://www.bilibili.com/")
	jar.SetCookies(u, []*http.Cookie{
		{Name: "SESSDATA", Value: "old", Domain: ".bilibili.com", Path: "/"},
		{Name: "buvid3", Value: "device", Domain: ".bilibili.com", Path: "/"},
	})

	jar.ReplaceCookies(u, []*http.Cookie{{Name: "SESSDATA", Value: "new", Domain: ".bilibili.com", Path: "/"}})

	values := map[string]string{}
	for _, cookie := range jar.Cookies(u) {
		values[cookie.Name] = cookie.Value
	}
	if values["SESSDATA"] != "new" || values["buvid3"] != "device" || len(values) != 2 {
		t.Fatalf("unexpected cookies after replacement: %#v", values)
	}
}
