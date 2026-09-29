package utils

import (
	"fmt"
	"regexp"
	"strings"
)

var dangerousTagPattern = regexp.MustCompile(`(?i)<\s*/?\s*(script|iframe|style|object|embed)\b`)

// ValidateMarkdown performs basic sanity checks on Markdown content.
// It returns an error describing the problem, or nil if the content is acceptable.
func ValidateMarkdown(text string) error {
	fenceCount := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			fenceCount++
		}
	}
	if fenceCount%2 != 0 {
		return fmt.Errorf("unclosed code fence (```) detected")
	}

	if dangerousTagPattern.MatchString(text) {
		return fmt.Errorf("HTML tags script, iframe, style, object, and embed are not allowed")
	}

	return nil
}
