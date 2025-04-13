package static

type Auth struct {
	Users map[string]string `json:"users"`
}

type Config struct {
	UsersJsonPath string
}
