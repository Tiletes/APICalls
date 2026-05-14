package models

import "encoding/json"

// Template represents an HTTP request template.
type Template struct {
	ID                  int64         `json:"id"`
	Name                string        `json:"name"`
	ServiceName         string        `json:"service_name"`
	Path                string        `json:"path"`
	Method              string        `json:"method"`
	URL                 string        `json:"url"`
	Body                string        `json:"body"`
	Headers             []CustomValue `json:"headers"`
	CustomValues        []CustomValue `json:"custom_values"`
	RestrictedExecution bool          `json:"restricted_execution"`
	TechnologyID        *int64        `json:"technology_id,omitempty"`
}

func (t *Template) HeadersToJSON() (string, error) {
	b, err := json.Marshal(t.Headers)
	return string(b), err
}

func (t *Template) HeadersFromJSON(s string) error {
	if s == "" {
		t.Headers = []CustomValue{}
		return nil
	}
	return json.Unmarshal([]byte(s), &t.Headers)
}

func (t *Template) CustomValuesToJSON() (string, error) {
	b, err := json.Marshal(t.CustomValues)
	return string(b), err
}

func (t *Template) CustomValuesFromJSON(s string) error {
	if s == "" {
		t.CustomValues = []CustomValue{}
		return nil
	}
	return json.Unmarshal([]byte(s), &t.CustomValues)
}
