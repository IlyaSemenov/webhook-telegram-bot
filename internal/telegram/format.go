package telegram

import (
	"fmt"
	"html"
	"net/url"
	"strings"
)

// FormatLink returns an HTML link with the given HTML text, or only the text if rawURL is not an absolute HTTP or HTTPS URL.
func FormatLink(rawURL, text string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return text
	}
	return fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(rawURL), text)
}

// FormatCommitLink returns the HTML-escaped first line of the message, linked to commitURL by the rules of FormatLink.
// If the message has multiple lines, it appends " …" to the result.
func FormatCommitLink(message, commitURL string) string {
	lines := strings.Split(strings.TrimSpace(message), "\n")
	firstLine := strings.TrimSpace(lines[0])

	link := FormatLink(commitURL, html.EscapeString(firstLine))

	if len(lines) > 1 {
		return link + " …"
	}
	return link
}
