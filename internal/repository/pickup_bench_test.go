//go:build integration

package repository_test

import (
	"os"
	"testing"

	"github.com/fairyhunter13/community-waste-collection-system/internal/domain"
	"github.com/fairyhunter13/community-waste-collection-system/internal/repository"
)

func BenchmarkPickupRepository_List(b *testing.B) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		b.Skip("DATABASE_URL not set")
	}
	db := repository.MustConnect(url)
	defer db.Close()

	repo := repository.NewPickupRepository(db)
	filter := domain.PickupFilter{Page: 1, PerPage: 20}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = repo.List(b.Context(), filter)
	}
}
