package entity

// Caller is a verified identity, populated entirely from a provider's signed token payload.
type Caller struct {
	UID   string `json:"uid"`
	Email string `json:"email"`
	Name  string `json:"name"`
}
