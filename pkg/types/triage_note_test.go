package types

import "testing"

func TestTriageNoteValidate(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantValid bool
	}{
		{name: "valid plain text", body: "some note", wantValid: true},
		{name: "valid markdown", body: "**bold** and *italic*", wantValid: true},
		{name: "empty body", body: "", wantValid: false},
		{name: "whitespace only", body: "   \n  ", wantValid: false},
		{name: "dangerous html", body: "hello <script>alert(1)</script>", wantValid: false},
		{name: "unclosed fence", body: "```\ncode without close", wantValid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &TriageNote{Body: tt.body}
			msg, valid := n.Validate()
			if valid != tt.wantValid {
				t.Errorf("Validate() valid = %v, want %v (msg: %s)", valid, tt.wantValid, msg)
			}
			if !valid && msg == "" {
				t.Error("Validate() returned invalid with empty message")
			}
		})
	}
}
