package null

import (
	"database/sql"
)

// #TODO implement json and text marshalling for the string type.

// String represents a nullable string
type String struct {
	sql.NullString
}

// NewString creates a new string marked as nonnull with the given value.
func NewString(value string) String {
	return String{sql.NullString{String: value, Valid: true}}
}
