package cogo

import (
	"fmt"
	"strings"
)

func tokenize(command string) ([]string, error) {
	var fields []string
	var b strings.Builder
	inQuote := false
	escaped := false
	tokenStarted := false

	for _, r := range command {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
			tokenStarted = true
		case r == '\\':
			escaped = true
			tokenStarted = true
		case r == '"':
			inQuote = !inQuote
			tokenStarted = true
		case r == ' ' || r == '\t' || r == '\n':
			if inQuote {
				b.WriteRune(r)
				tokenStarted = true
				continue
			}
			if tokenStarted {
				fields = append(fields, b.String())
				b.Reset()
				tokenStarted = false
			}
		default:
			b.WriteRune(r)
			tokenStarted = true
		}
	}
	if escaped {
		return nil, fmt.Errorf("unfinished escape")
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quote")
	}
	if tokenStarted {
		fields = append(fields, b.String())
	}
	return fields, nil
}

func Fields(command string) ([]string, error) {
	return tokenize(command)
}
