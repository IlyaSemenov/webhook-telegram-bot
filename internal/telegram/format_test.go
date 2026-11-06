package telegram

import (
	"fmt"
	"html"
	"testing"
)

func TestFormatLink(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "HTTPS", url: "https://example.com/a", want: `<a href="https://example.com/a">text</a>`},
		{name: "HTTP", url: "http://example.com/a", want: `<a href="http://example.com/a">text</a>`},
		{name: "uppercase scheme", url: "HTTPS://example.com", want: `<a href="HTTPS://example.com">text</a>`},
		{name: "ampersand is escaped", url: "https://example.com/?a=1&b=2", want: `<a href="https://example.com/?a=1&amp;b=2">text</a>`},
		{name: "quote cannot close the attribute", url: `https://example.com/"><b>x`, want: `<a href="https://example.com/&#34;&gt;&lt;b&gt;x">text</a>`},
		{name: "empty", url: "", want: "text"},
		{name: "relative", url: "/a", want: "text"},
		{name: "no host", url: "https:///a", want: "text"},
		{name: "javascript scheme", url: "javascript:alert(1)", want: "text"},
		{name: "Telegram scheme", url: "tg://resolve?domain=x", want: "text"},
		{name: "malformed", url: "https://exa mple.com", want: "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatLink(tt.url, "text"); got != tt.want {
				t.Errorf("FormatLink(%q)\n  got:  %q\n  want: %q", tt.url, got, tt.want)
			}
		})
	}
}

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
