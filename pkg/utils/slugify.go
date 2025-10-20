package utils

import (
	"regexp"
	"strings"
)

// Slugify converts a string to a URL-safe slug by:
// - Converting to lowercase
// - Replacing spaces and special characters with hyphens
// - Removing multiple consecutive hyphens
// - Trimming hyphens from start and end
func Slugify(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Replace non-alphanumeric characters with hyphens
	re := regexp.MustCompile(`[^a-z0-9]+`)
	text = re.ReplaceAllString(text, "-")

	// Remove multiple consecutive hyphens
	re = regexp.MustCompile(`-+`)
	text = re.ReplaceAllString(text, "-")

	// Trim hyphens from start and end
	text = strings.Trim(text, "-")

	return text
}
