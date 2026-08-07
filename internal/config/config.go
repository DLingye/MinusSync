// Package config provides INI-style configuration file management.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds configuration data organized into sections.
type Config struct {
	path     string
	sections map[string]map[string]string
}

// Load reads a configuration file from disk.
func Load(path string) (*Config, error) {
	c := &Config{
		path:     path,
		sections: make(map[string]map[string]string),
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	defer f.Close()

	var currentSection string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Parse section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line[1 : len(line)-1]
			if _, ok := c.sections[currentSection]; !ok {
				c.sections[currentSection] = make(map[string]string)
			}
			continue
		}

		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			// Lines without '=' are ignored
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		if currentSection == "" {
			currentSection = "core"
			if _, ok := c.sections[currentSection]; !ok {
				c.sections[currentSection] = make(map[string]string)
			}
		}

		c.sections[currentSection][key] = value
	}

	return c, scanner.Err()
}

// Save writes the configuration to disk.
func (c *Config) Save() error {
	if c.path == "" {
		return fmt.Errorf("no config path set")
	}

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(c.path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Sort sections for deterministic output
	sections := c.Sections()

	for _, section := range sections {
		fmt.Fprintf(f, "[%s]\n", section)
		keys := c.Keys(section)
		for _, key := range keys {
			value := c.sections[section][key]
			// Quote values containing spaces or special chars
			if strings.ContainsAny(value, " #;=") {
				value = fmt.Sprintf("%q", value)
			}
			fmt.Fprintf(f, "\t%s = %s\n", key, value)
		}
		fmt.Fprintln(f)
	}

	return nil
}

// Get returns the value for a key in a section, or empty string if not found.
func (c *Config) Get(section, key string) string {
	s, ok := c.sections[section]
	if !ok {
		return ""
	}
	return s[key]
}

// GetDefault returns the value or a default if not found.
func (c *Config) GetDefault(section, key, def string) string {
	v := c.Get(section, key)
	if v == "" {
		return def
	}
	return v
}

// Set sets a key-value pair in a section.
func (c *Config) Set(section, key, value string) {
	if _, ok := c.sections[section]; !ok {
		c.sections[section] = make(map[string]string)
	}
	c.sections[section][key] = value
}

// Unset removes a key from a section.
func (c *Config) Unset(section, key string) {
	if s, ok := c.sections[section]; ok {
		delete(s, key)
		if len(s) == 0 {
			delete(c.sections, section)
		}
	}
}

// Has reports whether a key exists in a section.
func (c *Config) Has(section, key string) bool {
	s, ok := c.sections[section]
	if !ok {
		return false
	}
	_, ok = s[key]
	return ok
}

// Keys returns all keys in a section, unsorted.
func (c *Config) Keys(section string) []string {
	s, ok := c.sections[section]
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	return keys
}

// Sections returns all section names.
func (c *Config) Sections() []string {
	sections := make([]string, 0, len(c.sections))
	for s := range c.sections {
		sections = append(sections, s)
	}
	return sections
}

// RemoveSection removes an entire section.
func (c *Config) RemoveSection(section string) {
	delete(c.sections, section)
}

// GetInt returns an integer value.
func (c *Config) GetInt(section, key string) (int, error) {
	v := c.Get(section, key)
	if v == "" {
		return 0, fmt.Errorf("key %s.%s not found", section, key)
	}
	return strconv.Atoi(v)
}

// GetBool returns a boolean value ("true", "1", "yes" are true).
func (c *Config) GetBool(section, key string) (bool, error) {
	v := strings.ToLower(c.Get(section, key))
	switch v {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off", "":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q for %s.%s", v, section, key)
	}
}

// Path returns the config file path.
func (c *Config) Path() string {
	return c.path
}
