package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"webapp/internal/middleware"
	"webapp/internal/model"
	"webapp/internal/pagination"
	"webapp/internal/repository"
)

const (
	sm2InitialEF = 2.5
	sm2MinEF     = 1.3
	sm2MaxEF     = 2.5
)

// ScheduleResult holds the computed spaced repetition values.
type ScheduleResult struct {
	NextDue      time.Time
	Streak       int
	LastResult   int16
	EaseFactor   float64
	IntervalDays int
}

// Schedule implements a simplified SM-2 spaced-repetition algorithm.
//
// Ratings:
//
//	0 = wrong  → reset to 1 day; EF decreases slightly
//	1 = hard   → interval grows slowly (×1.2); EF decreases slightly; streak kept
//	2 = correct → interval grows by EF; EF increases slightly; streak grows
//
// First two correct answers use fixed intervals (1 d, 6 d) to bootstrap the
// sequence before exponential growth kicks in.
func Schedule(result int, streak int, intervalDays int, easeFactor float64, now time.Time) ScheduleResult {
	if easeFactor <= 0 {
		easeFactor = sm2InitialEF
	}
	if intervalDays <= 0 {
		intervalDays = 1
	}

	var nextInterval int
	var newEF float64
	var newStreak int

	switch result {
	case 0: // wrong
		nextInterval = 1
		newEF = clampEF(easeFactor - 0.20)
		newStreak = 0
	case 1: // hard — counts as a partial pass
		nextInterval = max1(round(float64(intervalDays) * 1.2))
		newEF = clampEF(easeFactor - 0.15)
		newStreak = streak + 1
	default: // correct (2)
		switch streak {
		case 0:
			nextInterval = 1
		case 1:
			nextInterval = 6
		default:
			nextInterval = max1(round(float64(intervalDays) * easeFactor))
		}
		newEF = clampEF(easeFactor + 0.10)
		newStreak = streak + 1
	}

	return ScheduleResult{
		NextDue:      now.Add(time.Duration(nextInterval) * 24 * time.Hour),
		Streak:       newStreak,
		LastResult:   int16(result),
		EaseFactor:   newEF,
		IntervalDays: nextInterval,
	}
}

func clampEF(ef float64) float64 {
	if ef < sm2MinEF {
		return sm2MinEF
	}
	if ef > sm2MaxEF {
		return sm2MaxEF
	}
	return ef
}

func round(f float64) int {
	return int(f + 0.5)
}

func max1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

type StudyService struct {
	study   *repository.StudyRepo
	cards   *repository.CardRepo
	reviews *repository.ReviewRepo
}

func NewStudyService(study *repository.StudyRepo, cards *repository.CardRepo, reviews *repository.ReviewRepo) *StudyService {
	return &StudyService{study: study, cards: cards, reviews: reviews}
}

// studyAuthFromCtx extracts auth info that was set by the auth middleware.
func studyAuthFromCtx(ctx context.Context) model.AuthInfo {
	info, _ := middleware.GetAuthInfo(ctx)
	return info
}

// checkDeckAccess returns ErrForbidden if the calling user has no right to study
// from the given deck. Staff (professor, admin) always pass.
func (s *StudyService) checkDeckAccess(ctx context.Context, userID, deckID string) error {
	auth := studyAuthFromCtx(ctx)
	if auth.HasAnyRole("professor", "admin") {
		return nil
	}
	ok, err := s.study.DeckAccessible(ctx, userID, deckID)
	if err != nil {
		return fmt.Errorf("check deck access: %w", err)
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

// ListDecks returns a paginated page of decks for the home page.
// Empty decks are always hidden. Inactive/expired decks are hidden for students
// but visible for professors and admins (showAll=true).
func (s *StudyService) ListDecks(
	ctx context.Context,
	userID, cursorName, cursorID string,
	limit int,
	showAll bool,
) (pagination.Page[model.DeckWithCounts], error) {
	decks, err := s.study.ListDecksWithCountsPaged(ctx, repository.DeckListParams{
		UserID:          userID,
		CursorName:      cursorName,
		CursorID:        cursorID,
		ShowAll:         showAll,
		HideEmpty:       true,     // home page never shows empty decks
		ApplyVisibility: !showAll, // students see only their accessible decks
		IncludeHidden:   true,     // return hidden decks so the client can show the hidden section
		Limit:           limit + 1,
	})
	if err != nil {
		return pagination.Page[model.DeckWithCounts]{}, err
	}

	var nextCursor *string
	if len(decks) > limit {
		decks = decks[:limit]
		last := decks[len(decks)-1]
		c := pagination.EncodeNameIDCursor(last.Name, last.ID)
		nextCursor = &c
	}

	if decks == nil {
		decks = []model.DeckWithCounts{}
	}

	// Strip internal ownership field when returning the student home view.
	// showAll=false means this is a student-facing request; CreatedBy is only
	// needed by staff for UI ownership checks (teach/manage pages).
	if !showAll {
		for i := range decks {
			decks[i].CreatedBy = nil
		}
	}

	return pagination.Page[model.DeckWithCounts]{
		Items:      decks,
		NextCursor: nextCursor,
	}, nil
}

// ListDecksForManagement returns all decks (including empty and inactive ones)
// for professor/admin management views. Unlike ListDecks, it never hides empty
// decks so professors can see and populate newly created decks.
func (s *StudyService) ListDecksForManagement(
	ctx context.Context,
	userID, cursorName, cursorID string,
	limit int,
) (pagination.Page[model.DeckWithCounts], error) {
	decks, err := s.study.ListDecksWithCountsPaged(ctx, repository.DeckListParams{
		UserID:     userID,
		CursorName: cursorName,
		CursorID:   cursorID,
		ShowAll:    true,  // include inactive/expired decks
		HideEmpty:  false, // include empty decks so professors can add cards
		Limit:      limit + 1,
	})
	if err != nil {
		return pagination.Page[model.DeckWithCounts]{}, err
	}

	var nextCursor *string
	if len(decks) > limit {
		decks = decks[:limit]
		last := decks[len(decks)-1]
		c := pagination.EncodeNameIDCursor(last.Name, last.ID)
		nextCursor = &c
	}

	if decks == nil {
		decks = []model.DeckWithCounts{}
	}

	return pagination.Page[model.DeckWithCounts]{
		Items:      decks,
		NextCursor: nextCursor,
	}, nil
}

// NextCard returns the next card to study based on the selected mode.
// Pass topic="" to study all topics.
// Pass excludeIDs to skip cards already seen this session (applied to all modes).
// Returns sql.ErrNoRows (wrapped) when no card is available.
func (s *StudyService) NextCard(ctx context.Context, userID, deckID, mode, topic string, excludeIDs []string) (model.Card, error) {
	if err := s.checkDeckAccess(ctx, userID, deckID); err != nil {
		return model.Card{}, err
	}
	switch mode {
	case "random":
		return s.study.NextRandomCard(ctx, deckID, topic, excludeIDs)
	case "wrong":
		card, err := s.study.NextWrongCard(ctx, userID, deckID, topic, excludeIDs)
		if errors.Is(err, sql.ErrNoRows) {
			return s.study.NextDueCard(ctx, userID, deckID, topic, excludeIDs)
		}
		return card, err
	default:
		return s.study.NextDueCard(ctx, userID, deckID, topic, excludeIDs)
	}
}

// Progress returns global study statistics for a user.
func (s *StudyService) Progress(ctx context.Context, userID string) (model.ProgressStats, error) {
	return s.study.GlobalProgress(ctx, userID)
}

// Topics returns distinct topic values for cards in a deck.
func (s *StudyService) Topics(ctx context.Context, userID, deckID string) ([]string, error) {
	if err := s.checkDeckAccess(ctx, userID, deckID); err != nil {
		return nil, err
	}
	return s.study.DeckTopics(ctx, deckID)
}

func (s *StudyService) SubmitAnswer(ctx context.Context, userID, cardID string, result int) (model.AnswerResponse, error) {
	// Verify the card exists and the user has access to its deck.
	deckID, err := s.study.CardDeckID(ctx, cardID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.AnswerResponse{}, sql.ErrNoRows
		}
		return model.AnswerResponse{}, fmt.Errorf("lookup card deck: %w", err)
	}
	if err := s.checkDeckAccess(ctx, userID, deckID); err != nil {
		return model.AnswerResponse{}, err
	}

	var streak, intervalDays int
	var easeFactor float64
	existing, err := s.reviews.FindByUserAndCard(ctx, userID, cardID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.AnswerResponse{}, fmt.Errorf("lookup review: %w", err)
	}
	if err == nil {
		streak = existing.Streak
		intervalDays = existing.IntervalDays
		easeFactor = existing.EaseFactor
	} else {
		easeFactor = sm2InitialEF
		intervalDays = 1
	}

	sched := Schedule(result, streak, intervalDays, easeFactor, time.Now())

	_, err = s.reviews.Upsert(ctx, model.Review{
		UserID:       userID,
		CardID:       cardID,
		NextDue:      sched.NextDue,
		LastResult:   sched.LastResult,
		Streak:       sched.Streak,
		EaseFactor:   sched.EaseFactor,
		IntervalDays: sched.IntervalDays,
	})
	if err != nil {
		return model.AnswerResponse{}, fmt.Errorf("save review: %w", err)
	}

	return model.AnswerResponse{
		NextDue:      sched.NextDue,
		Streak:       sched.Streak,
		IntervalDays: sched.IntervalDays,
	}, nil
}

func (s *StudyService) Stats(ctx context.Context, userID, deckID string) (model.StudyStats, error) {
	if err := s.checkDeckAccess(ctx, userID, deckID); err != nil {
		return model.StudyStats{}, err
	}
	return s.study.Stats(ctx, userID, deckID)
}

func (s *StudyService) ProfessorStats(ctx context.Context) (model.ProfessorStats, error) {
	return s.study.ProfessorStats(ctx)
}

// OfflineBundle returns all cards for a deck plus the user's review state,
// packaged for IndexedDB caching so the student can study without network.
func (s *StudyService) OfflineBundle(ctx context.Context, userID, deckID string) (model.OfflineBundle, error) {
	if err := s.checkDeckAccess(ctx, userID, deckID); err != nil {
		return model.OfflineBundle{}, err
	}
	return s.study.GetOfflineBundle(ctx, userID, deckID)
}

// HideDeck sets or clears the hidden flag for a general deck on the student's home page.
// Only general (non-private, non-class) decks may be hidden; staff bypass the check.
func (s *StudyService) HideDeck(ctx context.Context, userID, deckID string, hide bool) error {
	auth := studyAuthFromCtx(ctx)
	if !auth.HasAnyRole("professor", "admin") {
		// Validate that the deck exists and is a general deck.
		ok, err := s.study.DeckAccessible(ctx, userID, deckID)
		if err != nil {
			return fmt.Errorf("check deck: %w", err)
		}
		if !ok {
			return ErrForbidden
		}
	}
	return s.study.HideDeck(ctx, userID, deckID, hide)
}
