// Package diff provides line-level diff computation using the Myers algorithm.
package diff

import (
	"fmt"
	"strings"
)

// Op represents a diff operation.
type Op int

const (
	OpEqual  Op = 0
	OpInsert Op = 1
	OpDelete Op = 2
)

// Edit represents one edit operation in a diff.
type Edit struct {
	Op   Op
	OldStart, OldEnd int // Range in old file (line numbers, 0-indexed)
	NewStart, NewEnd int // Range in new file (line numbers, 0-indexed)
}

// Hunk is a group of edits with context lines.
type Hunk struct {
	OldStart, OldCount int
	NewStart, NewCount int
	Lines              []HunkLine
}

// HunkLine is one line in a hunk.
type HunkLine struct {
	Op      Op
	Content string
}

// Diff computes the differences between two texts, returning edits.
func Diff(oldText, newText string) []Edit {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	// Remove trailing empty string from split
	if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}

	edits := myersDiff(oldLines, newLines)
	return compactEdits(edits)
}

// UnifiedDiff produces a unified diff string.
func UnifiedDiff(oldText, newText, oldLabel, newLabel string, contextLines int) string {
	edits := Diff(oldText, newText)
	if len(edits) == 0 {
		return ""
	}

	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")
	if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}

	hunks := buildHunks(edits, oldLines, newLines, contextLines)

	var buf strings.Builder
	fmt.Fprintf(&buf, "--- %s\n", oldLabel)
	fmt.Fprintf(&buf, "+++ %s\n", newLabel)

	for _, h := range hunks {
		fmt.Fprintf(&buf, "@@ -%d,%d +%d,%d @@\n",
			h.OldStart+1, h.OldCount,
			h.NewStart+1, h.NewCount)

		for _, line := range h.Lines {
			switch line.Op {
			case OpEqual:
				fmt.Fprintf(&buf, " %s\n", line.Content)
			case OpInsert:
				fmt.Fprintf(&buf, "+%s\n", line.Content)
			case OpDelete:
				fmt.Fprintf(&buf, "-%s\n", line.Content)
			}
		}
	}

	return buf.String()
}

// myersDiff implements the Myers O(ND) diff algorithm.
func myersDiff(a, b []string) []Edit {
	n := len(a)
	m := len(b)
	max := n + m

	// v[k] stores the furthest-reaching path for diagonal k
	v := make([]int, 2*max+1)
	trace := make([][]int, 0)

	// Fast path: if either is empty
	if n == 0 {
		return []Edit{{Op: OpInsert, OldStart: 0, OldEnd: 0, NewStart: 0, NewEnd: m}}
	}
	if m == 0 {
		return []Edit{{Op: OpDelete, OldStart: 0, OldEnd: n, NewStart: 0, NewEnd: 0}}
	}

	for d := 0; d <= max; d++ {
		vc := make([]int, 2*max+1)
		copy(vc, v)
		trace = append(trace, vc)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[idx(k-1, max)] < v[idx(k+1, max)]) {
				x = v[idx(k+1, max)]
			} else {
				x = v[idx(k-1, max)] + 1
			}

			y := x - k

			// Follow the diagonal while elements match
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}

			v[idx(k, max)] = x

			if x >= n && y >= m {
				// Found the solution — backtrack
				return backtrack(trace, a, b, d, max)
			}
		}
	}

	return nil
}

func idx(k, max int) int {
	return k + max
}

// backtrack reconstructs the edit script from the trace.
func backtrack(trace [][]int, a, b []string, d, max int) []Edit {
	var edits []Edit

	x := len(a)
	y := len(b)

	for d > 0 {
		v := trace[d]
		k := x - y

		var prevK int
		if k == -d || (k != d && v[idx(k-1, max)] < v[idx(k+1, max)]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}

		prevX := v[idx(prevK, max)]
		prevY := prevX - prevK

		// Follow diagonal
		for x > prevX && y > prevY {
			x--
			y--
		}

		if x == prevX {
			// Insert
			edits = append(edits, Edit{
				Op: OpInsert, OldStart: x, OldEnd: x,
				NewStart: prevY, NewEnd: y,
			})
		} else {
			// Delete
			edits = append(edits, Edit{
				Op: OpDelete, OldStart: prevX, OldEnd: x,
				NewStart: y, NewEnd: y,
			})
		}

		x = prevX
		y = prevY
		d--
	}

	// Reverse edits (they were built backwards)
	for i, j := 0, len(edits)-1; i < j; i, j = i+1, j-1 {
		edits[i], edits[j] = edits[j], edits[i]
	}

	return edits
}

// compactEdits merges adjacent edits of the same type.
func compactEdits(edits []Edit) []Edit {
	if len(edits) <= 1 {
		return edits
	}

	var result []Edit
	current := edits[0]

	for i := 1; i < len(edits); i++ {
		e := edits[i]
		if e.Op == current.Op {
			current.OldEnd = e.OldEnd
			current.NewEnd = e.NewEnd
		} else {
			result = append(result, current)
			current = e
		}
	}
	result = append(result, current)
	return result
}

// buildHunks groups edits into hunks with context.
func buildHunks(edits []Edit, oldLines, newLines []string, context int) []Hunk {
	if len(edits) == 0 {
		return nil
	}

	var hunks []Hunk
	var current *Hunk

	for _, e := range edits {
		// Add context before this edit
		contextStart := e.OldStart
		if current != nil {
			contextStart = max(contextStart, current.OldStart+current.OldCount)
		}

		if current == nil || contextStart-current.OldStart > 2*context {
			if current != nil {
				hunks = append(hunks, *current)
			}
			h := Hunk{
				OldStart: max(0, e.OldStart-context),
				NewStart: max(0, e.NewStart-context),
			}
			current = &h
		}

		// Add equal context before
		preContextStart := current.OldStart + current.OldCount
		for i := preContextStart; i < e.OldStart; i++ {
			if i < len(oldLines) {
				lineIdx := current.NewStart + current.NewCount + (i - preContextStart)
				current.Lines = append(current.Lines, HunkLine{Op: OpEqual, Content: oldLines[i]})
				current.OldCount++
				if lineIdx < len(newLines) {
					current.NewCount++
				}
			}
		}

		// Add the edit itself
		for i := e.OldStart; i < e.OldEnd; i++ {
			if i < len(oldLines) {
				current.Lines = append(current.Lines, HunkLine{Op: OpDelete, Content: oldLines[i]})
				current.OldCount++
			}
		}
		for i := e.NewStart; i < e.NewEnd; i++ {
			if i < len(newLines) {
				current.Lines = append(current.Lines, HunkLine{Op: OpInsert, Content: newLines[i]})
				current.NewCount++
			}
		}
	}

	if current != nil {
		// Add trailing context
		endOld := current.OldStart + current.OldCount
		for i := 0; i < context && endOld+i < len(oldLines); i++ {
			current.Lines = append(current.Lines, HunkLine{Op: OpEqual, Content: oldLines[endOld+i]})
			current.OldCount++
			current.NewCount++
		}
		hunks = append(hunks, *current)
	}

	return hunks
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
