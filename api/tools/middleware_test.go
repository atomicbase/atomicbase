package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atombasedev/atombase/config"
)

func TestDetectAPIType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/platform/definitions", want: "platform"},
		{path: "/data/query/users", want: "data"},
		{path: "/docs", want: "other"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := detectAPIType(tt.path); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestGetAuthContext_DefaultAnonymous(t *testing.T) {
	auth := GetAuthContext(context.Background())
	if auth.Role != RoleAnonymous {
		t.Fatalf("expected anonymous role, got %q", auth.Role)
	}
	if auth.Token != "" {
		t.Fatalf("expected empty token, got %q", auth.Token)
	}
}

func TestCORSMiddleware(t *testing.T) {
	originalOrigins := config.Cfg.CORSOrigins
	defer func() {
		config.Cfg.CORSOrigins = originalOrigins
	}()

	tests := []struct {
		name          string
		origins       []string
		method        string
		originHeader  string
		wantStatus    int
		wantAllowOrig string
		wantNext      bool
	}{
		{
			name:       "disabled passes through",
			origins:    nil,
			method:     http.MethodGet,
			wantStatus: http.StatusAccepted,
			wantNext:   true,
		},
		{
			name:          "allowed origin passes through",
			origins:       []string{"https://app.example.com"},
			method:        http.MethodGet,
			originHeader:  "https://app.example.com",
			wantStatus:    http.StatusAccepted,
			wantAllowOrig: "https://app.example.com",
			wantNext:      true,
		},
		{
			name:         "disallowed origin forbidden",
			origins:      []string{"https://app.example.com"},
			method:       http.MethodGet,
			originHeader: "https://evil.example.com",
			wantStatus:   http.StatusForbidden,
		},
		{
			name:          "preflight allowed",
			origins:       []string{"*"},
			method:        http.MethodOptions,
			originHeader:  "https://any.example.com",
			wantStatus:    http.StatusNoContent,
			wantAllowOrig: "https://any.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Cfg.CORSOrigins = tt.origins
			nextCalled := false

			handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusAccepted)
			}))

			req := httptest.NewRequest(tt.method, "/data/query/users", nil)
			if tt.originHeader != "" {
				req.Header.Set("Origin", tt.originHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllowOrig {
				t.Fatalf("expected allow origin %q, got %q", tt.wantAllowOrig, got)
			}
			if nextCalled != tt.wantNext {
				t.Fatalf("expected nextCalled=%v, got %v", tt.wantNext, nextCalled)
			}
			if tt.method == http.MethodOptions && tt.wantStatus == http.StatusNoContent {
				if rec.Header().Get("Access-Control-Allow-Methods") == "" {
					t.Fatal("expected preflight methods header")
				}
				if rec.Header().Get("Access-Control-Allow-Headers") == "" {
					t.Fatal("expected preflight headers header")
				}
			}
		})
	}
}

func TestTimeoutMiddleware_SetsDeadline(t *testing.T) {
	originalTimeout := config.Cfg.RequestTimeout
	config.Cfg.RequestTimeout = 2
	defer func() {
		config.Cfg.RequestTimeout = originalTimeout
	}()

	handler := TimeoutMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("expected context deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > 2*time.Second {
			t.Fatalf("expected deadline within 2s window, got %v", remaining)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/data/query/users", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestClientIPFromRequest(t *testing.T) {
	originalTrusted := config.Cfg.TrustedProxyCIDRs
	defer func() {
		config.Cfg.TrustedProxyCIDRs = originalTrusted
	}()

	tests := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		trustedCIDRs []string
		wantClientIP string
	}{
		{
			name:         "untrusted remote ignores forwarded header",
			remoteAddr:   "198.51.100.10:443",
			forwardedFor: "203.0.113.4",
			wantClientIP: "198.51.100.10",
		},
		{
			name:         "trusted proxy uses first forwarded ip",
			remoteAddr:   "10.0.0.5:443",
			forwardedFor: "203.0.113.4, 10.0.0.5",
			trustedCIDRs: []string{"10.0.0.0/8"},
			wantClientIP: "203.0.113.4",
		},
		{
			name:         "trusted proxy falls back when forwarded invalid",
			remoteAddr:   "10.0.0.5:443",
			forwardedFor: "unknown, bad",
			trustedCIDRs: []string{"10.0.0.5"},
			wantClientIP: "10.0.0.5",
		},
		{
			name:         "bare remote ip supported",
			remoteAddr:   "127.0.0.1",
			wantClientIP: "127.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Cfg.TrustedProxyCIDRs = tt.trustedCIDRs
			req := httptest.NewRequest(http.MethodGet, "/data/query/users", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}

			if got := clientIPFromRequest(req); got != tt.wantClientIP {
				t.Fatalf("expected client IP %q, got %q", tt.wantClientIP, got)
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	originalKey := config.Cfg.APIKey
	config.Cfg.APIKey = "secret-key"
	defer func() {
		config.Cfg.APIKey = originalKey
	}()

	tests := []struct {
		name         string
		path         string
		authHeader   string
		wantStatus   int
		wantRole     AuthRole
		wantToken    string
		wantBodyCode string
	}{
		{
			name:         "platform requires auth",
			path:         "/platform/definitions",
			wantStatus:   http.StatusUnauthorized,
			wantBodyCode: "UNAUTHORIZED",
		},
		{
			name:         "platform invalid format",
			path:         "/platform/definitions",
			authHeader:   "Token abc",
			wantStatus:   http.StatusUnauthorized,
			wantBodyCode: "UNAUTHORIZED",
		},
		{
			name:         "platform wrong service key",
			path:         "/platform/definitions",
			authHeader:   "Bearer service.wrong",
			wantStatus:   http.StatusUnauthorized,
			wantBodyCode: "UNAUTHORIZED",
		},
		{
			name:       "platform valid service key",
			path:       "/platform/definitions",
			authHeader: "Bearer service.secret-key",
			wantStatus: http.StatusNoContent,
			wantRole:   RoleService,
		},
		{
			name:       "data anonymous allowed",
			path:       "/data/query/users",
			wantStatus: http.StatusNoContent,
			wantRole:   RoleAnonymous,
		},
		{
			name:       "data valid service key",
			path:       "/data/query/users",
			authHeader: "Bearer service.secret-key",
			wantStatus: http.StatusNoContent,
			wantRole:   RoleService,
		},
		{
			name:       "data session token",
			path:       "/data/query/users",
			authHeader: "Bearer session.secret",
			wantStatus: http.StatusNoContent,
			wantRole:   RoleUser,
			wantToken:  "session.secret",
		},
		{
			name:         "data invalid bearer format",
			path:         "/data/query/users",
			authHeader:   "Token abc",
			wantStatus:   http.StatusUnauthorized,
			wantBodyCode: "UNAUTHORIZED",
		},
		{
			name:         "data invalid token format",
			path:         "/data/query/users",
			authHeader:   "Bearer invalidtoken",
			wantStatus:   http.StatusUnauthorized,
			wantBodyCode: "UNAUTHORIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAuth AuthContext
			handler := AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = GetAuthContext(r.Context())
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
			if tt.wantStatus == http.StatusNoContent {
				if gotAuth.Role != tt.wantRole {
					t.Fatalf("expected role %q, got %q", tt.wantRole, gotAuth.Role)
				}
				if gotAuth.Token != tt.wantToken {
					t.Fatalf("expected token %q, got %q", tt.wantToken, gotAuth.Token)
				}
				return
			}

			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if body["code"] != tt.wantBodyCode {
				t.Fatalf("expected body code %q, got %q", tt.wantBodyCode, body["code"])
			}
			if body["message"] == "" {
				t.Fatal("expected non-empty unauthorized message")
			}
		})
	}
}

func TestPanicRecoveryMiddleware(t *testing.T) {
	handler := PanicRecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/data/query/users", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected content-type application/json, got %q", got)
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("expected internal server error body, got %q", rec.Body.String())
	}
}
