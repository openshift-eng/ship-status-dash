package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"
)

var dangerousTagPattern = regexp.MustCompile(`(?i)<\s*/?\s*(script|iframe|style|object|embed)\b`)

// ValidateMarkdown performs basic sanity checks on Markdown content using
// the gomarkdown/markdown parser. It returns an error describing the
// problem, or nil if the content is acceptable.
func ValidateMarkdown(text string) error {
	if err := checkUnclosedFences(text); err != nil {
		return err
	}

	p := parser.NewWithExtensions(parser.CommonExtensions)
	doc := p.Parse([]byte(text))

	var htmlErr error
	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		if !entering {
			return ast.GoToNext
		}
		switch n := node.(type) {
		case *ast.HTMLBlock:
			if dangerousTagPattern.Match(n.Literal) {
				htmlErr = fmt.Errorf("HTML tags script, iframe, style, object, and embed are not allowed")
				return ast.Terminate
			}
		case *ast.HTMLSpan:
			if dangerousTagPattern.Match(n.Literal) {
				htmlErr = fmt.Errorf("HTML tags script, iframe, style, object, and embed are not allowed")
				return ast.Terminate
			}
		}
		return ast.GoToNext
	})

	return htmlErr
}

func checkUnclosedFences(text string) error {
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
	return nil
}
