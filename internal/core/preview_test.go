package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePreviewFile writes data to a file inside t.TempDir and returns its path.
func writePreviewFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestReadTextPreviewPlainASCII(t *testing.T) {
	const content = "hello\nworld\n"
	path := writePreviewFile(t, "plain.txt", []byte(content))

	got, err := ReadTextPreview(path, 1<<20)
	if err != nil {
		t.Fatalf("ReadTextPreview returned error: %v", err)
	}
	if got.Text != content {
		t.Fatalf("Text = %q, want %q", got.Text, content)
	}
	if got.Truncated {
		t.Fatalf("Truncated = true, want false")
	}
	if got.Binary {
		t.Fatalf("Binary = true, want false")
	}
	if got.Size != int64(len(content)) {
		t.Fatalf("Size = %d, want %d", got.Size, len(content))
	}
}

func TestReadTextPreviewBinaryNUL(t *testing.T) {
	path := writePreviewFile(t, "bin.dat", []byte("abc\x00def"))

	got, err := ReadTextPreview(path, 1<<20)
	if err != nil {
		t.Fatalf("ReadTextPreview returned error: %v", err)
	}
	if !got.Binary {
		t.Fatalf("Binary = false, want true")
	}
	if got.Text != "" {
		t.Fatalf("Text = %q, want empty for binary content", got.Text)
	}
}

func TestReadTextPreviewTruncated(t *testing.T) {
	path := writePreviewFile(t, "long.txt", []byte("0123456789ABCDEF"))

	got, err := ReadTextPreview(path, 10)
	if err != nil {
		t.Fatalf("ReadTextPreview returned error: %v", err)
	}
	if !got.Truncated {
		t.Fatalf("Truncated = false, want true")
	}
	if got.Text != "0123456789" {
		t.Fatalf("Text = %q, want %q", got.Text, "0123456789")
	}
	if got.Size != 16 {
		t.Fatalf("Size = %d, want 16", got.Size)
	}
}

func TestReadTextPreviewExactlyMaxBytesNotTruncated(t *testing.T) {
	path := writePreviewFile(t, "exact.txt", []byte("0123456789"))

	got, err := ReadTextPreview(path, 10)
	if err != nil {
		t.Fatalf("ReadTextPreview returned error: %v", err)
	}
	if got.Truncated {
		t.Fatalf("Truncated = true, want false when size == maxBytes")
	}
	if got.Text != "0123456789" {
		t.Fatalf("Text = %q, want %q", got.Text, "0123456789")
	}
}

func TestReadTextPreviewEmpty(t *testing.T) {
	path := writePreviewFile(t, "empty.txt", nil)

	got, err := ReadTextPreview(path, 1<<20)
	if err != nil {
		t.Fatalf("ReadTextPreview returned error: %v", err)
	}
	if got.Text != "" {
		t.Fatalf("Text = %q, want empty", got.Text)
	}
	if got.Truncated {
		t.Fatalf("Truncated = true, want false")
	}
	if got.Binary {
		t.Fatalf("Binary = true, want false")
	}
	if got.Size != 0 {
		t.Fatalf("Size = %d, want 0", got.Size)
	}
}

func TestReadTextPreviewCapsLongLine(t *testing.T) {
	long := strings.Repeat("x", 50000)
	content := long + "\ntail"
	path := writePreviewFile(t, "minified.js", []byte(content))

	got, err := ReadTextPreview(path, 1<<20)
	if err != nil {
		t.Fatalf("ReadTextPreview returned error: %v", err)
	}

	firstLine, rest, found := strings.Cut(got.Text, "\n")
	if !found {
		t.Fatalf("capped text lost the following line: %q", got.Text)
	}
	if rest != "tail" {
		t.Fatalf("rest of text = %q, want %q", rest, "tail")
	}
	if n := len([]rune(firstLine)); n != maxPreviewLineRunes+1 {
		t.Fatalf("capped line rune length = %d, want %d", n, maxPreviewLineRunes+1)
	}
	if !strings.HasSuffix(firstLine, "…") {
		t.Fatalf("capped line %q does not end with an ellipsis", firstLine)
	}
	if !strings.HasPrefix(firstLine, strings.Repeat("x", maxPreviewLineRunes)) {
		t.Fatalf("capped line lost its leading runes")
	}
}

func TestReadTextPreviewMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.txt")

	if _, err := ReadTextPreview(path, 1<<20); err == nil {
		t.Fatalf("ReadTextPreview returned nil error for a missing file")
	}
}

func TestReadTextPreviewPreservesCRLF(t *testing.T) {
	const content = "line1\r\nline2\r\n"
	path := writePreviewFile(t, "crlf.txt", []byte(content))

	got, err := ReadTextPreview(path, 1<<20)
	if err != nil {
		t.Fatalf("ReadTextPreview returned error: %v", err)
	}
	if got.Text != content {
		t.Fatalf("Text = %q, want %q", got.Text, content)
	}
}

func TestReadTextPreviewStripsUTF8BOM(t *testing.T) {
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hi")...)
	path := writePreviewFile(t, "bom.txt", content)

	got, err := ReadTextPreview(path, 1<<20)
	if err != nil {
		t.Fatalf("ReadTextPreview returned error: %v", err)
	}
	if got.Text != "hi" {
		t.Fatalf("Text = %q, want %q (BOM must be stripped)", got.Text, "hi")
	}
}

func TestReadTextPreviewFewControlBytesNotBinary(t *testing.T) {
	// One control byte in a long text run: well under the 30% threshold.
	data := append([]byte("text"), 0x01)
	data = append(data, []byte(strings.Repeat("a", 100))...)
	path := writePreviewFile(t, "nearly.txt", data)

	got, err := ReadTextPreview(path, 1<<20)
	if err != nil {
		t.Fatalf("ReadTextPreview returned error: %v", err)
	}
	if got.Binary {
		t.Fatalf("Binary = true, want false (control bytes below 30%%)")
	}
}

func TestReadTextPreviewMostlyControlBytesBinary(t *testing.T) {
	// Half the sampled bytes are control characters: over the 30% threshold.
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 'a', 'b', 'c', 'd'}
	path := writePreviewFile(t, "controls.bin", data)

	got, err := ReadTextPreview(path, 1<<20)
	if err != nil {
		t.Fatalf("ReadTextPreview returned error: %v", err)
	}
	if !got.Binary {
		t.Fatalf("Binary = false, want true (control bytes over 30%%)")
	}
}
