//go:build integration

package repository_test

import (
	"os"
	"testing"

	"github.com/fairyhunter13/community-waste-collection-system/internal/repository"
)

func BenchmarkPaymentRepository_WasteSummary(b *testing.B) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		b.Skip("DATABASE_URL not set")
	}
	db := repository.MustConnect(url)
	defer db.Close()

	repo := repository.NewPaymentRepository(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repo.WasteSummary(b.Context())
	}
}
