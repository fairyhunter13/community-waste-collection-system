package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/mocks"
	"github.com/fairyhunter13/community-waste-collection-system/internal/service"
)

func BenchmarkPickupService_Create(b *testing.B) {
	pickupRepo := mocks.NewPickupRepository(b)
	paymentRepo := mocks.NewPaymentRepository(b)

	pickup := &domain.WastePickup{ID: uuid.New(), Type: domain.WasteTypeOrganic, Status: domain.PickupStatusPending}
	pickupRepo.On("HasPendingPaymentForHousehold", mock.Anything, mock.Anything).Return(false, nil)
	pickupRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	_ = paymentRepo

	svc := service.NewPickupService(pickupRepo, paymentRepo, nil)
	req := domain.CreatePickupRequest{HouseholdID: uuid.New(), Type: domain.WasteTypeOrganic}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := svc.Create(context.Background(), req)
		if err != nil {
			b.Fatal(err)
		}
		_ = p
		_ = pickup
	}
}
