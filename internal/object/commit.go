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
	Sequence  uint64 // Sequence number (1-based, increments each commit)
	Author    string
	Email     string
	Hostname  string
	Timestamp int64  // Unix seconds
	TZOffset  int    // Timezone offset in seconds (e.g., 28800 for +0800)
	Message   string
}

// NewCommitData creates a CommitData with the current time and hostname.
func NewCommitData(tree hash.Hash, parents []hash.Hash, author, email, hostname, message string, parentSeq uint64) *CommitData {
	if hostname == "" {
		hostname = "unknown"
	}
	now := time.Now()
	_, tzOffset := now.Zone()

	seq := parentSeq + 1

	return &CommitData{
		Tree:      tree,
		Parents:   parents,
		Sequence:  seq,
		Author:    author,
		Email:     email,
		Hostname:  hostname,
		Timestamp: now.Unix(),
		TZOffset:  tzOffset,
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

	fmt.Fprintf(&b, "sequence %d\n", c.Sequence)

	tzSign := "+"
	if c.TZOffset < 0 {
		tzSign = "-"
		c.TZOffset = -c.TZOffset
	}
	tzHours := c.TZOffset / 3600
	tzMins := (c.TZOffset % 3600) / 60
	tzStr := fmt.Sprintf("%s%02d%02d", tzSign, tzHours, tzMins)

	fmt.Fprintf(&b, "author %s <%s> %d %s\n", c.Author, c.Email, c.Timestamp, tzStr)
	fmt.Fprintf(&b, "committer %s <%s> %d %s\n", c.Author, c.Email, c.Timestamp, tzStr)
	fmt.Fprintf(&b, "hostname %s\n", c.Hostname)
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
		case "sequence":
			seq, err := strconv.ParseUint(value, 10, 64)
			if err == nil {
				c.Sequence = seq
			}
		case "author":
			c.Author, c.Email, c.Timestamp, c.TZOffset = parsePersonLineFull(value)
		case "committer":
			a, e, ts, tz := parsePersonLineFull(value)
			if a != "" {
				c.Author = a
			}
			if e != "" {
				c.Email = e
			}
			if ts != 0 {
				c.Timestamp = ts
				c.TZOffset = tz
			}
		case "hostname":
			c.Hostname = value
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
	name, email, _, _ = parsePersonLineFull(line)
	return
}

// parsePersonLineFull extracts name, email, timestamp, and timezone from a line.
func parsePersonLineFull(line string) (name, email string, timestamp int64, tzOffset int) {
	// Find the email part <...>
	if idx := strings.IndexByte(line, '<'); idx >= 0 {
		name = strings.TrimSpace(line[:idx])
		rest := line[idx+1:]
		if idx2 := strings.IndexByte(rest, '>'); idx2 >= 0 {
			email = rest[:idx2]
			rest = strings.TrimSpace(rest[idx2+1:])
			// Parse timestamp and timezone: "1234567890 +0800"
			tsParts := strings.Fields(rest)
			if len(tsParts) >= 1 {
				ts, err := strconv.ParseInt(tsParts[0], 10, 64)
				if err == nil {
					timestamp = ts
				}
			}
			if len(tsParts) >= 2 {
				tzOffset = parseTZOffset(tsParts[1])
			}
		}
	}
	return
}

// parseTZOffset parses a timezone string like "+0800" or "-0500" to seconds.
func parseTZOffset(s string) int {
	if len(s) != 5 || (s[0] != '+' && s[0] != '-') {
		return 0
	}
	sign := 1
	if s[0] == '-' {
		sign = -1
	}
	hours, _ := strconv.Atoi(s[1:3])
	mins, _ := strconv.Atoi(s[3:5])
	return sign * (hours*3600 + mins*60)
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
	loc := time.FixedZone("", c.TZOffset)
	return time.Unix(c.Timestamp, 0).In(loc)
}

// FormatTimestamp formats the commit timestamp with timezone for display.
func FormatTimestamp(ts int64, tzOffset int) string {
	loc := time.FixedZone("", tzOffset)
	t := time.Unix(ts, 0).In(loc)
	tzSign := "+"
	if tzOffset < 0 {
		tzSign = "-"
		tzAbs := -tzOffset
		tzHours := tzAbs / 3600
		tzMins := (tzAbs % 3600) / 60
		return fmt.Sprintf("%s %s%02d%02d",
			t.Format("Mon Jan 2 15:04:05 2006"),
			tzSign, tzHours, tzMins)
	}
	tzHours := tzOffset / 3600
	tzMins := (tzOffset % 3600) / 60
	return fmt.Sprintf("%s %s%02d%02d",
		t.Format("Mon Jan 2 15:04:05 2006"),
		tzSign, tzHours, tzMins)
}

// GetTimestamp parses a Unix timestamp from a string.
func GetTimestamp(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
