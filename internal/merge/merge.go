// Package merge provides three-way merge logic.
package merge

import (
	"fmt"
	"strings"

	"github.com/MinusSync/internal/diff"
)

// MergeResult holds the result of a three-way merge.
type MergeResult struct {
	Merged   string
	HasConflicts bool
	Conflicts    []Conflict
}

// Conflict describes a merge conflict.
type Conflict struct {
	StartLine int // 0-indexed line in merged output
	OursText  string
	BaseText  string
	TheirsText string
}

// ThreeWay performs a three-way merge of two texts based on a common base.
func ThreeWay(base, ours, theirs string) (*MergeResult, error) {
	// Get diffs from base to each side
	oursEdits := diff.Diff(base, ours)
	theirsEdits := diff.Diff(base, theirs)

	// Check for conflicts
	conflicts := findConflicts(oursEdits, theirsEdits)

	if len(conflicts) > 0 {
		merged := applyMergeWithConflicts(base, oursEdits, theirsEdits, conflicts)
		return &MergeResult{
			Merged:       merged,
			HasConflicts: true,
			Conflicts:    conflicts,
		}, nil
	}

	// No conflicts — apply theirs edits on top of ours
	merged := applyEdits(ours, theirsEdits)
	return &MergeResult{
		Merged:       merged,
		HasConflicts: false,
	}, nil
}

// findConflicts checks if two edit sets overlap.
func findConflicts(ours, theirs []diff.Edit) []Conflict {
	var conflicts []Conflict

	// Simple overlap detection: check if edits touch the same lines
	// in the base document
	for _, o := range ours {
		for _, t := range theirs {
			if rangesOverlap(o.OldStart, o.OldEnd, t.OldStart, t.OldEnd) {
				conflicts = append(conflicts, Conflict{
					StartLine: max(o.OldStart, t.OldStart),
				})
			}
		}
	}

	return conflicts
}

func rangesOverlap(a1, a2, b1, b2 int) bool {
	return a1 < b2 && b1 < a2
}

// applyMergeWithConflicts applies edits and inserts conflict markers.
func applyMergeWithConflicts(base string, ours, theirs []diff.Edit, conflicts []Conflict) string {
	baseLines := strings.Split(base, "\n")
	if len(baseLines) > 0 && baseLines[len(baseLines)-1] == "" {
		baseLines = baseLines[:len(baseLines)-1]
	}

	var result []string
	pos := 0
	inConflict := false
	conflictSet := make(map[int]bool)
	for _, c := range conflicts {
		conflictSet[c.StartLine] = true
	}

	// Merge both edit lists into a timeline
	allEdits := mergeEditLists(ours, theirs)

	for _, e := range allEdits {
		// Output base lines before this edit
		for pos < e.OldStart && pos < len(baseLines) {
			result = append(result, baseLines[pos])
			pos++
		}

		if _, isConflict := conflictSet[e.OldStart]; isConflict && !inConflict {
			inConflict = true
			result = append(result, "<<<<<<< ours")
		}

		if inConflict && !rangesOverlapWithSet(e, conflictSet) {
			result = append(result, "=======")
			// Would need to add theirs side here
			result = append(result, ">>>>>>> theirs")
			inConflict = false
		}

		// Skip the edit region for now (simplified)
		pos = e.OldEnd
	}

	// Output remaining base lines
	for pos < len(baseLines) {
		result = append(result, baseLines[pos])
		pos++
	}

	return strings.Join(result, "\n")
}

func rangesOverlapWithSet(e diff.Edit, conflictSet map[int]bool) bool {
	for i := e.OldStart; i < e.OldEnd; i++ {
		if conflictSet[i] {
			return true
		}
	}
	return false
}

// mergeEditLists combines two edit lists chronologically.
func mergeEditLists(ours, theirs []diff.Edit) []diff.Edit {
	var result []diff.Edit
	oi, ti := 0, 0

	for oi < len(ours) || ti < len(theirs) {
		if oi >= len(ours) {
			result = append(result, theirs[ti])
			ti++
		} else if ti >= len(theirs) {
			result = append(result, ours[oi])
			oi++
		} else if ours[oi].OldStart <= theirs[ti].OldStart {
			result = append(result, ours[oi])
			oi++
		} else {
			result = append(result, theirs[ti])
			ti++
		}
	}

	return result
}

// applyEdits applies a set of edits to a text.
func applyEdits(text string, edits []diff.Edit) string {
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var result []string
	pos := 0

	for _, e := range edits {
		// Copy lines before the edit
		for pos < e.NewStart && pos < len(lines) {
			result = append(result, lines[pos])
			pos++
		}

		switch e.Op {
		case diff.OpDelete:
			pos = e.NewEnd
		case diff.OpInsert:
			// We need the inserted content; for now skip
			pos = e.NewEnd
		case diff.OpEqual:
			for pos < e.NewEnd && pos < len(lines) {
				result = append(result, lines[pos])
				pos++
			}
		}
	}

	// Copy remaining lines
	for pos < len(lines) {
		result = append(result, lines[pos])
		pos++
	}

	return strings.Join(result, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Ensure fmt is used
var _ = fmt.Sprintf
