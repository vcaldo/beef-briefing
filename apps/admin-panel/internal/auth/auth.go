package auth

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"

	"beef-briefing/apps/admin-panel/internal/config"
)

const (
	sessionName     = "admin-panel-session"
	sessionKeyAuth  = "authenticated"
	sessionKeyTheme = "theme"
	sessionMaxAge   = 60 * 60 * 24 * 7 // 7 days in seconds
	defaultTheme    = "dark"
)

// ValidThemes contains all allowed theme values
var ValidThemes = map[string]bool{
	"light":     true,
	"dark":      true,
	"business":  true,
	"cyberpunk": true,
	"forest":    true,
}

// Auth handles authentication and session management
type Auth struct {
	store         *sessions.CookieStore
	username      string
	passwordHash  string
	secureCookies bool
}

// NewAuth creates a new Auth instance
func NewAuth(cfg *config.Config) (*Auth, error) {
	// Get session secret as bytes (decoded from base64 if needed)
	secretBytes, err := cfg.SessionSecretBytes()
	if err != nil {
		return nil, err
	}

	store := sessions.NewCookieStore(secretBytes)
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		Secure:   cfg.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}

	return &Auth{
		store:         store,
		username:      cfg.AdminUsername,
		passwordHash:  cfg.AdminPasswordHash,
		secureCookies: cfg.SecureCookies,
	}, nil
}

// VerifyCredentials checks if the provided credentials are valid
func (a *Auth) VerifyCredentials(username, password string) bool {
	if username != a.username {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(a.passwordHash), []byte(password))
	return err == nil
}

// CreateSession creates a new authenticated session
func (a *Auth) CreateSession(w http.ResponseWriter, r *http.Request) error {
	// Always create a fresh session to avoid issues with stale/invalid cookies
	session := sessions.NewSession(a.store, sessionName)
	session.IsNew = true
	session.Options = a.store.Options

	session.Values[sessionKeyAuth] = true
	session.Values[sessionKeyTheme] = defaultTheme

	return session.Save(r, w)
} // DestroySession removes the authenticated session
func (a *Auth) DestroySession(w http.ResponseWriter, r *http.Request) error {
	session, err := a.store.Get(r, sessionName)
	if err != nil {
		return err
	}

	session.Values[sessionKeyAuth] = false
	session.Options.MaxAge = -1

	return session.Save(r, w)
}

// IsAuthenticated checks if the current request has a valid session
func (a *Auth) IsAuthenticated(r *http.Request) bool {
	session, err := a.store.Get(r, sessionName)
	if err != nil {
		return false
	}

	auth, ok := session.Values[sessionKeyAuth].(bool)
	return ok && auth
}

// GetTheme returns the user's theme preference from the session
func (a *Auth) GetTheme(r *http.Request) string {
	session, err := a.store.Get(r, sessionName)
	if err != nil {
		return defaultTheme
	}

	theme, ok := session.Values[sessionKeyTheme].(string)
	if !ok || !ValidThemes[theme] {
		return defaultTheme
	}

	return theme
}

// SetTheme stores the user's theme preference in the session
func (a *Auth) SetTheme(w http.ResponseWriter, r *http.Request, theme string) error {
	if !ValidThemes[theme] {
		theme = defaultTheme
	}

	session, err := a.store.Get(r, sessionName)
	if err != nil {
		return err
	}

	session.Values[sessionKeyTheme] = theme
	return session.Save(r, w)
}

// RequireAuth is middleware that ensures the request is authenticated
func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.IsAuthenticated(r) {
			slog.Debug("unauthenticated request, redirecting to login", "path", r.URL.Path)
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
