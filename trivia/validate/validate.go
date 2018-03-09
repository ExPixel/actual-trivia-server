package validate

import (
	"regexp"
)

var emailRegex = regexp.MustCompile("^[^@]+@[^@]+$")

// allows the characters a-z, A-Z, 0-9, -, _, <, >, and .
var usernameRegex = regexp.MustCompile("^[a-zA-Z0-9_\\-\\.\\<\\>]+$")

// IsEmail returns true if the given value is a "valid" email as far as syntax goes anyway.
// further validation should be done by sending an actual email to the given address.
func IsEmail(value string) bool {
	return emailRegex.MatchString(value)
}

// IsValidUsername returns true if the given value is a valid username.
func IsValidUsername(username string) bool {
	return usernameRegex.MatchString(username)
}
