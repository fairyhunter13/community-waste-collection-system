// Package service implements the business logic layer for all domain operations.
package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
)

type householdService struct {
	repo domain.HouseholdRepository
}

// NewHouseholdService creates a new HouseholdService backed by the given repository.
func NewHouseholdService(repo domain.HouseholdRepository) domain.HouseholdService {
	return &householdService{repo: repo}
}

func (s *householdService) Create(ctx context.Context, req domain.CreateHouseholdRequest) (*domain.Household, error) {
	h := &domain.Household{
		OwnerName: req.OwnerName,
		Address:   req.Address,
	}
	if err := s.repo.Create(ctx, h); err != nil {
		return nil, err
	}
	return h, nil
}

func (s *householdService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Household, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *householdService) List(ctx context.Context, page, perPage int) ([]*domain.Household, int, error) {
	return s.repo.List(ctx, page, perPage)
}

func (s *householdService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete household: %w", err)
	}
	return nil
}
