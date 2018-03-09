package validate

import "testing"

func TestEmailValidation(t *testing.T) {
	goodEmail := "expixel@gmail.com"
	badEmail := "nope"

	if !IsEmail(goodEmail) {
		t.Errorf("incorrect result from IsEmail: failed for good email")
	}

	if IsEmail(badEmail) {
		t.Errorf("incorrect result from IsEmail: passed for bad email")
	}
}
