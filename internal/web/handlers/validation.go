package handlers

import (
	"fmt"
	"strings"
)

// validateEmail checks that s is a well-formed, ASCII-only email address with
// no '+' in the local part. It returns a user-friendly error or nil.
//
// Rules (in order):
//  1. Non-empty, max 254 chars (RFC 5321)
//  2. ASCII-only — rejects Unicode / homograph addresses
//  3. No whitespace
//  4. Exactly one '@'
//  5. Local part: a-zA-Z0-9._- only; no leading/trailing '.' or '-'; no '..'
//  6. No '+' in local part (blocks plus-alias farming)
//  7. Domain: a-zA-Z0-9-. only; at least one '.'; no leading/trailing '.' or '-'
//
// The returned string is the lowercased canonical form of the address (caller
// should use it instead of the original input to ensure case-insensitive
// deduplication).
func validateEmail(s string) (canonical string, err error) {
	if s == "" {
		return "", fmt.Errorf("email address is required")
	}
	if len(s) > 254 {
		return "", fmt.Errorf("email address is too long (max 254 characters)")
	}

	// ASCII-only — reject any byte above 0x7E.
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7E {
			return "", fmt.Errorf("email address must contain ASCII characters only")
		}
	}

	// No whitespace.
	if strings.ContainsAny(s, " \t\r\n") {
		return "", fmt.Errorf("email address must not contain whitespace")
	}

	// Exactly one '@'.
	atCount := strings.Count(s, "@")
	if atCount != 1 {
		return "", fmt.Errorf("email address must contain exactly one '@'")
	}

	local, domain, _ := strings.Cut(s, "@")

	// Local part validation.
	if local == "" {
		return "", fmt.Errorf("email address has an empty local part")
	}
	if strings.Contains(local, "+") {
		return "", fmt.Errorf("email address must not contain '+' in the local part")
	}
	if local[0] == '.' || local[0] == '-' || local[len(local)-1] == '.' || local[len(local)-1] == '-' {
		return "", fmt.Errorf("email address local part must not start or end with '.' or '-'")
	}
	if strings.Contains(local, "..") {
		return "", fmt.Errorf("email address local part must not contain '..'")
	}
	for i := 0; i < len(local); i++ {
		if !isLocalChar(local[i]) {
			return "", fmt.Errorf("email address local part contains invalid character %q", local[i])
		}
	}

	// Domain validation.
	if domain == "" {
		return "", fmt.Errorf("email address has an empty domain")
	}
	if !strings.Contains(domain, ".") {
		return "", fmt.Errorf("email address domain must contain at least one '.'")
	}
	if domain[0] == '.' || domain[0] == '-' || domain[len(domain)-1] == '.' || domain[len(domain)-1] == '-' {
		return "", fmt.Errorf("email address domain must not start or end with '.' or '-'")
	}
	for i := 0; i < len(domain); i++ {
		if !isDomainChar(domain[i]) {
			return "", fmt.Errorf("email address domain contains invalid character %q", domain[i])
		}
	}

	return strings.ToLower(s), nil
}

func isLocalChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '.' || c == '_' || c == '-'
}

func isDomainChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '.' || c == '-'
}
