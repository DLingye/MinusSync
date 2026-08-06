// Package fsck provides repository integrity verification.
package fsck

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/object"
)

// Report holds the result of an integrity check.
type Report struct {
	ObjectsChecked int
	Errors         []Error
	Warnings       []Warning
}

// Error is an integrity error.
type Error struct {
	Severity string
	Object   string
	Message  string
}

// Warning is an integrity warning.
type Warning struct {
	Object  string
	Message string
}

// Verify checks the integrity of all objects in the repository.
func Verify(objectsDir string, verbose bool) (*Report, error) {
	report := &Report{}

	prefixDir := objectsDir
	entries, err := os.ReadDir(prefixDir)
	if err != nil {
		return report, fmt.Errorf("read objects dir: %w", err)
	}

	for _, prefix := range entries {
		if !prefix.IsDir() || len(prefix.Name()) != 2 {
			continue
		}

		objDir := filepath.Join(objectsDir, prefix.Name())
		objEntries, err := os.ReadDir(objDir)
		if err != nil {
			continue
		}

		for _, objFile := range objEntries {
			if objFile.IsDir() {
				continue
			}

			fullHex := prefix.Name() + objFile.Name()
			report.ObjectsChecked++

			if verbose {
				fmt.Printf("checking: %s\n", fullHex[:7])
			}

			h, err := hash.FromHex(fullHex)
			if err != nil {
				report.Errors = append(report.Errors, Error{
					Severity: "error",
					Object:   fullHex[:7],
					Message:  fmt.Sprintf("invalid hash: %v", err),
				})
				continue
			}

			// Read and verify the object with content extraction
			content, header, err := object.ReadContent(objectsDir, h)
			if err != nil {
				report.Errors = append(report.Errors, Error{
					Severity: "error",
					Object:   fullHex[:7],
					Message:  fmt.Sprintf("read error: %v", err),
				})
				continue
			}

			// Type-specific validation
			switch header.Type {
			case object.TypeCommit:
				_, err := object.ParseCommit(content)
				if err != nil {
					report.Errors = append(report.Errors, Error{
						Severity: "error",
						Object:   fullHex[:7],
						Message:  fmt.Sprintf("parse commit: %v", err),
					})
				}
			case object.TypeTree:
				_, err := object.ParseTree(content)
				if err != nil {
					report.Errors = append(report.Errors, Error{
						Severity: "error",
						Object:   fullHex[:7],
						Message:  fmt.Sprintf("parse tree: %v", err),
					})
				}
			case object.TypeTag:
				_, err := object.ParseTag(content)
				if err != nil {
					report.Errors = append(report.Errors, Error{
						Severity: "error",
						Object:   fullHex[:7],
						Message:  fmt.Sprintf("parse tag: %v", err),
					})
				}
			case object.TypeBlob:
				// Blob content is opaque — no further validation needed
			}
		}
	}

	return report, nil
}
