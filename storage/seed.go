package storage

import (
	"golang.org/x/crypto/bcrypt"
)

// seed inserts default data if tables are empty, and always ensures the admin user exists.
func (db *DB) seed() error {
	// Always upsert the admin user so the password is always correct on startup.
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	db.conn.Exec(`
		INSERT INTO users (username, password, role) VALUES (?, ?, ?)
		ON CONFLICT(username) DO UPDATE SET password=excluded.password, role=excluded.role`,
		"admin", string(hash), "admin")

	// Default environments
	var count int
	db.conn.QueryRow(`SELECT COUNT(*) FROM environments`).Scan(&count)
	if count == 0 {
		db.conn.Exec(`INSERT INTO environments (name,color) VALUES (?,?)`, "PRD", "#e74c3c")
		db.conn.Exec(`INSERT INTO environments (name,color) VALUES (?,?)`, "QMS", "#f39c12")
		db.conn.Exec(`INSERT INTO environments (name,color) VALUES (?,?)`, "DEV", "#27ae60")
	}
	return nil
}
