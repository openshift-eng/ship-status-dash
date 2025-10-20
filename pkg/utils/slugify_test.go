package utils

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "Prow", want: "prow"},
		{in: "Build Farm", want: "build-farm"},
		{in: "Sippy-Auth", want: "sippy-auth"},
		{in: "  Leading and  trailing   spaces  ", want: "leading-and-trailing-spaces"},
		{in: "Multiple---Hyphens", want: "multiple-hyphens"},
		{in: "Special!@#$%^&*()Chars", want: "special-chars"},
		{in: "Mixed_Case_and-Separators", want: "mixed-case-and-separators"},
		{in: "123 Numbers 456", want: "123-numbers-456"},
		{in: "__underscores__", want: "underscores"},
		{in: "Café Déjà", want: "caf-d-j"}, // non-ascii letters dropped
		{in: "", want: ""},
		{in: "---", want: ""},
	}

	for _, tt := range tests {
		got := Slugify(tt.in)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
