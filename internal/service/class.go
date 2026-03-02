package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"webapp/internal/middleware"
	"webapp/internal/model"
	"webapp/internal/repository"
)

var ErrClassNotFound = errors.New("class not found")
var ErrAlreadyMember = errors.New("already enrolled in this class")
var ErrClassInactive = errors.New("class is not active")

type ClassService struct {
	classes    *repository.ClassRepo
	classStats *repository.ClassStatsRepo
}

func NewClassService(classes *repository.ClassRepo, classStats *repository.ClassStatsRepo) *ClassService {
	return &ClassService{classes: classes, classStats: classStats}
}

func classAuthFromCtx(ctx context.Context) model.AuthInfo {
	info, _ := middleware.GetAuthInfo(ctx)
	return info
}

// CreateClass creates a new class. Only professor/admin.
func (s *ClassService) CreateClass(ctx context.Context, name string, description *string) (model.Class, error) {
	auth := classAuthFromCtx(ctx)
	name = strings.TrimSpace(name)
	return s.classes.Create(ctx, name, description, auth.UserID)
}

// ListMyClasses returns classes relevant to the caller:
//   - professor/admin → classes they created (includes invite_code)
//   - student         → classes they are enrolled in (no invite_code)
func (s *ClassService) ListMyClasses(ctx context.Context) ([]model.ClassSummary, error) {
	auth := classAuthFromCtx(ctx)
	if auth.HasAnyRole("professor", "admin") {
		return s.classes.ListByCreator(ctx, auth.UserID)
	}
	return s.classes.ListByMember(ctx, auth.UserID)
}

// GetClass returns a class, enforcing that the caller is the owner (prof/admin).
func (s *ClassService) GetClass(ctx context.Context, classID string) (model.Class, error) {
	auth := classAuthFromCtx(ctx)
	cl, err := s.classes.FindByID(ctx, classID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Class{}, ErrClassNotFound
		}
		return model.Class{}, err
	}
	if !auth.HasAnyRole("admin") && cl.CreatedBy != auth.UserID {
		return model.Class{}, ErrForbidden
	}
	return cl, nil
}

// UpdateClass updates a class name/description/active status (owner or admin only).
func (s *ClassService) UpdateClass(ctx context.Context, classID, name string, description *string, isActive bool) (model.Class, error) {
	if _, err := s.GetClass(ctx, classID); err != nil {
		return model.Class{}, err
	}
	return s.classes.Update(ctx, classID, strings.TrimSpace(name), description, isActive)
}

// DeleteClass deletes a class (owner or admin only).
func (s *ClassService) DeleteClass(ctx context.Context, classID string) error {
	if _, err := s.GetClass(ctx, classID); err != nil {
		return err
	}
	return s.classes.Delete(ctx, classID)
}

// RegenerateInviteCode issues a new invite code (owner or admin only).
func (s *ClassService) RegenerateInviteCode(ctx context.Context, classID string) (string, error) {
	if _, err := s.GetClass(ctx, classID); err != nil {
		return "", err
	}
	return s.classes.RegenerateInviteCode(ctx, classID)
}

// JoinClass enrolls the calling student using an invite code.
func (s *ClassService) JoinClass(ctx context.Context, inviteCode string) (model.Class, error) {
	auth := classAuthFromCtx(ctx)
	cl, err := s.classes.FindByInviteCode(ctx, strings.ToUpper(strings.TrimSpace(inviteCode)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Class{}, ErrClassNotFound
		}
		return model.Class{}, err
	}
	if !cl.IsActive {
		return model.Class{}, ErrClassInactive
	}
	already, err := s.classes.IsMember(ctx, cl.ID, auth.UserID)
	if err != nil {
		return model.Class{}, err
	}
	if already {
		return model.Class{}, ErrAlreadyMember
	}
	if err := s.classes.AddMember(ctx, cl.ID, auth.UserID); err != nil {
		return model.Class{}, err
	}
	// Return class without invite_code — student view
	cl.InviteCode = ""
	return cl, nil
}

// LeaveClass removes the calling student from a class.
func (s *ClassService) LeaveClass(ctx context.Context, classID string) error {
	auth := classAuthFromCtx(ctx)
	return s.classes.RemoveMember(ctx, classID, auth.UserID)
}

// AssignDeck adds a deck to a class (owner or admin only).
func (s *ClassService) AssignDeck(ctx context.Context, classID, deckID string) error {
	if _, err := s.GetClass(ctx, classID); err != nil {
		return err
	}
	return s.classes.AddDeck(ctx, classID, deckID)
}

// UnassignDeck removes a deck from a class (owner or admin only).
func (s *ClassService) UnassignDeck(ctx context.Context, classID, deckID string) error {
	if _, err := s.GetClass(ctx, classID); err != nil {
		return err
	}
	return s.classes.RemoveDeck(ctx, classID, deckID)
}

// ListClassDecks returns decks assigned to a class.
// Caller must be the class owner or a member.
func (s *ClassService) ListClassDecks(ctx context.Context, classID string) ([]model.ClassDeckSummary, error) {
	auth := classAuthFromCtx(ctx)
	cl, err := s.classes.FindByID(ctx, classID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrClassNotFound
		}
		return nil, err
	}
	// Professors see only their own classes; students must be enrolled.
	if auth.HasAnyRole("admin") {
		// admin sees all
	} else if auth.HasAnyRole("professor") {
		if cl.CreatedBy != auth.UserID {
			return nil, ErrForbidden
		}
	} else {
		isMember, err := s.classes.IsMember(ctx, classID, auth.UserID)
		if err != nil || !isMember {
			return nil, ErrForbidden
		}
	}
	return s.classes.ListClassDecks(ctx, classID)
}

// --- Analytics ---

// GetClassStats returns the full performance report for a class (owner or admin).
func (s *ClassService) GetClassStats(ctx context.Context, classID string) (model.ClassStats, error) {
	if _, err := s.GetClass(ctx, classID); err != nil {
		return model.ClassStats{}, err
	}
	return s.classStats.GetClassStats(ctx, classID)
}

// GetClassOverview returns a compact summary for all classes created by the caller
// (or all classes for admin).
func (s *ClassService) GetClassOverview(ctx context.Context) ([]model.ClassOverviewItem, error) {
	auth := classAuthFromCtx(ctx)
	if auth.HasAnyRole("admin") {
		return s.classStats.ListAllClassOverview(ctx)
	}
	return s.classStats.ListClassOverview(ctx, auth.UserID)
}
