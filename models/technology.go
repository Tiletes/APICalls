package models

import "encoding/json"

// CustomValue is a key/value pair used in Technologies and Templates.
type CustomValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Technology defines a reusable HTTP call blueprint.
type Technology struct {
	ID           int64         `json:"id"`
	Name         string        `json:"name"`
	Method       string        `json:"method"`
	URL          string        `json:"url"`
	Body         string        `json:"body"`
	Headers      []CustomValue `json:"headers"`
	CustomValues []CustomValue `json:"custom_values"`
}

func (t *Technology) HeadersToJSON() (string, error) {
	b, err := json.Marshal(t.Headers)
	return string(b), err
}

func (t *Technology) HeadersFromJSON(s string) error {
	if s == "" {
		t.Headers = []CustomValue{}
		return nil
	}
	return json.Unmarshal([]byte(s), &t.Headers)
}

func (t *Technology) CustomValuesToJSON() (string, error) {
	b, err := json.Marshal(t.CustomValues)
	return string(b), err
}

func (t *Technology) CustomValuesFromJSON(s string) error {
	if s == "" {
		t.CustomValues = []CustomValue{}
		return nil
	}
	return json.Unmarshal([]byte(s), &t.CustomValues)
}
