// Package search provides semantic search over tracked files.
package search

import (
	"strings"
	"unicode"

	"github.com/MinusSync/internal/util"
)

// Tokenizer splits text into searchable tokens.
type Tokenizer struct{}

// NewTokenizer creates a new tokenizer.
func NewTokenizer() *Tokenizer {
	return &Tokenizer{}
}

// Tokenize splits text into tokens for indexing.
// Returns lowercase tokens with code-aware splitting.
func (t *Tokenizer) Tokenize(text string, filePath string) []string {
	// Determine the tokenization strategy based on file extension
	isCode := util.IsTextFile(filePath) && !strings.HasSuffix(filePath, ".md")

	if isCode {
		return t.tokenizeCode(text)
	}
	return t.tokenizeText(text)
}

// tokenizeCode splits code text at word boundaries, camelCase, snake_case, operators.
func (t *Tokenizer) tokenizeCode(text string) []string {
	var tokens []string
	seen := make(map[string]bool)

	// Split on whitespace first
	words := strings.Fields(text)
	for _, word := range words {
		// Further split on camelCase and snake_case
		subTokens := util.CamelCaseSplit(word)
		for _, tok := range subTokens {
			// Skip very short tokens and pure numbers
			if len(tok) < 2 || isAllDigits(tok) {
				continue
			}
			tok = strings.ToLower(tok)
			if !seen[tok] {
				seen[tok] = true
				tokens = append(tokens, tok)
			}
		}
	}

	return tokens
}

// tokenizeText splits general text at word boundaries.
func (t *Tokenizer) tokenizeText(text string) []string {
	var tokens []string
	seen := make(map[string]bool)

	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-'
	})

	for _, word := range words {
		if len(word) < 2 || isAllDigits(word) {
			continue
		}
		if !seen[word] {
			seen[word] = true
			tokens = append(tokens, word)
		}
	}

	return tokens
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return len(s) > 0
}
