package null

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
)

// String represents a nullable string
type String struct {
	sql.NullString
}

// NewString creates a new string marked as nonnull with the given value.
func NewString(value string) String {
	return String{sql.NullString{String: value, Valid: true}}
}

// MarshalJSON implements json.Marshaler for String
func (s String) MarshalJSON() ([]byte, error) {
	if !s.Valid {
		return []byte("null"), nil
	}
	return []byte("\"" + s.String + "\""), nil
}

// UnmarshalJSON implements json.Unmarshaler for String
func (s *String) UnmarshalJSON(data []byte) (err error) {
	var v interface{}
	if err = json.Unmarshal(data, v); err != nil {
		return
	}

	// #NOTE if I ever pass by this code again I should consider
	// unmarshaling floats and integers too just to make this
	// simpler and more flexible, but it might defeat the point
	// of type safety, I don't know. -- Adolph C.
	switch x := v.(type) {
	case string:
		s.Valid = true
		s.String = x
	case nil:
		s.Valid = false
	default:
		err = fmt.Errorf("null: cannot unmarshal %v into null.String", reflect.TypeOf(v).Name())
	}
	return
}

// MarshalText implements TextMarshaler for String
func (s String) MarshalText() ([]byte, error) {
	if !s.Valid {
		return []byte{}, nil
	}
	return []byte(s.String), nil
}

// UnmarshalText implements TextUnarmshaler for String
func (s *String) UnmarshalText(text []byte) error {
	s.Valid = true
	s.String = string(text)
	return nil
}
