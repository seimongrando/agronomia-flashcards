package model

// AuthInfo is the identity extracted from a validated JWT and stored in request context.
// Only UserID and Roles are stored — no PII travels via the context chain.
// Profile data (name, email, picture) must be fetched from the database when needed.
type AuthInfo struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
}

// HasAnyRole returns true if the user holds at least one of the given roles.
func (a AuthInfo) HasAnyRole(roles ...string) bool {
	for _, required := range roles {
		for _, have := range a.Roles {
			if have == required {
				return true
			}
		}
	}
	return false
}

// GoogleProfile represents the user info returned by Google's userinfo endpoint.
type GoogleProfile struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}
