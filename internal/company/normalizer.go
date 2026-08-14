package company

import "strings"

func NormalizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))

	var b strings.Builder
	b.Grow(len(name))

	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '.':
			b.WriteRune(' ')
		}
	}

	return strings.Join(strings.Fields(b.String()), " ")
}
