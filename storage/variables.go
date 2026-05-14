package storage

import (
	"apicalls/models"
	"fmt"
)

func (db *DB) ListVariables() ([]*models.Variable, error) {
	rows, err := db.conn.Query(`SELECT id,name,is_password,values_json FROM variables ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vars []*models.Variable
	for rows.Next() {
		v := &models.Variable{}
		var isPass int
		var valJSON string
		if err := rows.Scan(&v.ID, &v.Name, &isPass, &valJSON); err != nil {
			return nil, err
		}
		v.IsPassword = isPass == 1
		v.ValuesFromJSON(valJSON)
		vars = append(vars, v)
	}
	return vars, nil
}

func (db *DB) CreateVariable(v *models.Variable) error {
	valJSON, err := v.ValuesToJSON()
	if err != nil {
		return err
	}
	isPass := 0
	if v.IsPassword {
		isPass = 1
	}
	res, err := db.conn.Exec(
		`INSERT INTO variables (name,is_password,values_json) VALUES (?,?,?)`,
		v.Name, isPass, valJSON,
	)
	if err != nil {
		return err
	}
	v.ID, _ = res.LastInsertId()
	return nil
}

func (db *DB) UpdateVariable(v *models.Variable) error {
	valJSON, err := v.ValuesToJSON()
	if err != nil {
		return err
	}
	isPass := 0
	if v.IsPassword {
		isPass = 1
	}
	_, err = db.conn.Exec(
		`UPDATE variables SET name=?,is_password=?,values_json=? WHERE id=?`,
		v.Name, isPass, valJSON, v.ID,
	)
	return err
}

func (db *DB) DeleteVariable(id int64) error {
	res, err := db.conn.Exec(`DELETE FROM variables WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("variable %d not found", id)
	}
	return nil
}
