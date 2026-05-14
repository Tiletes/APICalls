package storage

import "apicalls/models"

// SeedReservedVariables inserts the built-in reserved variables with their
// default descriptions.  INSERT OR IGNORE ensures this is idempotent.
func (db *DB) SeedReservedVariables() error {
	seeds := map[string]string{
		"GUID":         "Generates a random UUID (v4) at execution time.",
		"GUID1":        "Generates a second independent random UUID (v4) at execution time.",
		"SERVICENAME":  "The Service Name of the selected template.",
		"CURRENT_TIME": "Current execution timestamp, formatted as yyyymmdd-hh24miss.",
		"APPNAME":      "Application name — always \"APICaller\".",
		"APPUSER":      "Login username of the currently authenticated user.",
	}
	for _, name := range models.ReservedVarNames {
		desc := seeds[name]
		_, err := db.conn.Exec(
			`INSERT OR IGNORE INTO reserved_variables (name, description) VALUES (?, ?)`,
			name, desc,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// ListReservedVariables returns all reserved variables ordered by their
// canonical position (as defined in models.ReservedVarNames).
func (db *DB) ListReservedVariables() ([]*models.ReservedVariable, error) {
	rows, err := db.conn.Query(
		`SELECT name, description FROM reserved_variables`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Index by name first, then return in canonical order.
	index := make(map[string]*models.ReservedVariable)
	for rows.Next() {
		rv := &models.ReservedVariable{}
		if err := rows.Scan(&rv.Name, &rv.Description); err != nil {
			return nil, err
		}
		index[rv.Name] = rv
	}

	result := make([]*models.ReservedVariable, 0, len(models.ReservedVarNames))
	for _, name := range models.ReservedVarNames {
		if rv, ok := index[name]; ok {
			result = append(result, rv)
		} else {
			// Fallback: name exists in code but not yet in DB (migration might
			// not have run for this name yet).
			result = append(result, &models.ReservedVariable{Name: name})
		}
	}
	return result, nil
}

// UpdateReservedVariableDescription sets a new description for the given
// reserved variable.
func (db *DB) UpdateReservedVariableDescription(name, description string) error {
	_, err := db.conn.Exec(
		`UPDATE reserved_variables SET description = ? WHERE name = ?`,
		description, name,
	)
	return err
}
