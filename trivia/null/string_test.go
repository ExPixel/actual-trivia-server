package null

import (
	"encoding/json"
	"testing"
)

func TestStringToJSON(t *testing.T) {
	data, err := json.Marshal(NewString("hello"))
	if err != nil {
		t.Fatalf("failed to marshal string into JSON: %v", err)
	}
	s := string(data)
	if s != `"hello"` {
		t.Errorf("marshaling string into JSON returned incorrect result: %v", s)
	}

	data, err = json.Marshal(String{})
	if err != nil {
		t.Fatalf("failed to marshal null string into JSON: %v", err)
	}
	s = string(data)
	if s != "null" {
		t.Errorf("marshaling null string into JSON returned incorrect result: %v", s)
	}
}

func TestJSONToString(t *testing.T) {
	type check struct {
		I String `json:"check"`
	}

	const jsonValue = `{ "check": "hello" }`
	const jsonNull = `{ "check": null }`

	expectValue := check{I: String{}}
	err := json.Unmarshal([]byte(jsonValue), &expectValue)
	if err != nil {
		t.Fatalf("failed to unmarshal string with value from JSON: %v", err)
	}
	if !expectValue.I.Valid || expectValue.I.String != "hello" {
		t.Errorf("unmarshaled string from JSON returned incorrect result: %+v", expectValue)
	}

	expectNull := check{I: NewString("hello")}
	err = json.Unmarshal([]byte(jsonNull), &expectNull)
	if err != nil {
		t.Fatalf("failed to unmarshal string with null value from JSON: %v", err)
	}
	if expectNull.I.Valid {
		t.Errorf("unmarshaled null string from JSON returned incorrect result: %+v", expectNull)
	}
}

func TestStringTextMarshaling(t *testing.T) {
	withValue := NewString("hello")
	b, err := withValue.MarshalText()
	if err != nil {
		t.Fatalf("failed to marshal string with value into text: %v", err)
	}
	s := string(b)
	if s != "hello" {
		t.Errorf("marshaling string with value into text retruned incorrect result: %v", s)
	}

	isNull := String{}
	b, err = isNull.MarshalText()
	if err != nil {
		t.Fatalf("failed to marshal string with null value into text: %v", err)
	}
	s = string(b)
	if s != "" {
		t.Errorf("marhsaling string with null value into text returned incorrect result: %v", s)
	}
}
