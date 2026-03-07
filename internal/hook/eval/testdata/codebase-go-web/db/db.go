package db

// User represents a user record.
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// GetUsers returns all users.
func GetUsers() []User {
	return []User{
		{ID: 1, Name: "Alice"},
		{ID: 2, Name: "Bob"},
	}
}
