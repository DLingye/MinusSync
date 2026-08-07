package search

import (
	"encoding/binary"
	"os"

	"github.com/MinusSync/internal/hash"
	"github.com/MinusSync/internal/object"
)

// Index is a simple file-based inverted index for search.
// For simplicity, we use an in-memory index with JSON persistence.
// A production version would use BoltDB.
type Index struct {
	path      string
	terms     map[string]*PostingList // term → postings
	docs      map[uint32]*DocInfo     // docID → doc info
	nextDocID uint32
	dirty     bool
}

// PostingList holds postings for a term.
type PostingList struct {
	Postings []Posting
}

// Posting is one occurrence of a term in a document.
type Posting struct {
	DocID     uint32
	Frequency uint32
	Positions []uint32
}

// DocInfo holds metadata about an indexed document.
type DocInfo struct {
	Path string
	Hash hash.Hash
}

// SearchResult is a single search result.
type Result struct {
	File     string
	Score    float64
	Line     int
	Column   int
	LineText string
}

// Query represents a search query.
type Query struct {
	Terms         []string
	CaseSensitive bool
	MaxResults    int
}

// New creates a new search index.
func New(path string) *Index {
	return &Index{
		path:      path,
		terms:     make(map[string]*PostingList),
		docs:      make(map[uint32]*DocInfo),
		nextDocID: 1,
	}
}

// Open opens or creates a search index at the given path.
func Open(path string) (*Index, error) {
	idx := New(path)
	// Ensure directory exists
	os.MkdirAll(path, 0755)
	return idx, nil
}

// Close closes the index (no-op for in-memory index).
func (idx *Index) Close() error {
	return nil
}

// Update rebuilds the search index from the repository's working tree.
func (idx *Index) Update(objectsDir string, treeHash hash.Hash) error {
	// Clear existing index
	idx.terms = make(map[string]*PostingList)
	idx.docs = make(map[uint32]*DocInfo)
	idx.nextDocID = 1
	idx.dirty = true

	tokenizer := NewTokenizer()

	// Walk the tree and index each blob
	err := object.WalkTree(objectsDir, treeHash, func(path string, entry object.TreeEntry) error {
		// Only index text files
		if !isIndexable(path) {
			return nil
		}

		content, err := object.ReadBlob(objectsDir, entry.Hash)
		if err != nil {
			return nil // Skip unreadable files
		}

		// Assign a doc ID
		docID := idx.nextDocID
		idx.nextDocID++

		idx.docs[docID] = &DocInfo{
			Path: path,
			Hash: entry.Hash,
		}

		// Tokenize and index
		tokens := tokenizer.Tokenize(string(content), path)

		// Count term frequencies for this doc
		termFreq := make(map[string]*Posting)
		for pos, tok := range tokens {
			p, ok := termFreq[tok]
			if !ok {
				p = &Posting{DocID: docID}
				termFreq[tok] = p
			}
			p.Frequency++
			p.Positions = append(p.Positions, uint32(pos))
		}

		// Add to global index
		for term, posting := range termFreq {
			pl, ok := idx.terms[term]
			if !ok {
				pl = &PostingList{}
				idx.terms[term] = pl
			}
			pl.Postings = append(pl.Postings, *posting)
		}

		return nil
	})

	return err
}

// Search executes a query against the index.
func (idx *Index) Search(q Query) ([]Result, error) {
	if len(q.Terms) == 0 {
		return nil, nil
	}

	// Get postings for each term
	var allPostings [][]Posting
	for _, term := range q.Terms {
		if !q.CaseSensitive {
			term = toLower(term)
		}
		if pl, ok := idx.terms[term]; ok {
			allPostings = append(allPostings, pl.Postings)
		} else {
			return nil, nil // One term not found = no results
		}
	}

	if len(allPostings) == 0 {
		return nil, nil
	}

	// Intersect postings (AND semantics)
	intersection := intersectPostings(allPostings)

	// Score and convert to results
	var results []Result
	for _, posting := range intersection {
		doc, ok := idx.docs[posting.DocID]
		if !ok {
			continue
		}

		// Simple TF score
		score := float64(posting.Frequency)

		results = append(results, Result{
			File:  doc.Path,
			Score: score,
		})
	}

	// Sort by score descending and limit
	sortResults(results)

	if q.MaxResults > 0 && len(results) > q.MaxResults {
		results = results[:q.MaxResults]
	}

	return results, nil
}

// intersectPostings finds document postings present in all posting lists.
func intersectPostings(allPostings [][]Posting) []Posting {
	if len(allPostings) == 0 {
		return nil
	}

	// Start with docs from first term
	docIDs := make(map[uint32]*Posting)
	for _, p := range allPostings[0] {
		cp := p
		docIDs[p.DocID] = &cp
	}

	// Intersect with remaining terms
	for _, postings := range allPostings[1:] {
		nextDocIDs := make(map[uint32]*Posting)
		for _, p := range postings {
			if existing, ok := docIDs[p.DocID]; ok {
				// Combine frequencies
				combined := *existing
				combined.Frequency += p.Frequency
				nextDocIDs[p.DocID] = &combined
			}
		}
		docIDs = nextDocIDs
	}

	var result []Posting
	for _, p := range docIDs {
		result = append(result, *p)
	}
	return result
}

// sortResults sorts results by score descending using simple selection sort.
func sortResults(results []Result) {
	for i := 0; i < len(results); i++ {
		best := i
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[best].Score {
				best = j
			}
		}
		results[i], results[best] = results[best], results[i]
	}
}

// Stats returns index statistics.
type IndexStats struct {
	DocumentCount int
	TermCount     int
}

// Stats returns current index statistics.
func (idx *Index) Stats() IndexStats {
	return IndexStats{
		DocumentCount: len(idx.docs),
		TermCount:     len(idx.terms),
	}
}

// isIndexable reports whether a file should be indexed.
func isIndexable(path string) bool {
	textExts := map[string]bool{
		".go": true, ".rs": true, ".c": true, ".cpp": true, ".h": true,
		".py": true, ".js": true, ".ts": true, ".java": true, ".rb": true,
		".md": true, ".txt": true, ".yaml": true, ".yml": true, ".json": true,
		".toml": true, ".xml": true, ".html": true, ".css": true, ".sh": true,
		".sql": true, ".proto": true,
	}
	for ext := range textExts {
		if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	// Simple ASCII lowercase
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

// Ensure binary is imported
var _ = binary.BigEndian
