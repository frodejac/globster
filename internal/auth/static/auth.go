package static

import (
	"golang.org/x/crypto/bcrypt"
	"log"
)

func (a *Auth) Validate(username, password string) bool {
	hash, ok := a.Users[username]
	if !ok {
		log.Printf("User %s not found", username)
	}
	// We want to do a constant time comparison to prevent timing attacks, even if the password is empty
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		log.Printf("Password mismatch for user %s: %v", username, err)
		return false
	}
	log.Printf("User %s authenticated successfully", username)
	return true
}
