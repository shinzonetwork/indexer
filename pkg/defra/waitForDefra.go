package defra

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// WaitForDefraDB waits for a DefraDB instance to be ready by checking the GraphQL endpoint
// It will retry until the endpoint responds with a valid schema or until max attempts are reached
func WaitForDefraDB(url string) error {
	fmt.Println("Waiting for defra...")
	maxAttempts := 15  // Reduced to prevent excessive waiting in tests

	// Construct the GraphQL endpoint URL
	graphqlURL := strings.TrimSuffix(url, "/") + "/api/v0/graphql"

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Simple GraphQL introspection query to check if DefraDB is ready
	// This doesn't require any specific schema to be applied
	query := `{"query":"{ __schema { types { name } } }"}`

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Create request
		req, err := http.NewRequestWithContext(
			context.Background(),
			"POST",
			graphqlURL,
			strings.NewReader(query),
		)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		// Make request
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// Check if response is successful
		if resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			fmt.Println("Defra is responsive!")
			return nil
		}
		fmt.Printf("Attempt %d failed... Trying again\n", attempt)

		resp.Body.Close()
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("DefraDB failed to become ready after %d retry attempts", maxAttempts)
}
