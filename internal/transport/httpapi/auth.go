package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"termchat.local/termchat/internal/app"
	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/store"
)

const maxRequestBody = 1 << 20

type AuthHandler struct {
	auth           *app.AuthService
	attempts       *AttemptGuard
	disconnectUser func(string)
}

func NewAuthHandler(auth *app.AuthService, options ...any) *AuthHandler {
	handler := &AuthHandler{auth: auth}
	for _, option := range options {
		switch value := option.(type) {
		case *AttemptGuard:
			handler.attempts = value
		case func(string):
			handler.disconnectUser = value
		}
	}
	return handler
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Routes().ServeHTTP(w, r)
}

func (h *AuthHandler) Routes() http.Handler {
	router := chi.NewRouter()
	router.Post("/v1/auth/register", h.register)
	router.Post("/v1/auth/login", h.login)
	return router
}

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.")
		return
	}
	if err := h.auth.DeleteAccount(r.Context(), identity.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "The server could not complete the request.")
		return
	}
	if h.disconnectUser != nil {
		h.disconnectUser(identity.UserID)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	var request credentialsRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body.")
		return
	}
	result, err := h.auth.Register(r.Context(), request.Username, request.Password, time.Now().UTC())
	if err != nil {
		switch {
		case errors.Is(err, store.ErrConflict):
			writeError(w, http.StatusConflict, "USERNAME_TAKEN", "Username is already in use.")
		case errors.Is(err, domain.ErrInvalidUsername), errors.Is(err, app.ErrInvalidPassword):
			writeError(w, http.StatusBadRequest, "INVALID_CREDENTIALS_FORMAT", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "The server could not complete the request.")
		}
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var request credentialsRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body.")
		return
	}
	result, err := h.auth.Login(r.Context(), request.Username, request.Password, time.Now().UTC())
	if err != nil {
		if errors.Is(err, app.ErrInvalidCredentials) {
			if h.attempts != nil {
				h.attempts.RecordIPFailure(r, time.Now().UTC())
			}
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid username or password.")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "The server could not complete the request.")
		return
	}
	if h.attempts != nil && h.attempts.IsUserBlocked(result.User.ID, time.Now().UTC()) {
		writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", attemptRateLimitMessage)
		return
	}
	if h.attempts != nil {
		h.attempts.ResetIP(r)
	}
	writeJSON(w, http.StatusOK, result)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	response := errorResponse{}
	response.Error.Code = code
	response.Error.Message = message
	writeJSON(w, status, response)
}
