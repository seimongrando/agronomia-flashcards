package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"webapp/internal/model"
	"webapp/internal/pagination"
	"webapp/internal/repository"
)

type AdminService struct {
	users  *repository.UserRepo
	logger *slog.Logger
}

func NewAdminService(users *repository.UserRepo, logger *slog.Logger) *AdminService {
	return &AdminService{users: users, logger: logger}
}

// ListUsers returns a paginated page of users. Pass a zero cursorTS for the first page.
func (s *AdminService) ListUsers(
	ctx context.Context,
	emailQuery string,
	cursorTS time.Time, cursorID string,
	limit int,
) (pagination.Page[model.UserWithRoles], error) {
	users, err := s.users.ListUsersWithRolesPaged(ctx, repository.UserListParams{
		EmailQuery: emailQuery,
		CursorTS:   cursorTS,
		CursorID:   cursorID,
		Limit:      limit + 1, // fetch one extra to detect next page
	})
	if err != nil {
		return pagination.Page[model.UserWithRoles]{}, err
	}

	var nextCursor *string
	if len(users) > limit {
		users = users[:limit]
		last := users[len(users)-1]
		c := pagination.EncodeTimestampIDCursor(last.CreatedAt, last.ID)
		nextCursor = &c
	}

	if users == nil {
		users = []model.UserWithRoles{}
	}

	return pagination.Page[model.UserWithRoles]{
		Items:      users,
		NextCursor: nextCursor,
	}, nil
}

// SetRoles adds and/or removes roles for a target user.
// It guarantees the user always retains at least one role (student as fallback).
func (s *AdminService) SetRoles(
	ctx context.Context,
	adminUserID, targetUserID string,
	add, remove []string,
) ([]string, error) {
	if _, err := s.users.FindByID(ctx, targetUserID); err != nil {
		return nil, fmt.Errorf("target user not found: %w", err)
	}

	for _, r := range add {
		role := model.Role(r)
		if !role.Valid() {
			return nil, fmt.Errorf("invalid role: %s", r)
		}
	}
	for _, r := range remove {
		role := model.Role(r)
		if !role.Valid() {
			return nil, fmt.Errorf("invalid role: %s", r)
		}
	}

	var added, removed []string

	for _, r := range add {
		if err := s.users.AddRole(ctx, targetUserID, model.Role(r)); err != nil {
			return nil, fmt.Errorf("add role %s: %w", r, err)
		}
		added = append(added, r)
	}

	for _, r := range remove {
		current, err := s.users.RolesByUserID(ctx, targetUserID)
		if err != nil {
			return nil, fmt.Errorf("check roles: %w", err)
		}
		if len(current) <= 1 {
			return nil, fmt.Errorf("cannot remove role %q: user must retain at least one role", r)
		}
		if err := s.users.RemoveRole(ctx, targetUserID, model.Role(r)); err != nil {
			continue
		}
		removed = append(removed, r)
	}

	s.logger.Info("admin role change",
		"admin_user_id", adminUserID,
		"target_user_id", targetUserID,
		"roles_added", added,
		"roles_removed", removed,
	)

	final, err := s.users.RolesByUserID(ctx, targetUserID)
	if err != nil {
		return nil, fmt.Errorf("fetch final roles: %w", err)
	}
	out := make([]string, len(final))
	for i, role := range final {
		out[i] = string(role)
	}
	return out, nil
}
