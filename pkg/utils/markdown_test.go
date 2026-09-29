package utils

import (
	"testing"
)

func TestValidateMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "plain text",
			input:   "Hello, world!",
			wantErr: false,
		},
		{
			name:    "valid markdown with formatting",
			input:   "# Heading\n\nSome **bold** and *italic* text.\n\n- list item",
			wantErr: false,
		},
		{
			name:    "valid code fence",
			input:   "```\ncode block\n```",
			wantErr: false,
		},
		{
			name:    "valid code fence with language",
			input:   "```go\nfmt.Println(\"hello\")\n```",
			wantErr: false,
		},
		{
			name:    "multiple valid code fences",
			input:   "```\nblock 1\n```\n\ntext\n\n```\nblock 2\n```",
			wantErr: false,
		},
		{
			name:    "unclosed code fence",
			input:   "```\ncode block without closing",
			wantErr: true,
			errMsg:  "unclosed code fence",
		},
		{
			name:    "script tag",
			input:   "Hello <script>alert('xss')</script>",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "iframe tag",
			input:   "Check this <iframe src=\"evil.com\"></iframe>",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "style tag",
			input:   "<style>body { display: none }</style>",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "case insensitive dangerous tag",
			input:   "<SCRIPT>alert(1)</SCRIPT>",
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "safe HTML tags allowed",
			input:   "Some <b>bold</b> and <a href=\"https://example.com\">link</a>",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: false,
		},
		{
			name:    "multiline plain text",
			input:   "line 1\nline 2\nline 3",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMarkdown(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
