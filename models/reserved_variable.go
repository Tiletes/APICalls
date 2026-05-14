package models

// ReservedVariable represents a system-defined substitution keyword.
// Reserved variables cannot be created or deleted by users — only their
// descriptions can be updated (by administrators).
type ReservedVariable struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ReservedVarNames is the canonical, authoritative list of every reserved
// keyword accepted by the substitution engine.  New entries must be added
// here and handled in handlers/execution_handler.go (resolveReserved).
var ReservedVarNames = []string{
	"GUID",
	"GUID1",
	"SERVICENAME",
	"CURRENT_TIME",
	"APPNAME",
	"APPUSER",
}

// IsReservedVarName returns true if name is a reserved keyword (case-sensitive).
func IsReservedVarName(name string) bool {
	for _, n := range ReservedVarNames {
		if n == name {
			return true
		}
	}
	return false
}
