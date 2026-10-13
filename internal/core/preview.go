package core

import (
	"bytes"
	"io"
	"os"
	"strings"
)

// maxPreviewLineRunes caps a single line of a preview. A huge or minified line
// must not blow up memory or the renderer, so any line longer than this is
// shortened and marked with an ellipsis.
const maxPreviewLineRunes = 10000

// utf8BOM is the UTF-8 byte order mark stripped from previews.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// TextPreview is a bounded, display-ready sample of a file's contents.
type TextPreview struct {
	Text      string // the text to display ("" when Binary)
	Size      int64  // total size of the file on disk
	Truncated bool   // true when only the first maxBytes were read
	Binary    bool   // true when the sample looks like binary data
}

// ReadTextPreview reads at most maxBytes from the start of path and returns a
// display-ready preview. A read error (missing file, permission) is returned
// unchanged.
//
// Files larger than maxBytes are sampled from the start and flagged Truncated.
// A UTF-8 BOM is stripped. Each line is capped at maxPreviewLineRunes runes
// (lines are split on '\n' only, so tabs and CRLF endings survive as-is).
// Binary samples -- a NUL byte, or more than 30% non-text control bytes --
// yield Binary true with an empty Text.
func ReadTextPreview(path string, maxBytes int64) (TextPreview, error) {
	f, err := os.Open(path)
	if err != nil {
		return TextPreview{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return TextPreview{}, err
	}
	size := info.Size()

	if maxBytes < 0 {
		maxBytes = 0
	}
	truncated := size > maxBytes

	data, err := io.ReadAll(io.LimitReader(f, maxBytes))
	if err != nil {
		return TextPreview{}, err
	}

	// Strip a leading UTF-8 BOM before detection and display.
	data = bytes.TrimPrefix(data, utf8BOM)

	preview := TextPreview{Size: size, Truncated: truncated}
	if isBinarySample(data) {
		preview.Binary = true
		return preview, nil
	}
	preview.Text = capPreviewLines(string(data))
	return preview, nil
}

// isBinarySample reports whether the sampled bytes look like binary data: a
// NUL byte anywhere, or more than 30% non-text control bytes. A control byte
// is any byte below 0x20 other than tab, newline or carriage return (0x7f is
// above the threshold and is therefore not counted).
func isBinarySample(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data, 0x00) >= 0 {
		return true
	}
	control := 0
	for _, b := range data {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			control++
		}
	}
	// Strictly more than 30%: control*10 > len*3.
	return control*10 > len(data)*3
}

// capPreviewLines caps every line of text at maxPreviewLineRunes runes,
// appending an ellipsis to lines that are shortened. Lines are split on '\n'
// only. When no line needs capping the original string is returned unchanged,
// so invalid UTF-8 inside short lines is never transformed.
func capPreviewLines(text string) string {
	if text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	changed := false
	for i, line := range lines {
		// len counts bytes; a line with at most maxPreviewLineRunes bytes has
		// at most that many runes and never needs capping.
		if len(line) > maxPreviewLineRunes {
			lines[i] = capLineRunes(line)
			changed = true
		}
	}
	if !changed {
		return text
	}
	return strings.Join(lines, "\n")
}

// capLineRunes shortens a single line to maxPreviewLineRunes runes, ending it
// with an ellipsis when it is truncated. Lines that already fit are returned
// unchanged.
func capLineRunes(line string) string {
	r := []rune(line)
	if len(r) <= maxPreviewLineRunes {
		return line
	}
	return string(r[:maxPreviewLineRunes]) + "…"
}
