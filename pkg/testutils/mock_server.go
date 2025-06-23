package testutils

import (
	"net/http"
	"net/http/httptest"
)

// MockServerConfig holds configuration for creating mock servers
type MockServerConfig struct {
	ResponseBody    string
	StatusCode      int
	Headers         map[string]string
	ValidateRequest func(r *http.Request) error
}

// DefaultMockServerConfig returns a default configuration
func DefaultMockServerConfig(responseBody string) MockServerConfig {
	return MockServerConfig{
		ResponseBody: responseBody,
		StatusCode:   http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}
}

// CreateMockServer creates a mock HTTP server with the given configuration
func CreateMockServer(config MockServerConfig) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if config.ValidateRequest != nil {
			if err := config.ValidateRequest(r); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		for key, value := range config.Headers {
			w.Header().Set(key, value)
		}

		w.WriteHeader(config.StatusCode)

		w.Write([]byte(config.ResponseBody))
	}))
}

// CreateGraphQLMockServer creates a mock server that returns a GraphQL response
func CreateGraphQLMockServer(responseBody string) *httptest.Server {
	return CreateMockServer(DefaultMockServerConfig(responseBody))
}

// CreateGraphQLCreateResponse creates a standard GraphQL create response
func CreateGraphQLCreateResponse(collectionName, docID string) string {
	return `{
		"data": {
			"create_` + collectionName + `": [
				{
					"_docID": "` + docID + `"
				}
			]
		}
	}`
}

// CreateGraphQLUpdateResponse creates a standard GraphQL update response
func CreateGraphQLUpdateResponse(collectionName, docID string) string {
	return `{
		"data": {
			"update_` + collectionName + `": [
				{
					"_docID": "` + docID + `"
				}
			]
		}
	}`
}

// CreateGraphQLQueryResponse creates a standard GraphQL query response
func CreateGraphQLQueryResponse(collectionName, responseData string) string {
	return `{
		"data": {
			"` + collectionName + `": ` + responseData + `
		}
	}`
}

// CreateErrorServer creates a mock server that returns an error
func CreateErrorServer(statusCode int, errorMessage string) *httptest.Server {
	return CreateMockServer(MockServerConfig{
		ResponseBody: errorMessage,
		StatusCode:   statusCode,
		Headers:      map[string]string{},
	})
}
