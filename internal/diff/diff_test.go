package diff

import (
	"strings"
	"testing"
)

func TestDiffSame(t *testing.T) {
	edits := Diff("hello\nworld\n", "hello\nworld\n")
	if len(edits) != 0 {
		t.Errorf("expected no edits for identical texts, got %d", len(edits))
	}
}

func TestDiffInsert(t *testing.T) {
	edits := Diff("line1\n", "line1\nline2\n")
	if len(edits) != 1 || edits[0].Op != OpInsert {
		t.Errorf("expected one insert edit, got %+v", edits)
	}
}

func TestDiffDelete(t *testing.T) {
	edits := Diff("line1\nline2\n", "line1\n")
	if len(edits) != 1 || edits[0].Op != OpDelete {
		t.Errorf("expected one delete edit, got %+v", edits)
	}
}

func TestDiffChange(t *testing.T) {
	edits := Diff("old line\n", "new line\n")
	// Should have delete + insert
	if len(edits) == 0 {
		t.Error("expected edits for changed line")
	}
}

func TestUnifiedDiff(t *testing.T) {
	result := UnifiedDiff("old\n", "new\n", "a.txt", "b.txt", 3)
	if !strings.Contains(result, "--- a.txt") {
		t.Error("unified diff missing old label")
	}
	if !strings.Contains(result, "+++ b.txt") {
		t.Error("unified diff missing new label")
	}
	if !strings.Contains(result, "@@") {
		t.Error("unified diff missing hunk header")
	}
}

func TestSimilarity(t *testing.T) {
	sim := Similarity("hello world\nfoo bar\n", "hello world\nfoo baz\n")
	if sim < 0.4 || sim > 0.9 {
		t.Errorf("expected similarity around 0.5-0.8, got %f", sim)
	}

	simSame := Similarity("abc", "abc")
	if simSame != 1.0 {
		t.Errorf("identical texts should have similarity 1.0, got %f", simSame)
	}

	simDiff := Similarity("abc", "xyz")
	if simDiff >= 0.5 {
		t.Errorf("completely different texts should have low similarity, got %f", simDiff)
	}
}
