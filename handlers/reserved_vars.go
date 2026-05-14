package handlers

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// resolveReservedVarMap returns the current computed values for all reserved
// variables.  serviceName is the ServiceName field of the template being run;
// username is the authenticated user's login.
func resolveReservedVarMap(username, serviceName string) map[string]string {
	return map[string]string{
		"GUID":         generateGUID(),
		"GUID1":        generateGUID(),
		"SERVICENAME":  serviceName,
		"CURRENT_TIME": time.Now().Format("20060102-150405"),
		"APPNAME":      "APICaller",
		"APPUSER":      username,
	}
}

// applyReservedVars replaces any remaining {{RESERVED}} tokens in s using the
// supplied values map.  Call this after regular variable substitution.
func applyReservedVars(s string, values map[string]string) string {
	for name, val := range values {
		s = strings.ReplaceAll(s, "{{"+name+"}}", val)
	}
	return s
}

// generateGUID returns a version-4 UUID string.
func generateGUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10xx
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
