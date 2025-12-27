package schema

import (
	_ "embed"
	"strings"
)

//go:embed schema.graphql
var SchemaGraphQL string

// GetSchema returns the GraphQL schema found in `schema.graphql` as a string.
func GetSchema() string {
	return SchemaGraphQL
}

// IsBranchable returns true if the schema uses @branchable.
func IsBranchable() bool {
	return strings.Contains(SchemaGraphQL, "@branchable")
}
