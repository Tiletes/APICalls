package storage

import (
	"apicalls/models"
	"database/sql"
	"errors"
	"fmt"
)

// GetUserByUsername returns the user matching username or an error.
func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	u := &models.User{}
	err := db.conn.QueryRow(
		`SELECT id,username,password,role FROM users WHERE username=?`, username,
	).Scan(&u.ID, &u.Username, &u.Password, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

// ListUsers returns all users.
func (db *DB) ListUsers() ([]*models.User, error) {
	rows, err := db.conn.Query(`SELECT id,username,password,role FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &u.Role); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// CreateUser inserts a new user.
func (db *DB) CreateUser(u *models.User) error {
	res, err := db.conn.Exec(
		`INSERT INTO users (username,password,role) VALUES (?,?,?)`,
		u.Username, u.Password, u.Role,
	)
	if err != nil {
		return err
	}
	u.ID, _ = res.LastInsertId()
	return nil
}

// UpdateUser updates username, password and role.
func (db *DB) UpdateUser(u *models.User) error {
	_, err := db.conn.Exec(
		`UPDATE users SET username=?,password=?,role=? WHERE id=?`,
		u.Username, u.Password, u.Role, u.ID,
	)
	return err
}

// DeleteUser removes a user by ID.
func (db *DB) DeleteUser(id int64) error {
	res, err := db.conn.Exec(`DELETE FROM users WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user %d not found", id)
	}
	return nil
}
