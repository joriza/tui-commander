package ui

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Highlighter provides syntax highlighting for code files using chroma.
type Highlighter struct {
	lexer      chroma.Lexer
	formatter  chroma.Formatter
	style      *chroma.Style
	background string // tview background color (e.g., "black")
}

// NewHighlighter creates a highlighter for the given file extension.
// Returns nil if the extension is not supported.
func NewHighlighter(ext string, bgColor string) *Highlighter {
	lexer := lexers.Get(ext)
	if lexer == nil {
		lexer = lexers.Analyse("")
		if lexer == nil {
			return nil
		}
	}

	// Use a dark theme suitable for terminal
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	return &Highlighter{
		lexer:      lexer,
		formatter:  formatter,
		style:      style,
		background: bgColor,
	}
}

// HighlightToTviewSpans highlights and returns tview Spans directly.
// This is the preferred method for tview integration.
func (h *Highlighter) HighlightToTviewSpans(lines []string) ([]string, error) {
	if len(lines) == 0 {
		return nil, nil
	}

	text := strings.Join(lines, "\n") + "\n"
	iterator, err := h.lexer.Tokenise(nil, text)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, token := range iterator.Tokens() {
		if token.Type == chroma.Error {
			continue
		}
		style := tokenStyle(token.Type)
		if style != "" {
			result = append(result, fmt.Sprintf("[%s]%s[-]", style, escapeTview(token.Value)))
		} else {
			result = append(result, escapeTview(token.Value))
		}
	}

	return result, nil
}

// tokenStyle maps chroma token types to tview color names.
func tokenStyle(t chroma.TokenType) string {
	// Use string representation and check categories
	s := t.String()
	switch {
	case strings.HasPrefix(s, "Keyword"):
		return "yellow"
	case strings.HasPrefix(s, "Name.Function"):
		return "cyan"
	case strings.HasPrefix(s, "Name.Class"):
		return "cyan"
	case strings.HasPrefix(s, "Name.Namespace"):
		return "cyan"
	case strings.HasPrefix(s, "Literal.String"):
		return "green"
	case strings.HasPrefix(s, "Literal.Number"):
		return "magenta"
	case strings.HasPrefix(s, "Comment"):
		return "gray"
	case strings.HasPrefix(s, "Operator"):
		return "white"
	case strings.HasPrefix(s, "Punctuation"):
		return "white"
	case strings.HasPrefix(s, "Name.Builtin"):
		return "cyan"
	case strings.HasPrefix(s, "Name.Constant"):
		return "magenta"
	case strings.HasPrefix(s, "Literal.String.Interpol"):
		return "green"
	case strings.HasPrefix(s, "Literal.String.Regex"):
		return "green"
	case strings.HasPrefix(s, "Name.Variable"):
		return "white"
	default:
		return ""
	}
}

// escapeTview escapes characters that have special meaning in tview tags.
func escapeTview(s string) string {
	s = strings.ReplaceAll(s, "[", `\\[`)
	s = strings.ReplaceAll(s, "]", `\]`)
	return s
}

// SupportedExtensions returns a list of extensions that have syntax highlighting.
func SupportedExtensions() []string {
	return lexers.Names(true)
}

// HasSyntaxHighlighting returns true if the extension has syntax highlighting support.
func HasSyntaxHighlighting(ext string) bool {
	return lexers.Get(ext) != nil
}

// normalizeLineEndings converts CRLF and CR line endings to LF.
func normalizeLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

// HighlightLines highlights lines and returns tview-compatible formatted strings.
// This is the main entry point for the pager/viewer.
func HighlightLines(ext string, lines []string, bgColor string) ([]string, error) {
	highlighter := NewHighlighter(ext, "black")
	if highlighter == nil {
		return lines, nil // fallback to plain text
	}
	// Normalize line endings in input lines
	normalizedLines := make([]string, len(lines))
	for i, line := range lines {
		normalizedLines[i] = normalizeLineEndings(line)
	}
	return highlighter.HighlightToTviewSpans(normalizedLines)
}
