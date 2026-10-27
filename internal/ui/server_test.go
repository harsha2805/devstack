package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("DEVSTACK_APPDATA", t.TempDir())
	s := &Server{Version: "test"}
	if _, err := s.Listen(); err != nil {
		t.Fatal(err)
	}
	return s
}

func do(t *testing.T, s *Server, method, path string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://"+s.addr+path, strings.NewReader("{}"))
	req.Host = s.addr
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)
	return rec
}

func TestAPIGuard(t *testing.T) {
	s := testServer(t)
	withToken := func(r *http.Request) { r.Header.Set("X-Token", s.token) }

	if rec := do(t, s, "GET", "/api/state", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d, want 401", rec.Code)
	}
	if rec := do(t, s, "GET", "/api/state", func(r *http.Request) { r.Header.Set("X-Token", "wrong") }); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: got %d, want 401", rec.Code)
	}
	if rec := do(t, s, "GET", "/api/state", func(r *http.Request) { withToken(r); r.Host = "evil.example:80" }); rec.Code != http.StatusForbidden {
		t.Errorf("DNS-rebound host: got %d, want 403", rec.Code)
	}
	if rec := do(t, s, "POST", "/api/stop", withToken); rec.Code != http.StatusForbidden {
		t.Errorf("POST without X-Devstack: got %d, want 403", rec.Code)
	}
	if rec := do(t, s, "GET", "/api/state", withToken); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"needs_setup":true`) {
		t.Errorf("authorized state: %d %s", rec.Code, rec.Body.String())
	}
}

func TestTokenLoginSetsCookie(t *testing.T) {
	s := testServer(t)
	rec := do(t, s, "GET", "/?t="+s.token, nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("login: got %d", rec.Code)
	}
	c := rec.Result().Cookies()
	if len(c) != 1 || c[0].Value != s.token || !c[0].HttpOnly || c[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie: %+v", c)
	}
	if rec := do(t, s, "GET", "/", func(r *http.Request) { r.AddCookie(c[0]) }); rec.Code != http.StatusOK {
		t.Errorf("page with cookie: %d", rec.Code)
	}
	if rec := do(t, s, "GET", "/", nil); rec.Code != http.StatusForbidden {
		t.Errorf("page without cookie: %d", rec.Code)
	}
	if rec := do(t, s, "GET", "/?t=nope", nil); rec.Code != http.StatusForbidden {
		t.Errorf("bad login token: %d", rec.Code)
	}
}

func TestOpenRefusesArbitraryTargets(t *testing.T) {
	s := testServer(t)
	for _, body := range []string{
		`{"kind":"url","target":"file:///C:/Windows/System32/calc.exe"}`,
		`{"kind":"url","target":"https://example.com/"}`,
		`{"kind":"folder","target":"C:\\Windows"}`,
		`{"kind":"exe","target":"cmd.exe"}`,
	} {
		req := httptest.NewRequest("POST", "http://"+s.addr+"/api/open", strings.NewReader(body))
		req.Header.Set("X-Token", s.token)
		req.Header.Set("X-Devstack", "1")
		rec := httptest.NewRecorder()
		s.routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: got %d, want 403", body, rec.Code)
		}
	}
}

func TestLocalURL(t *testing.T) {
	ok := []string{"http://localhost:8080/", "http://hello.localhost", "http://api.kbs.localhost:8080/x?y", "http://kbs.test/",
		"https://hello.localhost", "https://kbs.test:8443/"}
	bad := []string{"https://evil.com/", "https://hello.localhost.evil.com/", "http://localhost.evil.com/", "http://example.com/?x=.localhost", "file:///C:/x", "javascript:alert(1)"}
	for _, u := range ok {
		if !localURL(u) {
			t.Errorf("%s should be allowed", u)
		}
	}
	for _, u := range bad {
		if localURL(u) {
			t.Errorf("%s should be refused", u)
		}
	}
}
