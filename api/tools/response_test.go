package tools

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildAPIError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "unauthorized sentinel",
			err:        UnauthorizedErr("session expired"),
			wantStatus: http.StatusUnauthorized,
			wantCode:   CodeUnauthorized,
			wantMsg:    "session expired",
		},
		{
			name:       "invalid identifier wraps",
			err:        ValidateIdentifier("1bad"),
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeInvalidIdentifier,
			wantMsg:    "identifier contains invalid characters: identifier must start with letter or underscore",
		},
		{
			name:       "invalid request prefix",
			err:        InvalidRequestErr("name is required"),
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeInvalidRequest,
			wantMsg:    "name is required",
		},
		{
			name:       "unique constraint string match",
			err:        errors.New("UNIQUE constraint failed: users.email"),
			wantStatus: http.StatusConflict,
			wantCode:   CodeUniqueViolation,
			wantMsg:    "record already exists",
		},
		{
			name:       "turso 404 string match",
			err:        errors.New("turso API error: 404"),
			wantStatus: http.StatusNotFound,
			wantCode:   CodeTursoNotFound,
			wantMsg:    "Turso resource not found",
		},
		{
			name:       "unknown error fallback",
			err:        errors.New("boom"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   CodeInternalError,
			wantMsg:    "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, apiErr := BuildAPIError(tt.err)
			if status != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, status)
			}
			if apiErr.Code != tt.wantCode {
				t.Fatalf("expected code %s, got %s", tt.wantCode, apiErr.Code)
			}
			if apiErr.Message != tt.wantMsg {
				t.Fatalf("expected message %q, got %q", tt.wantMsg, apiErr.Message)
			}
			if apiErr.Hint == "" {
				t.Fatal("expected non-empty hint")
			}
		})
	}
}

func TestRespErr(t *testing.T) {
	rec := httptest.NewRecorder()

	RespErr(rec, ErrMissingDatabase)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected content-type application/json, got %q", got)
	}

	var apiErr APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if apiErr.Code != CodeMissingDatabase {
		t.Fatalf("expected code %s, got %s", CodeMissingDatabase, apiErr.Code)
	}
}
