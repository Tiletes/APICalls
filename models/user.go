package models

// Role constants.
const (
	RoleAdmin      = "admin"
	RoleStandard   = "standard"
	RoleRestricted = "restricted"
	RoleGuest      = "guest"
)

// User represents an application user.
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"` // bcrypt hash
	Role     string `json:"role"`
}

// HasRole returns true if the user's role is one of the given roles.
func (u *User) HasRole(roles ...string) bool {
	for _, r := range roles {
		if u.Role == r {
			return true
		}
	}
	return false
}
