package entity

// Caller is a verified identity, populated entirely from a provider's signed token payload.
type Caller struct {
	UID   string `json:"uid"`
	Email string `json:"email"`
	Name  string `json:"name"`
	// EmailVerified is the provider's own assertion that this caller controls Email, carried
	// separately because Email alone is not evidence of anything: a provider that lets an account
	// hold an unverified address would otherwise let it hold somebody else's. Anything that treats
	// an address as an identity - the assistant allowlist does - has to check this first.
	EmailVerified bool `json:"emailVerified"`
}
