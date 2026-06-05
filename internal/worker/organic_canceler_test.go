package worker_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
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

var errFindFailed = &testError{"find failed"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
