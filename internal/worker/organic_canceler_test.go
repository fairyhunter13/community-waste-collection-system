package worker_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/config"
	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/mocks"
	"github.com/fairyhunter13/community-waste-collection-system/internal/worker"
)

type OrganicCancelerSuite struct {
	suite.Suite
	repo   *mocks.PickupRepository
	logger *slog.Logger
	cfg    *config.Config
}

func (s *OrganicCancelerSuite) SetupTest() {
	s.repo = mocks.NewPickupRepository(s.T())
	s.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	s.cfg = &config.Config{
		WorkerCancelInterval:    100 * time.Millisecond,
		WorkerOrganicCutoffDays: 3,
	}
}

func TestOrganicCanceler(t *testing.T) {
	suite.Run(t, new(OrganicCancelerSuite))
}

func (s *OrganicCancelerSuite) TestRun_CancelsStalePickups() {
	id1, id2 := uuid.New(), uuid.New()
	pickups := []*domain.WastePickup{{ID: id1}, {ID: id2}}
	ids := []uuid.UUID{id1, id2}

	s.repo.On("FindExpiredOrganic", mock.Anything, mock.AnythingOfType("time.Time")).Return(pickups, nil)
	s.repo.On("BulkCancel", mock.Anything, ids).Return(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	w := worker.NewOrganicCanceler(s.repo, s.logger, s.cfg)
	w.Start(ctx)

	s.repo.AssertCalled(s.T(), "FindExpiredOrganic", mock.Anything, mock.AnythingOfType("time.Time"))
	s.repo.AssertCalled(s.T(), "BulkCancel", mock.Anything, ids)
}

func (s *OrganicCancelerSuite) TestRun_NoStalePickups_SkipsBulkCancel() {
	s.repo.On("FindExpiredOrganic", mock.Anything, mock.AnythingOfType("time.Time")).Return([]*domain.WastePickup{}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	w := worker.NewOrganicCanceler(s.repo, s.logger, s.cfg)
	w.Start(ctx)

	s.repo.AssertNotCalled(s.T(), "BulkCancel")
}

func (s *OrganicCancelerSuite) TestRun_FindError_SkipsBulkCancel() {
	s.repo.On("FindExpiredOrganic", mock.Anything, mock.AnythingOfType("time.Time")).
		Return(nil, errFindFailed)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	w := worker.NewOrganicCanceler(s.repo, s.logger, s.cfg)
	w.Start(ctx)

	s.repo.AssertNotCalled(s.T(), "BulkCancel")
}

func (s *OrganicCancelerSuite) TestRun_BulkCancelError_LogsAndReturns() {
	id := uuid.New()
	s.repo.On("FindExpiredOrganic", mock.Anything, mock.AnythingOfType("time.Time")).
		Return([]*domain.WastePickup{{ID: id}}, nil)
	s.repo.On("BulkCancel", mock.Anything, []uuid.UUID{id}).Return(errors.New("db error"))

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	w := worker.NewOrganicCanceler(s.repo, s.logger, s.cfg)
	w.Start(ctx) // must not panic

	s.repo.AssertCalled(s.T(), "BulkCancel", mock.Anything, []uuid.UUID{id})
}

// U1: context cancel must cause Start to return within a generous deadline.
func (s *OrganicCancelerSuite) TestContextCancelExitsPromptly() {
	// Set a long tick interval so the goroutine blocks inside the select.
	s.cfg.WorkerCancelInterval = 10 * time.Second
	w := worker.NewOrganicCanceler(s.repo, s.logger, s.cfg)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	// Give Start time to enter its select loop, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// exited promptly — pass
	case <-time.After(2 * time.Second):
		s.Fail("Start did not exit within 2s after context cancellation")
	}
}

// TestRunWithRecover_PanicDoesNotKillWorker verifies that a panic inside a
// worker cycle is caught by runWithRecover and that the loop continues until
// the context is cancelled.
func (s *OrganicCancelerSuite) TestRunWithRecover_PanicDoesNotKillWorker() {
	var firstCall atomic.Bool
	s.repo.On("FindExpiredOrganic", mock.Anything, mock.AnythingOfType("time.Time")).
		Run(func(_ mock.Arguments) {
			if firstCall.CompareAndSwap(false, true) {
				panic("simulated db panic")
			}
		}).
		Return([]*domain.WastePickup{}, nil).
		Maybe()

	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		w := worker.NewOrganicCanceler(s.repo, s.logger, s.cfg)
		w.Start(ctx)
	}()

	select {
	case <-done:
		// Start returned after ctx cancelled — panic was recovered.
	case <-time.After(3 * time.Second):
		s.Fail("Start hung; runWithRecover likely did not recover the panic")
	}
}

var errFindFailed = &testError{"find failed"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
