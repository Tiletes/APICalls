package models

import "time"

// Note is an annotation that users can attach to a specific template execution.
type Note struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	Body          string    `json:"body"`
	IsPrivate     bool      `json:"is_private"`
	OwnerUsername string    `json:"owner_username"`
	EnvironmentID *int64    `json:"environment_id,omitempty"` // nil = all environments
	TemplateID    *int64    `json:"template_id,omitempty"`    // required — note belongs to a template
	CreatedAt     time.Time `json:"created_at"`
}
