package models

import "encoding/json"

// Variable represents a substitution variable with per-environment values.
type Variable struct {
	ID         int64             `json:"id"`
	Name       string            `json:"name"`
	IsPassword bool              `json:"is_password"`
	Values     map[string]string `json:"values"` // envName -> value
}

// ValuesToJSON serialises the Values map.
func (v *Variable) ValuesToJSON() (string, error) {
	b, err := json.Marshal(v.Values)
	return string(b), err
}

// ValuesFromJSON deserialises the Values map.
func (v *Variable) ValuesFromJSON(s string) error {
	v.Values = make(map[string]string)
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), &v.Values)
}

// ValuesJSON returns a JSON string of the Values map (single-return, template-safe).
func (v *Variable) ValuesJSON() string {
	s, _ := v.ValuesToJSON()
	return s
}
