package schema

import (
	_ "embed"
)

//go:embed schema.graphql
var SchemaGraphQL string

// GetSchema returns the GraphQL schema as a string.
// This is the source of truth for the schema used by both indexer and host.
// Note: The actual schema file is maintained in the root schema/ directory,
// and this file is a copy for embedding purposes.
func GetSchema() string {
	return SchemaGraphQL
}

