package models

// Environment represents a deployment environment (e.g. PRD, QMS, DEV).
type Environment struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`    // hex colour, e.g. "#ff0000"
	Priority int    `json:"priority"` // higher value = shown first
}
