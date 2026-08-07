package util

import (
	"strings"
	"unicode"
)

// Indent adds prefix to each line of s.
func Indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if i < len(lines)-1 || line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// TruncateString truncates s to maxLen and appends "..." if truncated.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// IsBinary detects whether data appears to be binary (contains null bytes).
func IsBinary(data []byte) bool {
	// Check first 8KB at most
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// IsTextFile checks if a filename likely contains text content.
func IsTextFile(name string) bool {
	ext := strings.ToLower(name)
	textExts := []string{
		".go", ".rs", ".c", ".cpp", ".h", ".hpp", ".py", ".js", ".ts",
		".java", ".kt", ".scala", ".rb", ".php", ".cs", ".swift",
		".md", ".txt", ".rst", ".adoc", ".org",
		".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf",
		".xml", ".html", ".css", ".svg",
		".sh", ".bash", ".zsh", ".fish", ".ps1",
		".vim", ".el", ".lua",
		".sql", ".proto", ".graphql",
		".gitignore", ".msyncign", ".dockerignore",
		"Dockerfile", "Makefile", "CMakeLists.txt",
	}
	for _, e := range textExts {
		if strings.HasSuffix(ext, e) || strings.EqualFold(ext, e) {
			return true
		}
	}
	return false
}

// CamelCaseSplit splits a camelCase or PascalCase word into its components.
func CamelCaseSplit(s string) []string {
	var words []string
	var current strings.Builder

	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			// Check if the previous char was lowercase (camelCase boundary)
			prev := rune(s[i-1])
			if unicode.IsLower(prev) {
				words = append(words, strings.ToLower(current.String()))
				current.Reset()
			} else if i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
				// PascalCase boundary like "HTTPServer" → "HTTP" + "Server"
				words = append(words, strings.ToLower(current.String()))
				current.Reset()
			}
		}
		if r == '_' || r == '-' {
			words = append(words, strings.ToLower(current.String()))
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		words = append(words, strings.ToLower(current.String()))
	}

	var result []string
	for _, w := range words {
		if w != "" {
			result = append(result, w)
		}
	}
	return result
}
