package money

import "strings"

func Normalize(s string) string {
	return strings.TrimSpace(s)
}

func Parse(s string) string {
	return Normalize(strings.ToLower(s))
}
