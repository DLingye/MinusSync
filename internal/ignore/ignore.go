// Package ignore provides .msyncign file parsing and pattern matching.
// Uses gitignore-style pattern syntax.
package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Matcher holds a list of ignore patterns.
type Matcher struct {
	patterns []pattern
}

type pattern struct {
	raw     string
	dirOnly bool
	negate  bool
}

// Load reads a .msyncign file and returns a Matcher.
func Load(filePath string) (*Matcher, error) {
	m := &Matcher{}

	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		p := pattern{raw: line}

		// Check for negation
		if strings.HasPrefix(line, "!") {
			p.negate = true
			p.raw = line[1:]
		}

		// Check for directory-only marker
		if strings.HasSuffix(line, "/") {
			p.dirOnly = true
			p.raw = strings.TrimSuffix(p.raw, "/")
		}

		m.patterns = append(m.patterns, p)
	}

	return m, scanner.Err()
}

// IsIgnored reports whether a path should be ignored.
// isDir indicates if the path is a directory.
func (m *Matcher) IsIgnored(path string, isDir bool) bool {
	if m == nil || len(m.patterns) == 0 {
		return false
	}

	ignored := false

	for _, p := range m.patterns {
		// dirOnly patterns only match directories
		if p.dirOnly && !isDir {
			continue
		}

		if matchPattern(p.raw, path) {
			ignored = !p.negate
		}
	}

	return ignored
}

// matchPattern matches a gitignore-style pattern against a path.
func matchPattern(pattern, path string) bool {
	// Normalize path separators
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// If pattern doesn't contain a slash, match against the filename only
	if !strings.Contains(pattern, "/") {
		path = filepath.Base(path)
	} else {
		// Remove leading / if present
		pattern = strings.TrimPrefix(pattern, "/")
	}

	// Convert glob pattern to match function
	return globMatch(pattern, path)
}

// globMatch implements simple glob matching with ** support.
func globMatch(pattern, name string) bool {
	px := 0
	nx := 0
	nextPx := 0
	nextNx := 0

	for px < len(pattern) || nx < len(name) {
		if px < len(pattern) {
			c := pattern[px]
			switch c {
			case '?':
				if nx < len(name) {
					px++
					nx++
					continue
				}
			case '*':
				// Check for ** pattern (matches everything including /)
				if px+1 < len(pattern) && pattern[px+1] == '*' {
					// ** pattern
					if px+2 < len(pattern) && pattern[px+2] == '/' {
						px += 3 // skip **/
					} else {
						px += 2 // skip **
					}
					nextPx = px
					nextNx = nx + 1
					continue
				}

				// Single * doesn't match /
				nextPx = px
				nextNx = nx + 1
				px++
				continue
			default:
				if nx < len(name) && name[nx] == c {
					px++
					nx++
					continue
				}
			}
		}

		if 0 < nextNx && nextNx <= len(name) {
			px = nextPx
			nx = nextNx
			continue
		}

		return false
	}

	return true
}

// Add adds a pattern to the matcher.
func (m *Matcher) Add(pat string) {
	m.patterns = append(m.patterns, pattern{raw: pat})
}

// Remove removes a pattern from the matcher.
func (m *Matcher) Remove(pattern string) {
	for i, p := range m.patterns {
		if p.raw == pattern {
			m.patterns = append(m.patterns[:i], m.patterns[i+1:]...)
			return
		}
	}
}

// List returns all patterns in the matcher.
func (m *Matcher) List() []string {
	result := make([]string, len(m.patterns))
	for i, p := range m.patterns {
		result[i] = p.raw
	}
	return result
}

// Save writes the patterns to a file.
func (m *Matcher) Save(filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, p := range m.patterns {
		f.WriteString(p.raw + "\n")
	}
	return nil
}
