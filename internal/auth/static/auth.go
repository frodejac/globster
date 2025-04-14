package static

import (
	"golang.org/x/crypto/bcrypt"
)

func (a *Auth) Validate(username, password string) bool {
	hash, _ := a.Users[username]

	// We want to do a constant time comparison to prevent timing attacks, even if the password is empty
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return false
	}
	return true
}
