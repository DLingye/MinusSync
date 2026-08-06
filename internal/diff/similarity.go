package diff

// Similarity computes a similarity ratio (0.0 to 1.0) between two texts.
// Uses a simple ratio of matching lines to total lines.
func Similarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	edits := Diff(a, b)

	// Count changed lines
	var changed int
	for _, e := range edits {
		if e.Op == OpInsert {
			changed += e.NewEnd - e.NewStart
		} else if e.Op == OpDelete {
			changed += e.OldEnd - e.OldStart
		}
	}

	aLines := countLines(a)
	bLines := countLines(b)
	total := aLines + bLines
	if total == 0 {
		return 1.0
	}

	return 1.0 - float64(changed)/float64(total)
}

// IsRename returns true if two files are likely a rename of each other.
// Threshold is typically 0.6 (60% similar).
func IsRename(a, b string, threshold float64) bool {
	return Similarity(a, b) >= threshold
}

func countLines(s string) int {
	if len(s) == 0 {
		return 0
	}
	n := 1
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	return n
}
