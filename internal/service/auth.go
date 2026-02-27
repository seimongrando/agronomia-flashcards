package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"webapp/internal/model"
	"webapp/internal/repository"
)

type tokenClaims struct {
	jwt.RegisteredClaims
	Email   string   `json:"email,omitempty"`
	Name    string   `json:"name,omitempty"`
	Picture string   `json:"picture,omitempty"`
	Roles   []string `json:"roles"`
}

type AuthService struct {
	users       *repository.UserRepo
	jwtSecret   []byte
	jwtExpiry   time.Duration
	adminEmails map[string]bool
}

func NewAuthService(
	users *repository.UserRepo,
	jwtSecret string,
	jwtExpiry time.Duration,
	adminEmails map[string]bool,
) *AuthService {
	return &AuthService{
		users:       users,
		jwtSecret:   []byte(jwtSecret),
		jwtExpiry:   jwtExpiry,
		adminEmails: adminEmails,
	}
}

func (s *AuthService) TokenExpiry() time.Duration {
	return s.jwtExpiry
}

// LoginWithGoogle upserts the user, assigns roles, and returns a signed JWT.
func (s *AuthService) LoginWithGoogle(ctx context.Context, profile model.GoogleProfile) (string, error) {
	var pic *string
	if profile.Picture != "" {
		pic = &profile.Picture
	}

	user, err := s.users.UpsertByGoogleSub(ctx, model.User{
		GoogleSub:  profile.Sub,
		Email:      profile.Email,
		Name:       profile.Name,
		PictureURL: pic,
	})
	if err != nil {
		return "", fmt.Errorf("upsert user: %w", err)
	}

	if s.adminEmails[strings.ToLower(user.Email)] {
		if err := s.users.AddRole(ctx, user.ID, model.RoleAdmin); err != nil {
			return "", fmt.Errorf("add admin role: %w", err)
		}
	}

	roles, err := s.users.RolesByUserID(ctx, user.ID)
	if err != nil {
		return "", fmt.Errorf("get roles: %w", err)
	}

	if len(roles) == 0 {
		if err := s.users.AddRole(ctx, user.ID, model.RoleStudent); err != nil {
			return "", fmt.Errorf("add default role: %w", err)
		}
		roles = []model.Role{model.RoleStudent}
	}

	now := time.Now()
	roleStrs := make([]string, len(roles))
	for i, r := range roles {
		roleStrs[i] = string(r)
	}

	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtExpiry)),
		},
		Email: user.Email,
		Name:  user.Name,
		Roles: roleStrs,
	}
	if user.PictureURL != nil {
		claims.Picture = *user.PictureURL
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}
