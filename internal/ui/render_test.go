package ui

import (
	"strings"
	"testing"

	"tui-commander/internal/core"
)

func TestTruncateMiddle(t *testing.T) {
	tests := []struct {
		name string
		in   string
		w    int
		want string
	}{
		{
			name: "shorter than width unchanged",
			in:   "short.txt",
			w:    30,
			want: "short.txt",
		},
		{
			name: "exact length unchanged",
			in:   "12345",
			w:    5,
			want: "12345",
		},
		{
			name: "w <= 1 returns ellipsis",
			in:   "anything",
			w:    1,
			want: "…",
		},
		{
			name: "multi-byte runes measured by rune length",
			in:   "áéíóú.txt",
			w:    10,
			want: "áéíóú.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateMiddle(tt.in, tt.w)
			if got != tt.want {
				t.Fatalf("truncateMiddle(%q, %d) = %q, want %q", tt.in, tt.w, got, tt.want)
			}
		})
	}
}

func TestTruncateMiddleLongASCII(t *testing.T) {
	name := "very-long-file-name-that-keeps-going.tar.gz"
	w := 20

	got := truncateMiddle(name, w)

	if n := len([]rune(got)); n != w {
		t.Fatalf("rune length = %d, want %d (got %q)", n, w, got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("result %q does not contain the middle ellipsis", got)
	}
	if !strings.HasPrefix(got, "very") {
		t.Fatalf("result %q lost the leading runes", got)
	}
	if !strings.HasSuffix(got, "gz") {
		t.Fatalf("result %q lost the trailing extension", got)
	}
}

func TestTruncateMiddleMultiByteCountsRunes(t *testing.T) {
	// Seven multi-byte runes; the byte length is larger than w, but the
	// rune length equals w, so the string must be returned unchanged.
	in := "áéíóúñü"
	if n := len([]rune(in)); n != 7 {
		t.Fatalf("test setup: rune length = %d, want 7", n)
	}
	if len(in) <= 7 {
		t.Fatalf("test setup: byte length %d should exceed rune length 7", len(in))
	}

	got := truncateMiddle(in, 7)
	if got != in {
		t.Fatalf("truncateMiddle(%q, 7) = %q, want unchanged", in, got)
	}
}

func TestNameColumnWidth(t *testing.T) {
	tests := []struct {
		name       string
		innerWidth int
		want       int
	}{
		{
			name:       "wide width subtracts fixed columns",
			innerWidth: 120,
			want:       120 - 28,
		},
		{
			name:       "narrow width clamps to minimum 20",
			innerWidth: 30,
			want:       20,
		},
		{
			name:       "at clamp boundary",
			innerWidth: 48,
			want:       20,
		},
		{
			name:       "just above clamp boundary",
			innerWidth: 49,
			want:       21,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nameColumnWidth(tt.innerWidth)
			if got != tt.want {
				t.Fatalf("nameColumnWidth(%d) = %d, want %d", tt.innerWidth, got, tt.want)
			}
		})
	}
}

func TestNameColumnWidthNonDecreasing(t *testing.T) {
	prev := nameColumnWidth(0)
	for w := 1; w <= 200; w++ {
		got := nameColumnWidth(w)
		if got < prev {
			t.Fatalf("nameColumnWidth(%d) = %d is less than nameColumnWidth(%d) = %d", w, got, w-1, prev)
		}
		prev = got
	}
}

func TestPagerTitle(t *testing.T) {
	tests := []struct {
		name      string
		fileName  string
		preview   core.TextPreview
		wantName  string
		wantSize  string
		wantTrunc bool
	}{
		{
			name:      "plain file carries name and size",
			fileName:  "notes.txt",
			preview:   core.TextPreview{Size: 1536},
			wantName:  "notes.txt",
			wantSize:  "1.5KB",
			wantTrunc: false,
		},
		{
			name:      "truncated preview is marked",
			fileName:  "big.log",
			preview:   core.TextPreview{Size: 2048, Truncated: true},
			wantName:  "big.log",
			wantSize:  "2.0KB",
			wantTrunc: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title := pagerTitle(tt.fileName, tt.preview)
			if !strings.Contains(title, tt.wantName) {
				t.Fatalf("title %q does not contain the file name %q", title, tt.wantName)
			}
			if !strings.Contains(title, tt.wantSize) {
				t.Fatalf("title %q does not contain the size %q", title, tt.wantSize)
			}
			if hasTrunc := strings.Contains(title, "truncated"); hasTrunc != tt.wantTrunc {
				t.Fatalf("title %q truncated marker = %v, want %v", title, hasTrunc, tt.wantTrunc)
			}
		})
	}
}

func TestPagerBodyText(t *testing.T) {
	const content = "hello\nworld\n"
	got := pagerBody(core.TextPreview{Text: content, Size: int64(len(content))})
	if got != content {
		t.Fatalf("pagerBody() = %q, want %q", got, content)
	}
}

func TestPagerBodyBinaryDoesNotDumpBytes(t *testing.T) {
	got := pagerBody(core.TextPreview{Binary: true, Size: 2048, Text: "raw-bytes"})
	if !strings.Contains(got, "Binary") {
		t.Fatalf("binary body %q does not mention that the file is binary", got)
	}
	if !strings.Contains(got, "2.0KB") {
		t.Fatalf("binary body %q does not carry the size", got)
	}
	if strings.Contains(got, "raw-bytes") {
		t.Fatalf("binary body %q dumped raw content", got)
	}
}
