package storage

import (
	"apicalls/models"
	"fmt"
)

func (db *DB) ListEnvironments() ([]*models.Environment, error) {
	rows, err := db.conn.Query(`SELECT id,name,color,priority FROM environments ORDER BY priority DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var envs []*models.Environment
	for rows.Next() {
		e := &models.Environment{}
		if err := rows.Scan(&e.ID, &e.Name, &e.Color, &e.Priority); err != nil {
			return nil, err
		}
		envs = append(envs, e)
	}
	return envs, nil
}

func (db *DB) GetEnvironmentByID(id int64) (*models.Environment, error) {
	e := &models.Environment{}
	err := db.conn.QueryRow(`SELECT id,name,color,priority FROM environments WHERE id=?`, id).
		Scan(&e.ID, &e.Name, &e.Color, &e.Priority)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (db *DB) CreateEnvironment(e *models.Environment) error {
	res, err := db.conn.Exec(`INSERT INTO environments (name,color,priority) VALUES (?,?,?)`, e.Name, e.Color, e.Priority)
	if err != nil {
		return err
	}
	e.ID, _ = res.LastInsertId()
	return nil
}

func (db *DB) UpdateEnvironment(e *models.Environment) error {
	_, err := db.conn.Exec(`UPDATE environments SET name=?,color=?,priority=? WHERE id=?`, e.Name, e.Color, e.Priority, e.ID)
	return err
}

func (db *DB) DeleteEnvironment(id int64) error {
	res, err := db.conn.Exec(`DELETE FROM environments WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("environment %d not found", id)
	}
	return nil
}
