package null

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

// Int64 represents a nullable int64
type Int64 struct {
	sql.NullInt64
}

// NewInt64 creates a new int64 marked as nonnull with the given vaue.
func NewInt64(value int64) Int64 {
	return Int64{sql.NullInt64{Int64: value, Valid: true}}
}

// MarshalJSON implements json.Marshaler for Int64
func (i Int64) MarshalJSON() ([]byte, error) {
	if !i.Valid {
		return []byte("null"), nil
	}
	return []byte(strconv.FormatInt(i.Int64, 10)), nil
}

// UnmarshalJSON implements json.Unmarshaler for Int64
func (i *Int64) UnmarshalJSON(data []byte) (err error) {
	var v interface{}
	if err = json.Unmarshal(data, v); err != nil {
		return
	}
	switch x := v.(type) {
	case float64:
		err = json.Unmarshal(data, &i.Int64)
	case string:
		if len(x) == 0 {
			i.Valid = false
		} else {
			i.Int64, err = strconv.ParseInt(x, 10, 64)
		}
	case nil:
		i.Valid = false
	default:
		err = fmt.Errorf("null: cannot unmarshal %v into null.Int64", reflect.TypeOf(v).Name())
	}
	return
}

// MarshalText implements TextMarshaler for Int64
func (i Int64) MarshalText() ([]byte, error) {
	if !i.Valid {
		return []byte{}, nil
	}
	return []byte(strconv.FormatInt(i.Int64, 10)), nil
}

// UnmarshalText implements TextUnmarshaler for Int64
func (i *Int64) UnmarshalText(text []byte) error {
	s := string(text)
	if s == "" || s == "null" {
		i.Valid = false
		return nil
	}
	val, err := strconv.ParseInt(string(text), 10, 64)
	i.Int64 = val
	i.Valid = (err == nil) // #FIXME Maybe I should be throwing an error here.
	return err
}
