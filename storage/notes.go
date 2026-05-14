package storage

import (
	"apicalls/models"
	"database/sql"
	"fmt"
	"time"
)

// ListNotes returns notes visible to the given user for a specific template.
// Returns global notes (environment_id IS NULL) plus notes specific to envID.
// envID == 0 returns only global notes. templateID == 0 returns an empty list.
// Private notes are only visible to their owner.
func (db *DB) ListNotes(viewerUsername string, envID int64, templateID int64) ([]*models.Note, error) {
	if templateID == 0 {
		return nil, nil
	}
	query := `SELECT id,title,body,is_private,owner_username,environment_id,template_id,created_at
		FROM notes
		WHERE (is_private=0 OR owner_username=?)
		  AND (environment_id IS NULL OR environment_id=?)
		  AND template_id=?
		ORDER BY created_at DESC`
	rows, err := db.conn.Query(query, viewerUsername, envID, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var notes []*models.Note
	for rows.Next() {
		n := &models.Note{}
		var priv int
		var envIDNull sql.NullInt64
		var tmplIDNull sql.NullInt64
		var createdAt string
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &priv, &n.OwnerUsername, &envIDNull, &tmplIDNull, &createdAt); err != nil {
			return nil, err
		}
		n.IsPrivate = priv == 1
		if envIDNull.Valid {
			n.EnvironmentID = &envIDNull.Int64
		}
		if tmplIDNull.Valid {
			n.TemplateID = &tmplIDNull.Int64
		}
		n.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		if n.CreatedAt.IsZero() {
			n.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
		}
		if n.CreatedAt.IsZero() {
			n.CreatedAt, _ = time.Parse("2006-01-02 15:04:05.999999999", createdAt)
		}
		if n.CreatedAt.IsZero() {
			n.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		}
		notes = append(notes, n)
	}
	return notes, nil
}

func (db *DB) CreateNote(n *models.Note) error {
	priv := 0
	if n.IsPrivate {
		priv = 1
	}
	var envID interface{}
	if n.EnvironmentID != nil {
		envID = *n.EnvironmentID
	}
	var tmplID interface{}
	if n.TemplateID != nil {
		tmplID = *n.TemplateID
	}
	res, err := db.conn.Exec(
		`INSERT INTO notes (title,body,is_private,owner_username,environment_id,template_id) VALUES (?,?,?,?,?,?)`,
		n.Title, n.Body, priv, n.OwnerUsername, envID, tmplID,
	)
	if err != nil {
		return err
	}
	n.ID, _ = res.LastInsertId()
	return nil
}

func (db *DB) UpdateNote(n *models.Note) error {
	priv := 0
	if n.IsPrivate {
		priv = 1
	}
	var envID interface{}
	if n.EnvironmentID != nil {
		envID = *n.EnvironmentID
	}
	var tmplID interface{}
	if n.TemplateID != nil {
		tmplID = *n.TemplateID
	}
	_, err := db.conn.Exec(
		`UPDATE notes SET title=?,body=?,is_private=?,environment_id=?,template_id=? WHERE id=? AND owner_username=?`,
		n.Title, n.Body, priv, envID, tmplID, n.ID, n.OwnerUsername,
	)
	return err
}

func (db *DB) DeleteNote(id int64, ownerUsername string, isAdmin bool) error {
	var res sql.Result
	var err error
	if isAdmin {
		res, err = db.conn.Exec(`DELETE FROM notes WHERE id=?`, id)
	} else {
		res, err = db.conn.Exec(`DELETE FROM notes WHERE id=? AND owner_username=?`, id, ownerUsername)
	}
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("note %d not found or not owned by %s", id, ownerUsername)
	}
	return nil
}
