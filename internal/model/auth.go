package model

// AuthInfo is the identity extracted from a validated JWT and stored in request context.
type AuthInfo struct {
	UserID  string   `json:"user_id"`
	Email   string   `json:"email"`
	Name    string   `json:"name"`
	Picture string   `json:"picture,omitempty"`
	Roles   []string `json:"roles"`
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
