package static

import (
	"encoding/json"
	"fmt"
	"os"
)

func NewAuthFromConfig(config *Config) (*Auth, error) {
	data, err := os.ReadFile(config.UsersJsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read static auth config: %v", err)
	}
	auth := &Auth{}
	if err := json.Unmarshal(data, &auth.Users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal static auth config: %v", err)
	}
	if len(auth.Users) == 0 {
		return nil, fmt.Errorf("no users found in static auth config")
	}
	return auth, nil
}
