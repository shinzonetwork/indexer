package schema

import (
	_ "embed"
)

//go:embed schema.graphql
var SchemaGraphQL string

//go:embed schema_branchable.graphql
var SchemaBranchGraphQL string

// GetSchema returns the GraphQL schema found in `schema.graphql` as a string.
func GetSchema() string {
	return SchemaGraphQL
}

// GetBranchableSchema returns the branchable GraphQL schema.
func GetBranchableSchema() string {
	return SchemaBranchGraphQL
}

// GetSchemaForBuild returns the appropriate schema based on build tags.
func GetSchemaForBuild() string {
	if IsBranchable() {
		return GetBranchableSchema()
	}
	return GetSchema()
}
