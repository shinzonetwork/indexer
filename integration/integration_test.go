package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"shinzo/version1/pkg/indexer"
	"strings"
	"testing"
	"time"
)

const graphqlURL = "http://localhost:9181/api/v0/graphql"

func TestMain(m *testing.M) {
	fmt.Println("TestMain - Starting indexer in background")

	// Start indexer in a goroutine
	go func() {
		err := indexer.StartIndexing("./.defra", "http://localhost:9181")
		if err != nil {
			panic(fmt.Sprintf("Encountered unexpected error starting defra dependency: %v", err))
		}
	}()

	// Wait for indexer to be ready
	fmt.Println("Waiting for indexer to start...")
	for !indexer.IsStarted || !indexer.HasIndexedAtLeastOneBlock {
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("Indexer is ready!")

	// Run tests
	exitCode := m.Run()

	// Teardown
	fmt.Println("TestMain - Teardown")
	indexer.StopIndexing()

	os.Exit(exitCode)
}

func TestGraphQLConnection(t *testing.T) {
	t.Log("Testing graphql connection\n")
	resp, err := http.Post(graphqlURL, "application/json", bytes.NewBuffer([]byte(`{"query":"query { __typename }"}`)))
	if err != nil {
		t.Fatalf("Failed to connect to GraphQL endpoint: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Unexpected status code: %d", resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if _, ok := result["data"]; !ok {
		t.Fatalf("No data field in response: %s", string(body))
	}
}

func postGraphQLQuery(t *testing.T, query string, variables map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(graphqlURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		t.Fatalf("Failed to POST query: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Unexpected status code: %d", resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	return result
}

// Helper to find the project root by looking for go.mod
func getProjectRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("Could not find project root (go.mod)")
		}
		dir = parent
	}
}

// Helper to extract a named query from a .graphql file
func loadGraphQLQuery(filename, queryName string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	content := string(data)
	start := strings.Index(content, "query "+queryName)
	if start == -1 {
		return "", fmt.Errorf("query %s not found", queryName)
	}
	// Find the next "query " after start, or end of file
	next := strings.Index(content[start+1:], "query ")
	var query string
	if next == -1 {
		query = content[start:]
	} else {
		query = content[start : start+next+1]
	}
	query = strings.TrimSpace(query)
	return query, nil
}

func MakeQuery(t *testing.T, queryPath string, query string, args map[string]interface{}) map[string]interface{} {
	query, err := loadGraphQLQuery(queryPath, query)
	if err != nil {
		t.Errorf("Failed to load query %v", err)
	}
	result := postGraphQLQuery(t, query, args)
	return result
}
