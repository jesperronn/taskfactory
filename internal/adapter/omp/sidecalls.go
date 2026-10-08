package omp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sideCallRoles are the OMP config roles that route side calls away from the
// requested model.
var sideCallRoles = []string{"tiny", "smol"}

// sideCallNote describes which models the tiny and smol roles use, read from
// <home>/.omp/agent/config.yml. A missing or unreadable file, or a missing role,
// is reported as unknown rather than guessed.
func sideCallNote(home string) string {
	if home == "" {
		return fmt.Sprintf("side-call models for roles %s unknown: home directory unavailable", strings.Join(sideCallRoles, "/"))
	}
	path := filepath.Join(home, ".omp", "agent", "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Sprintf("side-call models for roles %s unknown: %s does not exist; side calls may use other models", strings.Join(sideCallRoles, "/"), path)
		}
		return fmt.Sprintf("side-call models for roles %s unknown: cannot read %s: %v", strings.Join(sideCallRoles, "/"), path, err)
	}
	roles := parseRoles(string(data))
	var parts []string
	var unknown []string
	for _, role := range sideCallRoles {
		model, ok := roles[role]
		if !ok {
			unknown = append(unknown, role)
			continue
		}
		parts = append(parts, role+"="+model)
	}
	if len(unknown) > 0 {
		return fmt.Sprintf("side-call models unknown for roles %s in %s; side calls may use other models", strings.Join(unknown, "/"), path)
	}
	return fmt.Sprintf("side calls use %s from %s; they do not use only the requested model", strings.Join(parts, ", "), path)
}

// parseRoles returns the first value found for each side-call role. It accepts
// indented or unindented `role: value` lines, strips quotes and trailing
// comments, and ignores anything else.
func parseRoles(text string) map[string]string {
	found := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if i := strings.Index(trimmed, "#"); i >= 0 {
			trimmed = strings.TrimSpace(trimmed[:i])
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if value == "" {
			continue
		}
		for _, role := range sideCallRoles {
			if key == role {
				if _, seen := found[role]; !seen {
					found[role] = value
				}
			}
		}
	}
	return found
}
