package nicctl

import (
	"strings"
)

// DeriveMeasurement produces a Telegraf measurement name from a nicctl command string.
// It strips tokens like "sudo", "nicctl", "show", "--json", and any flags (--* or -*),
// joins the remaining tokens with "_", prefixes with "nicctl_", lowercases, and replaces
// hyphens with underscores.
func DeriveMeasurement(command string) string {
	skip := map[string]bool{
		"sudo":   true,
		"nicctl": true,
		"show":   true,
		"--json": true,
	}

	tokens := strings.Fields(command)
	var parts []string
	for _, tok := range tokens {
		if skip[tok] {
			continue
		}
		if strings.HasPrefix(tok, "--") || (strings.HasPrefix(tok, "-") && len(tok) > 1) {
			continue
		}
		parts = append(parts, tok)
	}

	name := strings.Join(parts, "_")
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "-", "_")

	if name == "" {
		return "nicctl"
	}
	return "nicctl_" + name
}
