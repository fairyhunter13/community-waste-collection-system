// Package service implements the business logic layer for all domain operations.
package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

type householdService struct {
	repo domain.HouseholdRepository
}

// NewHouseholdService creates a new HouseholdService backed by the given repository.
func NewHouseholdService(repo domain.HouseholdRepository) domain.HouseholdService {
	return &householdService{repo: repo}
}

func (s *householdService) Create(ctx context.Context, req domain.CreateHouseholdRequest) (*domain.Household, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.household.Create")
	defer span.End()

	h := &domain.Household{
		OwnerName: req.OwnerName,
		Address:   req.Address,
	}
	if err := s.repo.Create(ctx, h); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.String("household.id", h.ID.String()))
	span.SetStatus(codes.Ok, "")
	return h, nil
}

func (s *householdService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Household, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.household.GetByID")
	span.SetAttributes(attribute.String("household.id", id.String()))
	defer span.End()

	h, err := s.repo.FindByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetStatus(codes.Ok, "")
	return h, nil
}

func (s *householdService) List(ctx context.Context, page, perPage int) ([]*domain.Household, int, error) {
	ctx, span := observability.Tracer().Start(ctx, "service.household.List")
	span.SetAttributes(attribute.Int("page", page), attribute.Int("per_page", perPage))
	defer span.End()

	households, total, err := s.repo.List(ctx, page, perPage)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, 0, err
	}
	span.SetAttributes(attribute.Int("result.count", total))
	span.SetStatus(codes.Ok, "")
	return households, total, nil
}

func (s *householdService) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, span := observability.Tracer().Start(ctx, "service.household.Delete")
	span.SetAttributes(attribute.String("household.id", id.String()))
	defer span.End()

	if err := s.repo.Delete(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("delete household: %w", err)
	}
	span.SetStatus(codes.Ok, "")
	return nil
}
