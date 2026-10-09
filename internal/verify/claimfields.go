package verify

// ClaimFields returns the Branch, Base commit and Worktree values of the Claim
// block in the bytes of an active task file. It uses the same section and
// metadata parser that Verify applies, and it never writes. A key that is
// missing from the block has no entry in the returned map.
func ClaimFields(data []byte) map[string]string {
	return metadata(parseSections(string(data))["Claim"])
}
