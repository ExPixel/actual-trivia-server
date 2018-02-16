package auth

import (
	"bytes"
	"testing"
)

// bcrypt -> 60 bytes
// 16byte block size -> +4 bytes (64 bytes)
// prepend IV -> +16 bytes (80 bytes)
const expectedAESLength = 80

func TestBlockPadding(t *testing.T) {
	// ^ it's not a complicated function but I plan to change it (maybe) and I don't trust myself.
	unpadded := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	padded := padToBlocksize(unpadded, 8, 6)
	expected := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 6, 6, 6, 6, 6, 6}
	if !bytes.Equal(padded, expected) {
		t.Fatal("auth: slices are being padded incorrectly")
	}
}

func TestPasswordComparisons(t *testing.T) {
	rawShortPassword := "short"
	encodedShortPassword, err := PreparePassword(rawShortPassword)
	if err != nil {
		t.Log("an error occurred while preparing the short password for storage:")
		t.Fatal(err)
	}

	if len(encodedShortPassword) != expectedAESLength {
		t.Fatalf("expected prepared password length to be %d bytes but it was %d bytes long instead.", expectedAESLength, len(encodedShortPassword))
	}

	err = ComparePassword(encodedShortPassword, rawShortPassword)
	if err != nil {
		t.Log("an error occurred while comparing the short password:")
		t.Fatal(err)
	}

	rawLongPassword := "this password would normally get truncated by bcrypt but we use sha256 to avoid losing that information."
	encodedLongPassword, err := PreparePassword(rawLongPassword)
	if err != nil {
		t.Log("an error occurred while preparing the long password for storage:")
		t.Fatal(err)
	}

	if len(encodedLongPassword) != expectedAESLength {
		t.Fatalf("expected prepared password length to be %d bytes but it was %d bytes long instead.", expectedAESLength, len(encodedLongPassword))
	}

	err = ComparePassword(encodedLongPassword, rawLongPassword)
	if err != nil {
		t.Log("an error occurred while comparing the long password:")
		t.Fatal(err)
	}
}
