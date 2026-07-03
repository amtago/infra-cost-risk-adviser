// Package tags loads a required-tags list from a plain-text file.
// Format: one tag name per line; lines starting with # are comments; blank lines ignored.
package tags

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadFile reads a required-tags file and returns the tag names.
func LoadFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening required-tags file: %w", err)
	}
	defer f.Close()

	var tags []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tags = append(tags, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading required-tags file: %w", err)
	}
	return tags, nil
}
