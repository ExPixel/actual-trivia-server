package null

import (
	"encoding/json"
	"testing"
)

func TestInt64ToJSON(t *testing.T) {
	data, err := json.Marshal(NewInt64(16))
	if err != nil {
		t.Fatalf("failed to marshal int64 into JSON: %v", err)
	}
	s := string(data)
	if s != "16" {
		t.Errorf("marshaling int64 into JSON returned incorrect result: %v", s)
	}

	data, err = json.Marshal(Int64{})
	if err != nil {
		t.Fatalf("failed to marshal null int64 into JSON: %v", err)
	}
	s = string(data)
	if s != "null" {
		t.Errorf("marshaling null int64 into JSON returned incorrect result: %v", s)
	}
}

func TestJSONToInt64(t *testing.T) {
	type check struct {
		I Int64 `json:"check"`
	}

	const jsonValue = `{ "check": 16 }`
	const jsonNull = `{ "check": null }`

	expectValue := check{I: Int64{}}
	err := json.Unmarshal([]byte(jsonValue), &expectValue)
	if err != nil {
		t.Fatalf("failed to unmarshal int64 with value from JSON: %v", err)
	}
	if !expectValue.I.Valid || expectValue.I.Int64 != 16 {
		t.Errorf("unmarshaled int64 from JSON returned incorrect result: %+v", expectValue)
	}

	expectNull := check{I: NewInt64(64)}
	err = json.Unmarshal([]byte(jsonNull), &expectNull)
	if err != nil {
		t.Fatalf("failed to unmarshal int64 with null value from JSON: %v", err)
	}
	if expectNull.I.Valid {
		t.Errorf("unmarshaled null int64 from JSON returned incorrect result: %+v", expectNull)
	}
}

func TestInt64TextMarshaling(t *testing.T) {
	withValue := NewInt64(16)
	b, err := withValue.MarshalText()
	if err != nil {
		t.Fatalf("failed to marshal int64 with value into text: %v", err)
	}
	s := string(b)
	if s != "16" {
		t.Errorf("marshaling int64 with value into text retruned incorrect result: %v", s)
	}

	isNull := Int64{}
	b, err = isNull.MarshalText()
	if err != nil {
		t.Fatalf("failed to marshal int64 with null value into text: %v", err)
	}
	s = string(b)
	if s != "" {
		t.Errorf("marhsaling int64 with null value into text returned incorrect result: %v", s)
	}
}
