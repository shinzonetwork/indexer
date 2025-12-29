//go:build branchable
// +build branchable

package schema

// IsBranchable returns true if the binary was built with the branchable tag.
func IsBranchable() bool {
	return true
}
