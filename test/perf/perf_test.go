//go:build perf

package perf_test

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

func benchBaseURL(b *testing.B) string {
	url := os.Getenv("BASE_URL")
	if url == "" {
		b.Skip("BASE_URL not set — skipping performance tests")
	}
	return url
}

func BenchmarkListHouseholds(b *testing.B) {
	base := benchBaseURL(b)
	client := &http.Client{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(base + "/api/households")
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkCreateHousehold(b *testing.B) {
	base := benchBaseURL(b)
	client := &http.Client{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := fmt.Sprintf(`{"owner_name":"bench-%d","address":"Jl. Test %d"}`, i, i)
		resp, err := client.Post(base+"/api/households", "application/json", strings.NewReader(body))
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkListPickups(b *testing.B) {
	base := benchBaseURL(b)
	client := &http.Client{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(base + "/api/pickups")
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkWasteSummary(b *testing.B) {
	base := benchBaseURL(b)
	client := &http.Client{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(base + "/api/reports/waste-summary")
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

func BenchmarkPaymentSummary(b *testing.B) {
	base := benchBaseURL(b)
	client := &http.Client{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(base + "/api/reports/payment-summary")
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
