package auth

import (
	"apicalls/models"
	"net/http"
)

// RequireLogin redirects to /login if no session user is present.
func (m *Manager) RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.CurrentUser(r) == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole returns a middleware that checks the user has one of the given roles.
// If not, it responds with 403.
func (m *Manager) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := m.CurrentUser(r)
			if user == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			if !user.HasRole(roles...) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP returns the client IP from the request.
func ClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// RoleLabel returns a human-friendly label for a role.
func RoleLabel(role string) string {
	switch role {
	case models.RoleAdmin:
		return "Administrator"
	case models.RoleStandard:
		return "Standard"
	case models.RoleRestricted:
		return "Restricted"
	case models.RoleGuest:
		return "Guest"
	default:
		return role
	}
}
