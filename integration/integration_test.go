//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
)

const graphqlURL = "http://localhost:9181/api/v0/graphql"

func TestGraphQLConnection(t *testing.T) {
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
