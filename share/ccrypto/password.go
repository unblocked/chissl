package ccrypto

import (
	"crypto/subtle"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// passwordHashCost is the bcrypt work factor used for newly hashed passwords.
const passwordHashCost = 12

// HashPassword returns a bcrypt hash of the given password suitable for storage.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", bcrypt.ErrHashTooShort
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordHashCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// IsPasswordHash reports whether a stored value is a bcrypt hash produced by HashPassword.
func IsPasswordHash(stored string) bool {
	return strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") || strings.HasPrefix(stored, "$2y$")
}

// CheckPassword verifies a password against a stored value.
//
// The stored value is either a bcrypt hash or, for rows written before hashing
// was introduced, the legacy plaintext password. An empty stored value or an
// empty candidate never matches, so accounts provisioned by SSO (which have no
// usable password) cannot be logged into with a password.
//
// ok reports whether the password matched. legacy reports that the match was
// against a plaintext value and the caller should re-store it as a hash.
func CheckPassword(stored, password string) (ok bool, legacy bool) {
	if stored == "" || password == "" {
		return false, false
	}
	if IsPasswordHash(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil, false
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(password)) == 1, true
}

// ConstantTimeEquals compares two strings in constant time.
func ConstantTimeEquals(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
