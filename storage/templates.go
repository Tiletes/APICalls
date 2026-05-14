package storage

import (
	"apicalls/models"
	"database/sql"
	"fmt"
)

func (db *DB) ListTemplates() ([]*models.Template, error) {
	rows, err := db.conn.Query(
		`SELECT id,name,service_name,path,method,url,body,headers_json,custom_values_json,restricted_execution,technology_id
		 FROM templates ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var templates []*models.Template
	for rows.Next() {
		t := &models.Template{}
		var hj, cvj string
		var re int
		var techID sql.NullInt64
		if err := rows.Scan(&t.ID, &t.Name, &t.ServiceName, &t.Path, &t.Method, &t.URL, &t.Body, &hj, &cvj, &re, &techID); err != nil {
			return nil, err
		}
		t.RestrictedExecution = re == 1
		if techID.Valid {
			t.TechnologyID = &techID.Int64
		}
		t.HeadersFromJSON(hj)
		t.CustomValuesFromJSON(cvj)
		templates = append(templates, t)
	}
	return templates, nil
}

func (db *DB) GetTemplateByID(id int64) (*models.Template, error) {
	t := &models.Template{}
	var hj, cvj string
	var re int
	var techID sql.NullInt64
	err := db.conn.QueryRow(
		`SELECT id,name,service_name,path,method,url,body,headers_json,custom_values_json,restricted_execution,technology_id
		 FROM templates WHERE id=?`, id,
	).Scan(&t.ID, &t.Name, &t.ServiceName, &t.Path, &t.Method, &t.URL, &t.Body, &hj, &cvj, &re, &techID)
	if err != nil {
		return nil, err
	}
	t.RestrictedExecution = re == 1
	if techID.Valid {
		t.TechnologyID = &techID.Int64
	}
	t.HeadersFromJSON(hj)
	t.CustomValuesFromJSON(cvj)
	return t, nil
}

func (db *DB) CreateTemplate(t *models.Template) error {
	hj, _ := t.HeadersToJSON()
	cvj, _ := t.CustomValuesToJSON()
	re := 0
	if t.RestrictedExecution {
		re = 1
	}
	var techID interface{}
	if t.TechnologyID != nil {
		techID = *t.TechnologyID
	}
	res, err := db.conn.Exec(
		`INSERT INTO templates (name,service_name,path,method,url,body,headers_json,custom_values_json,restricted_execution,technology_id)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		t.Name, t.ServiceName, t.Path, t.Method, t.URL, t.Body, hj, cvj, re, techID,
	)
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

func (db *DB) UpdateTemplate(t *models.Template) error {
	hj, _ := t.HeadersToJSON()
	cvj, _ := t.CustomValuesToJSON()
	re := 0
	if t.RestrictedExecution {
		re = 1
	}
	var techID interface{}
	if t.TechnologyID != nil {
		techID = *t.TechnologyID
	}
	_, err := db.conn.Exec(
		`UPDATE templates SET name=?,service_name=?,path=?,method=?,url=?,body=?,headers_json=?,custom_values_json=?,
		 restricted_execution=?,technology_id=? WHERE id=?`,
		t.Name, t.ServiceName, t.Path, t.Method, t.URL, t.Body, hj, cvj, re, techID, t.ID,
	)
	return err
}

func (db *DB) DeleteTemplate(id int64) error {
	res, err := db.conn.Exec(`DELETE FROM templates WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("template %d not found", id)
	}
	return nil
}
