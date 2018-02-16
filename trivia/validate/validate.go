package validate

import (
	"regexp"
)

var emailRegex = regexp.MustCompile("[^@]+@[^@]+")

// IsEmail returns true if the given value is a "valid" email as far as syntax goes anyway.
// further validation should be done by sending an actual email to the given address.
func IsEmail(value string) bool {
	return emailRegex.MatchString(value)
}
