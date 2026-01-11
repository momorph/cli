package cmd

import "strings"

// maskEmail partially masks the local part and shows domain
// e.g., john@example.com -> j***n@example.com
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}

	localPart := parts[0]
	domain := parts[1]

	if len(localPart) == 0 {
		return "***@" + domain
	}

	if len(localPart) == 1 {
		return "*@" + domain
	}

	if len(localPart) == 2 {
		// Show first char, mask last
		return string(localPart[0]) + "*@" + domain
	}

	// Show first and last char, mask middle
	return string(localPart[0]) + "***" + string(localPart[len(localPart)-1]) + "@" + domain
}
