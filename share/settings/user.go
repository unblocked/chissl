package settings

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var UserAllowAll = regexp.MustCompile(".*")

func ParseAuth(auth string) (string, string) {
	if strings.Contains(auth, ":") {
		pair := strings.SplitN(auth, ":", 2)
		return pair[0], pair[1]
	}
	return "", ""
}

type User struct {
	Name    string           `json:"username"`
	Pass    string           `json:"password"`
	Addrs   []*regexp.Regexp `json:"addresses"`
	IsAdmin bool             `json:"is_admin"`
}

func (u *User) HasAccess(addr string) bool {
	m := false
	for _, r := range u.Addrs {
		if r.MatchString(addr) {
			m = true
			break
		}
	}
	return m
}

// ValidateUser validates the fields of the User struct
func (u *User) ValidateUser() error {
	// Validate Name: alphanumeric, no special characters
	for _, r := range u.Name {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			return errors.New("name must be alphanumeric with no special characters")
		}
	}

	// Validate Password: minimum length of 8 characters
	if len(u.Pass) < 8 {
		return errors.New("password must have a minimum length of 8 characters")
	}

	// Validate Addrs: each address must have a minimum length of 1
	if len(u.Addrs) == 0 {
		return errors.New("at least one address must be provided")
	}
	for _, r := range u.Addrs {
		if len(r.String()) == 0 {
			return errors.New("address regex must not be empty. supply '.*' to match all")
		}
	}

	return nil
}

func (u *User) ToJSON() (string, error) {
	jsonData, err := json.Marshal(u)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// RedactedUser is the public representation of a User: it never carries the password.
type RedactedUser struct {
	Name    string   `json:"username"`
	Addrs   []string `json:"addresses"`
	IsAdmin bool     `json:"is_admin"`
}

// Redacted returns a copy of the user safe to return from the API.
func (u *User) Redacted() RedactedUser {
	addrs := make([]string, 0, len(u.Addrs))
	for _, a := range u.Addrs {
		addrs = append(addrs, a.String())
	}
	return RedactedUser{Name: u.Name, Addrs: addrs, IsAdmin: u.IsAdmin}
}
