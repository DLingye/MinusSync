package object

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MinusSync/internal/hash"
)

func TestFormat(t *testing.T) {
	data := Format(TypeBlob, []byte("hello"))
	expected := "blob 5\x00hello"
	if string(data) != expected {
		t.Errorf("expected %q, got %q", expected, string(data))
	}
}

func TestParseHeader(t *testing.T) {
	data := Format(TypeBlob, []byte("hello"))
	header, offset, err := ParseHeader(data)
	if err != nil {
		t.Fatal(err)
	}
	if header.Type != TypeBlob {
		t.Errorf("expected blob type, got %s", header.Type)
	}
	if header.Size != 5 {
		t.Errorf("expected size 5, got %d", header.Size)
	}
	if offset != 7 { // "blob 5\0" = 7 bytes
		t.Errorf("expected offset 7, got %d", offset)
	}
}

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()

	data := Format(TypeBlob, []byte("test content"))
	h, err := Write(dir, data)
	if err != nil {
		t.Fatal(err)
	}

	if !Exists(dir, h) {
		t.Error("object should exist after Write")
	}

	readData, err := Read(dir, h)
	if err != nil {
		t.Fatal(err)
	}

	if string(readData) != string(data) {
		t.Error("read data doesn't match written data")
	}
}

func TestWriteBlob(t *testing.T) {
	dir := t.TempDir()

	h, err := WriteBlob(dir, []byte("test blob content"))
	if err != nil {
		t.Fatal(err)
	}

	content, err := ReadBlob(dir, h)
	if err != nil {
		t.Fatal(err)
	}

	if string(content) != "test blob content" {
		t.Error("blob content mismatch")
	}
}

func TestTreeRoundtrip(t *testing.T) {
	dir := t.TempDir()

	// Create some blobs first
	blob1, _ := WriteBlob(dir, []byte("file1 content"))
	blob2, _ := WriteBlob(dir, []byte("file2 content"))

	entries := []TreeEntry{
		{Mode: ModeRegular, Name: "file1.txt", Hash: blob1, Type: TypeBlob},
		{Mode: ModeRegular, Name: "file2.txt", Hash: blob2, Type: TypeBlob},
		{Mode: ModeDir, Name: "subdir", Hash: hash.Zero, Type: TypeTree},
	}

	treeHash, err := WriteTree(dir, entries)
	if err != nil {
		t.Fatal(err)
	}

	readEntries, err := ReadTree(dir, treeHash)
	if err != nil {
		t.Fatal(err)
	}

	if len(readEntries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(readEntries))
	}

	// Entries should be sorted by name
	if readEntries[0].Name != "file1.txt" || readEntries[1].Name != "file2.txt" || readEntries[2].Name != "subdir" {
		t.Errorf("entries not sorted: %v", readEntries)
	}
}

func TestCommitRoundtrip(t *testing.T) {
	dir := t.TempDir()

	treeHash, _ := WriteBlob(dir, []byte("dummy"))

	c := NewCommitData(treeHash, nil, "Test User", "test@test.com", "host", "test commit", 0)
	commitHash, err := WriteCommit(dir, c)
	if err != nil {
		t.Fatal(err)
	}

	readCommit, err := ReadCommit(dir, commitHash)
	if err != nil {
		t.Fatal(err)
	}

	if readCommit.Author != "Test User" {
		t.Errorf("author mismatch: %s", readCommit.Author)
	}
	if readCommit.Message != "test commit" {
		t.Errorf("message mismatch: %s", readCommit.Message)
	}
}

func TestCreateBlob(t *testing.T) {
	dir := t.TempDir()

	// Create a temp file
	tmpFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("file content"), 0644); err != nil {
		t.Fatal(err)
	}

	h, err := CreateBlob(dir, tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	content, err := ReadBlob(dir, h)
	if err != nil {
		t.Fatal(err)
	}

	if string(content) != "file content" {
		t.Error("file blob content mismatch")
	}
}

func TestTypeString(t *testing.T) {
	if TypeBlob.String() != "blob" {
		t.Error("blob string mismatch")
	}
	if TypeTree.String() != "tree" {
		t.Error("tree string mismatch")
	}
	if TypeCommit.String() != "commit" {
		t.Error("commit string mismatch")
	}
}

func TestErrUnexpectedType(t *testing.T) {
	err := &ErrUnexpectedType{Got: TypeTree, Want: TypeBlob}
	if err.Error() == "" {
		t.Error("error should have a message")
	}
}
