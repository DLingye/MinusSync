package search

import "strings"

// ParseQuery parses a raw query string into a Query.
func ParseQuery(raw string, caseSensitive bool, maxResults int) Query {
	// Split on whitespace for simple term queries
	terms := strings.Fields(raw)
	if len(terms) == 0 {
		return Query{MaxResults: maxResults}
	}

	// Filter out very short terms
	var filtered []string
	for _, t := range terms {
		if len(t) >= 1 {
			filtered = append(filtered, t)
		}
	}

	return Query{
		Terms:         filtered,
		CaseSensitive: caseSensitive,
		MaxResults:    maxResults,
	}
}
