package telegram

import (
	"fmt"
	"html"
	"testing"
)

func TestFormatCommitLink(t *testing.T) {
	link := func(url, text string) string {
		return fmt.Sprintf(`<a href="%s">%s</a>`, url, text)
	}

	tests := []struct {
		name    string
		message string
		url     string
		want    string
	}{
		{
			name:    "simple single line message",
			message: "fix: resolve login issue",
			url:     "https://github.com/repo/commit/abc123",
			want:    link("https://github.com/repo/commit/abc123", "fix: resolve login issue"),
		},
		{
			name:    "multi-line message adds ellipsis",
			message: "fix: resolve login issue\n\nDetailed description here",
			url:     "https://github.com/repo/commit/abc123",
			want:    link("https://github.com/repo/commit/abc123", "fix: resolve login issue") + " …",
		},
		{
			name:    "HTML characters are escaped in message",
			message: "feat: add <script>alert('xss')</script>",
			url:     "https://github.com/repo/commit/abc123",
			want:    link("https://github.com/repo/commit/abc123", html.EscapeString("feat: add <script>alert('xss')</script>")),
		},
		{
			name:    "message with leading/trailing whitespace",
			message: "  fix: trim whitespace  ",
			url:     "https://github.com/repo/commit/abc123",
			want:    link("https://github.com/repo/commit/abc123", "fix: trim whitespace"),
		},
		{
			name:    "multi-line with blank second line",
			message: "fix: something\n",
			url:     "https://github.com/repo/commit/abc123",
			want:    link("https://github.com/repo/commit/abc123", "fix: something"),
		},
		{
			name:    "multi-line with actual second line",
			message: "fix: something\n\nMore details",
			url:     "https://github.com/repo/commit/abc123",
			want:    link("https://github.com/repo/commit/abc123", "fix: something") + " …",
		},
		{
			name:    "empty message",
			message: "",
			url:     "https://github.com/repo/commit/abc123",
			want:    link("https://github.com/repo/commit/abc123", ""),
		},
		{
			name:    "message with ampersand",
			message: "fix A & B",
			url:     "https://github.com/repo/commit/abc123",
			want:    link("https://github.com/repo/commit/abc123", html.EscapeString("fix A & B")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCommitLink(tt.message, tt.url)
			if got != tt.want {
				t.Errorf("FormatCommitLink(%q, %q)\n  got:  %q\n  want: %q", tt.message, tt.url, got, tt.want)
			}
		})
	}
}
