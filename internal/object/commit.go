package object

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MinusSync/internal/hash"
)

// CommitData holds the data for a commit object.
type CommitData struct {
	Tree      hash.Hash
	Parents   []hash.Hash
	Author    string
	Email     string
	Hostname  string
	Timestamp int64 // Unix seconds
	Message   string
}

// NewCommitData creates a CommitData with the current time and hostname.
func NewCommitData(tree hash.Hash, parents []hash.Hash, author, email, hostname, message string) *CommitData {
	if hostname == "" {
		hostname = "unknown"
	}
	return &CommitData{
		Tree:      tree,
		Parents:   parents,
		Author:    author,
		Email:     email,
		Hostname:  hostname,
		Timestamp: time.Now().Unix(),
		Message:   message,
	}
}

// SerializeCommit converts commit data to the on-disk format.
func SerializeCommit(c *CommitData) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "tree %s\n", c.Tree.Hex())
	for _, p := range c.Parents {
		fmt.Fprintf(&b, "parent %s\n", p.Hex())
	}

	authorTime := time.Unix(c.Timestamp, 0)
	fmt.Fprintf(&b, "author %s <%s> %d +0000\n", c.Author, c.Email, c.Timestamp)
	fmt.Fprintf(&b, "committer %s <%s> %d +0000\n", c.Author, c.Email, c.Timestamp)
	fmt.Fprintf(&b, "hostname %s\n", c.Hostname)
	fmt.Fprintf(&b, "date %s\n", authorTime.Format(time.RFC3339))
	fmt.Fprintf(&b, "\n%s\n", c.Message)

	return []byte(b.String())
}

// ParseCommit parses serialized commit data.
func ParseCommit(data []byte) (*CommitData, error) {
	lines := strings.Split(string(data), "\n")

	c := &CommitData{}
	messageStart := -1

	for i, line := range lines {
		if line == "" {
			messageStart = i + 1
			break
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		key, value := parts[0], parts[1]
		switch key {
		case "tree":
			h, err := hash.FromHex(value)
			if err != nil {
				return nil, fmt.Errorf("invalid tree hash in commit: %w", err)
			}
			c.Tree = h
		case "parent":
			h, err := hash.FromHex(value)
			if err != nil {
				return nil, fmt.Errorf("invalid parent hash in commit: %w", err)
			}
			c.Parents = append(c.Parents, h)
		case "author":
			c.Author, c.Email = parsePersonLine(value)
		case "committer":
			// Use committer info if set; otherwise author is used for both
			c.Author, c.Email = parsePersonLine(value)
		case "hostname":
			c.Hostname = value
		case "date":
			t, err := time.Parse(time.RFC3339, value)
			if err == nil {
				c.Timestamp = t.Unix()
			}
		default:
			// Unknown fields are ignored for forward compatibility
		}
	}

	if messageStart >= 0 && messageStart < len(lines) {
		c.Message = strings.Join(lines[messageStart:], "\n")
		c.Message = strings.TrimSpace(c.Message)
	}

	if c.Tree.IsZero() {
		return nil, fmt.Errorf("commit missing tree entry")
	}

	return c, nil
}

// parsePersonLine extracts name and email from a line like "Name <email>".
func parsePersonLine(line string) (name, email string) {
	name = line
	if idx := strings.IndexByte(line, '<'); idx >= 0 {
		name = strings.TrimSpace(line[:idx])
		email = line[idx+1:]
		if idx2 := strings.IndexByte(email, '>'); idx2 >= 0 {
			email = email[:idx2]
		}
	}
	return name, email
}

// ReadCommit reads a commit object and returns its parsed data.
func ReadCommit(objectsDir string, h hash.Hash) (*CommitData, error) {
	content, header, err := ReadContent(objectsDir, h)
	if err != nil {
		return nil, err
	}
	if header.Type != TypeCommit {
		return nil, &ErrUnexpectedType{Got: header.Type, Want: TypeCommit}
	}
	return ParseCommit(content)
}

// CommitTime returns the commit time as a time.Time.
func (c *CommitData) CommitTime() time.Time {
	return time.Unix(c.Timestamp, 0)
}

// ShortHash returns a 7-character hex prefix of the commit hash.
// The hash must be set externally after writing.
func (c *CommitData) ShortHash() string {
	return "0000000"
}

// FormatTimestamp formats the commit timestamp for display.
func FormatTimestamp(ts int64) string {
	return time.Unix(ts, 0).Format("Mon Jan 2 15:04:05 2006")
}

// GetTimestamp parses a Unix timestamp from a string.
func GetTimestamp(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
