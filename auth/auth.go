package auth

import (
	"apicalls/models"
	"apicalls/storage"
	"net/http"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

const sessionUserKey = "user"

// Manager handles authentication and session management.
type Manager struct {
	store  *sessions.CookieStore
	db     *storage.DB
	name   string
	maxAge int
}

// NewManager creates an auth Manager.
func NewManager(secret, sessionName string, maxAge int, db *storage.DB) *Manager {
	store := sessions.NewCookieStore([]byte(secret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	return &Manager{store: store, db: db, name: sessionName, maxAge: maxAge}
}

// Login authenticates the user and stores the session.
func (m *Manager) Login(w http.ResponseWriter, r *http.Request, username, password string) (*models.User, error) {
	user, err := m.db.GetUserByUsername(username)
	if err != nil || user == nil {
		return nil, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, nil
	}
	session, _ := m.store.Get(r, m.name)
	session.Values[sessionUserKey] = user.Username
	session.Save(r, w)
	return user, nil
}

// Logout destroys the session.
func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := m.store.Get(r, m.name)
	session.Options.MaxAge = -1
	session.Save(r, w)
}

// CurrentUser returns the logged-in user from the session, or nil.
func (m *Manager) CurrentUser(r *http.Request) *models.User {
	session, err := m.store.Get(r, m.name)
	if err != nil {
		return nil
	}
	username, ok := session.Values[sessionUserKey].(string)
	if !ok || username == "" {
		return nil
	}
	user, _ := m.db.GetUserByUsername(username)
	return user
}
