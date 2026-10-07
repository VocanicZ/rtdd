package main

// plural picks the noun form for n.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
