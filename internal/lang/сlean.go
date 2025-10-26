package lang

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

func CleanText(s string) string {
	if s == "" {
		return s
	}

	if looksLikeHTML(s) {
		if t := htmlToText(s); t != "" {
			s = t
		} else {
			s = stripTagsFallback(s)
		}
	}

	s = reReplyHeaderAnywhere.ReplaceAllString(s, "\n")

	if loc := reSignatureAnywhere.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}

	if idx := findSignatureIndex(s); idx >= 0 {
		s = s[:idx]
	}

	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	s = reReplyHeader.ReplaceAllString(s, "\n")
	s = reForwardHeader.ReplaceAllString(s, "\n")
	s = reOriginalMessage.ReplaceAllString(s, "\n")

	lines := strings.Split(s, "\n")
	outLines := make([]string, 0, len(lines))
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if trim == "" {
			continue
		}
		if reQuoteLine.MatchString(trim) {
			continue
		}
		if reSignatureDelimiter.MatchString(trim) {
			continue
		}
		if reHeaderLike.MatchString(trim) {
			continue
		}
		outLines = append(outLines, trim)
	}
	s = strings.Join(outLines, "\n")

	s = reURL.ReplaceAllString(s, " ")
	s = reEmail.ReplaceAllString(s, " ")
	s = reSignatureWords.ReplaceAllString(s, " ")
	s = reMultiWS.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	s = trimNonLetters(s)
	s = reMultiNewlines.ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)

	return s
}

func findSignatureIndex(s string) int {
	candidates := []string{
		"\n--",
		"\n-- ",
		"\n—",
		"\n— ",
		"\n--\n",
	}
	for _, pat := range candidates {
		if idx := strings.Index(s, pat); idx != -1 {
			return idx
		}
	}
	if idx := strings.Index(s, " -- "); idx != -1 {
		return idx
	}
	if idx := strings.Index(s, " — "); idx != -1 {
		return idx
	}
	return -1
}

func looksLikeHTML(s string) bool {
	l := strings.ToLower(s)
	if strings.Contains(l, "<html") || strings.Contains(l, "<body") {
		return true
	}
	if strings.Contains(l, "<div") || strings.Contains(l, "<span") || strings.Contains(l, "<p") {
		return true
	}
	if strings.Contains(l, "&nbsp;") {
		return true
	}
	if strings.Contains(s, "<") && strings.Contains(s, ">") {
		return true
	}
	return false
}

func htmlToText(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var b strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			txt := strings.TrimSpace(n.Data)
			if txt != "" {
				if b.Len() > 0 {
					last := b.String()[b.Len()-1]
					if last != '\n' && last != ' ' {
						b.WriteByte(' ')
					}
				}
				b.WriteString(txt)
			}
			return
		}
		if n.Type == html.ElementNode {
			block := isBlockElement(n.Data)
			if block && b.Len() > 0 {
				b.WriteString("\n")
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
			if block && b.Len() > 0 {
				b.WriteString("\n")
			}
		}
	}
	f(doc)
	out := b.String()
	out = strings.ReplaceAll(out, "\u00a0", " ")
	out = reMultiWS.ReplaceAllString(out, " ")
	out = reMultiNewlines.ReplaceAllString(out, "\n\n")
	out = strings.TrimSpace(out)
	return out
}

func stripTagsFallback(s string) string {
	reTags := regexp.MustCompile(`(?s)<[^>]*>`)
	out := reTags.ReplaceAllString(s, " ")
	out = reMultiWS.ReplaceAllString(out, " ")
	out = strings.TrimSpace(out)
	return out
}

func isBlockElement(tag string) bool {
	switch strings.ToLower(tag) {
	case "p", "div", "br", "li", "ul", "ol", "table", "tr", "td", "header", "footer", "section", "article", "h1", "h2", "h3", "h4":
		return true
	default:
		return false
	}
}

var (
	reReplyHeaderAnywhere = regexp.MustCompile(`(?mi)(?:^|[\r\n>]\s*)(?:on\s+.+?wrote:?.*$|am\s+.+?schrieb:?.*$)`)

	reSignatureAnywhere = regexp.MustCompile(`(?mi)(?:^|[\r\n]|[\.!\?]\s)(?:regards|best regards|best|cheers|sincerely|yours sincerely|yours truly|thanks|thank you|с уважением|с наилучшими пожеланиями|спасибо|mit freundlichen grüßen|viele grüße|beste grüße|herzliche grüße|liebe grüße|grüße|danke)[\s\,\.\-]*`)

	reReplyHeader = regexp.MustCompile(`(?mi)^(?:on\s+.+wrote:.*$|am\s+.+schrieb:.*$|on\s+.+wrote:.*$)`)

	reHeaderLike = regexp.MustCompile(`(?mi)^(?:from|sent|to|subject|date|cc|von|an|betreff|gesendet):\s+.*$`)

	reForwardHeader = regexp.MustCompile(`(?mi)^(?:begin forwarded message:|forwarded message:|weitergeleitete nachricht|weitergeleitete nachricht:).*`)

	reOriginalMessage = regexp.MustCompile(`(?mi)-----Original Message-----|-----Forwarded message-----|-----Ursprüngliche Nachricht-----`)

	reQuoteLine = regexp.MustCompile(`(?m)^[>\|].*$`)

	reSignatureDelimiter = regexp.MustCompile(`(?mi)^(--\s*$|__\s*$|-\s*$)`)

	reSignatureWords = regexp.MustCompile(`(?mi)^(?:regards|best regards|best|cheers|sincerely|yours sincerely|yours truly|thanks|thank you|с уважением|с наилучшими пожеланиями|спасибо|mit freundlichen grüßen|viele grüße|beste grüße|herzliche grüße|liebe grüße|grüße|danke)[\s\,\.\-]*`)

	reURL = regexp.MustCompile(`(?i)\bhttps?://[^\s]+|\bwww\.[^\s]+`)

	reEmail = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)

	reMultiWS = regexp.MustCompile(`\s{2,}`)

	reMultiNewlines = regexp.MustCompile(`\n{3,}`)
)

func trimNonLetters(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	start := 0
	end := len(r)
	for i, ch := range r {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			start = i
			break
		}
	}
	for i := len(r) - 1; i >= 0; i-- {
		if unicode.IsLetter(r[i]) || unicode.IsDigit(r[i]) {
			end = i + 1
			break
		}
	}
	if start >= end {
		return ""
	}
	return strings.TrimSpace(string(r[start:end]))
}
