package dbauth

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestDummyPasswordHashIsValidAtBcryptCost guards the unknown-user timing
// defense in AuthenticatePassword (yjr3): the dummy hash must parse as a real
// bcrypt hash at BcryptCost, or the comparison short-circuits without doing
// the key expansion a known-user wrong-password attempt pays for.
func TestDummyPasswordHashIsValidAtBcryptCost(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyPasswordHash))
	if err != nil {
		t.Fatalf("dummy password hash must be a parseable bcrypt hash, got: %v", err)
	}
	if cost != BcryptCost {
		t.Errorf("dummy password hash must use BcryptCost (%d), got cost %d", BcryptCost, cost)
	}
}

// TestDummyPasswordHashComparisonReachesMismatch asserts the dummy comparison
// ends in a normal password mismatch rather than a hash-parse rejection
// (ErrHashTooShort and friends return before any bcrypt work).
func TestDummyPasswordHashComparisonReachesMismatch(t *testing.T) {
	for _, password := range []string{"", "anything", "testpassword123"} {
		err := bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
		if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			t.Errorf("comparing %q against the dummy hash must return ErrMismatchedHashAndPassword, got: %v", password, err)
		}
	}
}

// TestBcryptCostIsTwelve pins the shared hashing cost: lowering it weakens
// stored hashes, and changing it without regenerating dummyPasswordHash
// reopens the timing gap.
func TestBcryptCostIsTwelve(t *testing.T) {
	if BcryptCost != 12 {
		t.Errorf("BcryptCost must be 12, got %d", BcryptCost)
	}
}
