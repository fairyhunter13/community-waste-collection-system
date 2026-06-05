package service_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/mocks"
	"github.com/fairyhunter13/community-waste-collection-system/internal/service"
)

type HouseholdServiceSuite struct {
	suite.Suite
	repo *mocks.HouseholdRepository
	svc  domain.HouseholdService
}

func (s *HouseholdServiceSuite) SetupTest() {
	s.repo = mocks.NewHouseholdRepository(s.T())
	s.svc = service.NewHouseholdService(s.repo)
}

func TestHouseholdService(t *testing.T) {
	suite.Run(t, new(HouseholdServiceSuite))
}

func (s *HouseholdServiceSuite) TestCreate_Success() {
	s.repo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Household")).
		Run(func(args mock.Arguments) {
			h := args.Get(1).(*domain.Household)
			h.ID = uuid.New()
		}).
		Return(nil)

	req := domain.CreateHouseholdRequest{OwnerName: "John", Address: "Jl. Merdeka 1"}
	got, err := s.svc.Create(s.T().Context(), req)
	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Equal("John", got.OwnerName)
	s.NotEqual(uuid.Nil, got.ID)
}

func (s *HouseholdServiceSuite) TestCreate_RepositoryError_Propagates() {
	s.repo.On("Create", mock.Anything, mock.Anything).Return(domain.ErrInternalFailure)

	_, err := s.svc.Create(s.T().Context(), domain.CreateHouseholdRequest{OwnerName: "X", Address: "Y"})
	s.Require().ErrorIs(err, domain.ErrInternalFailure)
}

func (s *HouseholdServiceSuite) TestGetByID_Found() {
	id := uuid.New()
	expected := &domain.Household{ID: id, OwnerName: "Jane"}
	s.repo.On("FindByID", mock.Anything, id).Return(expected, nil)

	got, err := s.svc.GetByID(s.T().Context(), id)
	s.Require().NoError(err)
	s.Equal(expected, got)
}

func (s *HouseholdServiceSuite) TestGetByID_NotFound() {
	id := uuid.New()
	s.repo.On("FindByID", mock.Anything, id).Return(nil, domain.ErrNotFound)

	_, err := s.svc.GetByID(s.T().Context(), id)
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

func (s *HouseholdServiceSuite) TestList_ReturnsPage() {
	households := []*domain.Household{{ID: uuid.New()}, {ID: uuid.New()}}
	s.repo.On("List", mock.Anything, 1, 20).Return(households, 2, nil)

	got, total, err := s.svc.List(s.T().Context(), 1, 20)
	s.Require().NoError(err)
	s.Equal(2, total)
	s.Len(got, 2)
}

func (s *HouseholdServiceSuite) TestDelete_Success() {
	id := uuid.New()
	s.repo.On("Delete", mock.Anything, id).Return(nil)

	s.Require().NoError(s.svc.Delete(s.T().Context(), id))
}

func (s *HouseholdServiceSuite) TestDelete_NotFound() {
	id := uuid.New()
	s.repo.On("Delete", mock.Anything, id).Return(domain.ErrNotFound)

	err := s.svc.Delete(s.T().Context(), id)
	s.Require().ErrorIs(err, domain.ErrNotFound)
}
