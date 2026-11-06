package telegram

import (
	"fmt"
	"html"
	"strings"
)

// FormatCommitLink returns an HTML link to a commit with the first line of the message.
// If the message has multiple lines, it appends " …" outside the link tag.
// The message is HTML-escaped for safe display.
func FormatCommitLink(message, url string) string {
	lines := strings.Split(strings.TrimSpace(message), "\n")
	firstLine := strings.TrimSpace(lines[0])

	link := fmt.Sprintf("<a href=\"%s\">%s</a>", url, html.EscapeString(firstLine))

	if len(lines) > 1 {
		return link + " …"
	}
	return link
}
