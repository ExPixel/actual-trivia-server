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

func TestUsernameValidation(t *testing.T) {
	goodUsername := "expixel"
	badUsername := "ex pixel"
	badUsername2 := "#expixel!"

	if !IsValidUsername(goodUsername) {
		t.Errorf("incorrect result from IsValidUsername: '%s' should be valid", goodUsername)
	}

	if IsValidUsername(badUsername) {
		t.Errorf("incorrect result from IsValidUsername: '%s' should be invalid", badUsername)
	}

	if IsValidUsername(badUsername2) {
		t.Errorf("incorrect result from IsValidUsername: '%s' should be invalid", badUsername2)
	}
}
