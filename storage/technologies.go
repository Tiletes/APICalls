package storage

import (
	"apicalls/models"
	"fmt"
)

func (db *DB) ListTechnologies() ([]*models.Technology, error) {
	rows, err := db.conn.Query(
		`SELECT id,name,method,url,body,headers_json,custom_values_json FROM technologies ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var techs []*models.Technology
	for rows.Next() {
		t := &models.Technology{}
		var hj, cvj string
		if err := rows.Scan(&t.ID, &t.Name, &t.Method, &t.URL, &t.Body, &hj, &cvj); err != nil {
			return nil, err
		}
		t.HeadersFromJSON(hj)
		t.CustomValuesFromJSON(cvj)
		techs = append(techs, t)
	}
	return techs, nil
}

func (db *DB) GetTechnologyByID(id int64) (*models.Technology, error) {
	t := &models.Technology{}
	var hj, cvj string
	err := db.conn.QueryRow(
		`SELECT id,name,method,url,body,headers_json,custom_values_json FROM technologies WHERE id=?`, id,
	).Scan(&t.ID, &t.Name, &t.Method, &t.URL, &t.Body, &hj, &cvj)
	if err != nil {
		return nil, err
	}
	t.HeadersFromJSON(hj)
	t.CustomValuesFromJSON(cvj)
	return t, nil
}

func (db *DB) CreateTechnology(t *models.Technology) error {
	hj, _ := t.HeadersToJSON()
	cvj, _ := t.CustomValuesToJSON()
	res, err := db.conn.Exec(
		`INSERT INTO technologies (name,method,url,body,headers_json,custom_values_json) VALUES (?,?,?,?,?,?)`,
		t.Name, t.Method, t.URL, t.Body, hj, cvj,
	)
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

func (db *DB) UpdateTechnology(t *models.Technology) error {
	hj, _ := t.HeadersToJSON()
	cvj, _ := t.CustomValuesToJSON()
	_, err := db.conn.Exec(
		`UPDATE technologies SET name=?,method=?,url=?,body=?,headers_json=?,custom_values_json=? WHERE id=?`,
		t.Name, t.Method, t.URL, t.Body, hj, cvj, t.ID,
	)
	return err
}

func (db *DB) DeleteTechnology(id int64) error {
	res, err := db.conn.Exec(`DELETE FROM technologies WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("technology %d not found", id)
	}
	return nil
}
