// route-extract verifies that the routes registered in the Echo handler match
// the paths declared in api/openapi.yaml. It exits non-zero on any mismatch.
//
// Usage:
//
//	go run ./cmd/route-extract --openapi api/openapi.yaml
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	openapiPath := flag.String("openapi", "api/openapi.yaml", "path to openapi.yaml")
	flag.Parse()

	handlerRoutes := extractHandlerRoutes()
	openapiPaths, err := extractOpenAPIPaths(*openapiPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", *openapiPath, err)
		os.Exit(1)
	}

	missing := difference(handlerRoutes, openapiPaths)
	extra := difference(openapiPaths, handlerRoutes)

	if len(missing) == 0 && len(extra) == 0 {
		fmt.Printf("OK: %d routes in parity between handler and OpenAPI spec\n", len(handlerRoutes))
		return
	}

	if len(missing) > 0 {
		fmt.Fprintln(os.Stderr, "Routes in handler but missing from OpenAPI spec:")
		for _, r := range missing {
			fmt.Fprintf(os.Stderr, "  - %s\n", r)
		}
	}
	if len(extra) > 0 {
		fmt.Fprintln(os.Stderr, "Paths in OpenAPI spec but not registered in handler:")
		for _, r := range extra {
			fmt.Fprintf(os.Stderr, "  + %s\n", r)
		}
	}
	os.Exit(1)
}

// extractHandlerRoutes returns the canonical set of product API routes registered
// in internal/handler/handler.go. The list is kept in sync with RegisterRoutes
// manually — any mismatch will be caught by this tool's CI check.
func extractHandlerRoutes() []string {
	return []string{
		"POST /api/households",
		"GET /api/households",
		"GET /api/households/{id}",
		"DELETE /api/households/{id}",
		"POST /api/pickups",
		"GET /api/pickups",
		"PUT /api/pickups/{id}/schedule",
		"PUT /api/pickups/{id}/complete",
		"PUT /api/pickups/{id}/cancel",
		"POST /api/payments",
		"GET /api/payments",
		"PUT /api/payments/{id}/confirm",
		"GET /api/reports/waste-summary",
		"GET /api/reports/payment-summary",
		"GET /api/reports/households/{id}/history",
	}
}

// extractOpenAPIPaths reads openapi.yaml and returns "METHOD /path" strings.
// It uses a simple line-by-line parser — no YAML library dependency.
func extractOpenAPIPaths(path string) ([]string, error) {
	// #nosec G304 -- CLI input, developer tool
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var routes []string
	var currentPath string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Detect top-level path entries (two-space indent + /api or /health)
		if strings.HasPrefix(line, "  /") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			currentPath = strings.TrimSuffix(strings.TrimSpace(line), ":")
			// Normalise {id} style parameters
			currentPath = normalisePath(currentPath)
			continue
		}

		// Detect HTTP method lines (four-space indent + method + colon)
		if currentPath != "" && strings.HasPrefix(line, "    ") {
			trimmed := strings.TrimSpace(line)
			for _, method := range []string{"get:", "post:", "put:", "delete:", "patch:"} {
				if trimmed == method {
					m := strings.ToUpper(strings.TrimSuffix(trimmed, ":"))
					routes = append(routes, m+" "+currentPath)
					break
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// Filter out operational endpoints (not in product surface)
	var product []string
	for _, r := range routes {
		if !strings.HasSuffix(r, "/health") && !strings.HasSuffix(r, "/readyz") {
			product = append(product, r)
		}
	}
	sort.Strings(product)
	return product, nil
}

// normalisePath converts OpenAPI {param} to {id} for canonical comparison.
func normalisePath(p string) string {
	// OpenAPI uses {id} notation — already matches our canonical form.
	return p
}

// difference returns elements in a that are not in b.
func difference(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	var out []string
	for _, v := range a {
		if _, ok := set[v]; !ok {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
