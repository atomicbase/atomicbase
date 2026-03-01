package auth

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/atombasedev/atombase/config"
	"github.com/atombasedev/atombase/tools"
)

type API struct {
	db *sql.DB
}

func NewAPI(db *sql.DB) *API {
	return &API{db: db}
}

func (api *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/magic-link/start", api.withBody(api.handleMagicLinkStart))
	mux.HandleFunc("GET /auth/magic-link/complete", api.handleMagicLinkComplete)
	mux.HandleFunc("POST /auth/signout", api.handleSignout)
	mux.HandleFunc("GET /auth/me", api.handleMe)
}

func (api *API) withBody(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, config.Cfg.MaxRequestBody)
		defer r.Body.Close()
		handler(w, r)
	}
}

// POST /auth/magic-link/start
func (api *API) handleMagicLinkStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		tools.RespErr(w, tools.ErrInvalidJSON)
		return
	}

	if err := BeginMagicLogin(req.Email, api.db, r.Context()); err != nil {
		if err == ErrInvalidEmail {
			tools.RespErr(w, tools.InvalidRequestErr("invalid email"))
			return
		}
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "magic link sent",
	})
}

// GET /auth/magic-link/complete?token=...
func (api *API) handleMagicLinkComplete(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		tools.RespErr(w, tools.InvalidRequestErr("token is required"))
		return
	}

	user, session, isNew, err := CompleteMagicLink(token, api.db, r.Context())
	if err != nil {
		if err == ErrInvalidOrExpiredMagicLink {
			tools.RespErr(w, tools.UnauthorizedErr("invalid or expired magic link"))
			return
		}
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"user":       user,
		"token":      session.Token(),
		"expires_at": session.ExpiresAt,
		"is_new":     isNew,
	})
}

// POST /auth/signout
func (api *API) handleSignout(w http.ResponseWriter, r *http.Request) {
	session, err := api.getSession(r)
	if err != nil {
		tools.RespErr(w, tools.UnauthorizedErr("invalid session"))
		return
	}

	if err := DeleteSession(session.Id, api.db, r.Context()); err != nil {
		tools.RespErr(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /auth/me
func (api *API) handleMe(w http.ResponseWriter, r *http.Request) {
	session, err := api.getSession(r)
	if err != nil {
		tools.RespErr(w, tools.UnauthorizedErr("invalid session"))
		return
	}

	user, err := GetUserByID(session.UserID, api.db, r.Context())
	if err != nil {
		tools.RespErr(w, err)
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// Extract and validate session from auth context
func (api *API) getSession(r *http.Request) (*Session, error) {
	auth := tools.GetAuthContext(r.Context())
	if auth.Role != tools.RoleUser || auth.Token == "" {
		return nil, ErrInvalidSession
	}
	return ValidateSession(SessionToken(auth.Token), api.db, r.Context())
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
