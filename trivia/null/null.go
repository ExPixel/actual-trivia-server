package null

import (
	"database/sql"
)

// #TODO Implement JSON marshalling and unmarshalling for Int64 and String

// Int64 represents a nullable int64
type Int64 struct {
	sql.NullInt64
}

// NewInt64 creates a new int64 marked as nonnull with the given vaue.
func NewInt64(value int64) Int64 {
	return Int64{sql.NullInt64{Int64: value, Valid: true}}
}

// String represents a nullable string
type String struct {
	sql.NullString
}

// NewString creates a new string marked as nonnull with the given value.
func NewString(value string) String {
	return String{sql.NullString{String: value, Valid: true}}
}
