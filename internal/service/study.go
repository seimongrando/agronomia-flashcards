package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
		UserID:     userID,
		CursorName: cursorName,
		CursorID:   cursorID,
		ShowAll:    showAll,
		HideEmpty:  true, // home page never shows empty decks regardless of role
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
// Returns sql.ErrNoRows (wrapped) when no card is available.
func (s *StudyService) NextCard(ctx context.Context, userID, deckID, mode, topic string) (model.Card, error) {
	switch mode {
	case "random":
		return s.study.NextRandomCard(ctx, deckID, topic)
	case "wrong":
		card, err := s.study.NextWrongCard(ctx, userID, deckID, topic)
		if errors.Is(err, sql.ErrNoRows) {
			return s.study.NextDueCard(ctx, userID, deckID, topic)
		}
		return card, err
	default:
		return s.study.NextDueCard(ctx, userID, deckID, topic)
	}
}

// Progress returns global study statistics for a user.
func (s *StudyService) Progress(ctx context.Context, userID string) (model.ProgressStats, error) {
	return s.study.GlobalProgress(ctx, userID)
}

// Topics returns distinct topic values for cards in a deck.
func (s *StudyService) Topics(ctx context.Context, deckID string) ([]string, error) {
	return s.study.DeckTopics(ctx, deckID)
}

func (s *StudyService) SubmitAnswer(ctx context.Context, userID, cardID string, result int) (model.AnswerResponse, error) {
	if _, err := s.cards.FindByID(ctx, cardID); err != nil {
		return model.AnswerResponse{}, fmt.Errorf("card not found: %w", err)
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
	return s.study.Stats(ctx, userID, deckID)
}

func (s *StudyService) ProfessorStats(ctx context.Context) (model.ProfessorStats, error) {
	return s.study.ProfessorStats(ctx)
}
