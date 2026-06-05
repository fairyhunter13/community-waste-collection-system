//go:build integration

package repository_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/repository"
)

type HouseholdRepoSuite struct {
	baseRepoSuite
	repo domain.HouseholdRepository
}

func (s *HouseholdRepoSuite) SetupSuite() {
	s.baseRepoSuite.SetupSuite()
	s.repo = repository.NewHouseholdRepository(s.db)
}

func TestHouseholdRepository(t *testing.T) {
	suite.Run(t, new(HouseholdRepoSuite))
}

func (s *HouseholdRepoSuite) TestCreate_SetsIDAndTimestamps() {
	h := &domain.Household{OwnerName: "John Doe", Address: "Jl. Merdeka 1"}
	s.Require().NoError(s.repo.Create(s.T().Context(), h))

	s.Require().NotEqual(uuid.Nil, h.ID)
	s.Require().False(h.CreatedAt.IsZero())
	s.Require().False(h.UpdatedAt.IsZero())
	s.Equal("John Doe", h.OwnerName)
}

func (s *HouseholdRepoSuite) TestFindByID_Found() {
	h := &domain.Household{OwnerName: "Jane", Address: "Jl. Sudirman 5"}
	s.Require().NoError(s.repo.Create(s.T().Context(), h))

	got, err := s.repo.FindByID(s.T().Context(), h.ID)
	s.Require().NoError(err)
	s.Equal(h.ID, got.ID)
	s.Equal(h.OwnerName, got.OwnerName)
	s.Equal(h.Address, got.Address)
}

func (s *HouseholdRepoSuite) TestFindByID_NotFound() {
	_, err := s.repo.FindByID(s.T().Context(), uuid.New())
	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

func (s *HouseholdRepoSuite) TestList_Empty() {
	households, total, err := s.repo.List(s.T().Context(), 1, 20)
	s.Require().NoError(err)
	s.Equal(0, len(households))
	s.Equal(0, total)
}

func (s *HouseholdRepoSuite) TestList_Paginated() {
	for i := range 5 {
		h := &domain.Household{
			OwnerName: "Owner " + string(rune('A'+i)),
			Address:   "Address " + string(rune('A'+i)),
		}
		s.Require().NoError(s.repo.Create(s.T().Context(), h))
	}

	// Page 1: 3 per page
	page1, total, err := s.repo.List(s.T().Context(), 1, 3)
	s.Require().NoError(err)
	s.Equal(3, len(page1))
	s.Equal(5, total)

	// Page 2: remaining 2
	page2, total2, err := s.repo.List(s.T().Context(), 2, 3)
	s.Require().NoError(err)
	s.Equal(2, len(page2))
	s.Equal(5, total2)
}

func (s *HouseholdRepoSuite) TestDelete_Found() {
	h := &domain.Household{OwnerName: "To Delete", Address: "Somewhere"}
	s.Require().NoError(s.repo.Create(s.T().Context(), h))

	s.Require().NoError(s.repo.Delete(s.T().Context(), h.ID))

	_, err := s.repo.FindByID(s.T().Context(), h.ID)
	s.Require().ErrorIs(err, domain.ErrNotFound)
}

func (s *HouseholdRepoSuite) TestDelete_NotFound() {
	err := s.repo.Delete(s.T().Context(), uuid.New())
	s.Require().ErrorIs(err, domain.ErrNotFound)
}
