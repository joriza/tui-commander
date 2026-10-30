package ui

import (
	"os"
	"testing"
)

func TestParseEditorCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		wantProg string
		wantArgs []string
		wantOk   bool
	}{
		{
			name:     "simple notepad",
			cmd:      "notepad",
			wantProg: "notepad",
			wantArgs: nil,
			wantOk:   true,
		},
		{
			name:     "code with wait",
			cmd:      "code --wait",
			wantProg: "code",
			wantArgs: []string{"--wait"},
			wantOk:   true,
		},
		{
			name:     "quoted program with spaces",
			cmd:      `"C:\Program Files\VSCode\Code.exe" --wait`,
			wantProg: `C:\Program Files\VSCode\Code.exe`,
			wantArgs: []string{"--wait"},
			wantOk:   true,
		},
		{
			name:     "extra whitespace collapses",
			cmd:      "  code   --wait   --new-window  ",
			wantProg: "code",
			wantArgs: []string{"--wait", "--new-window"},
			wantOk:   true,
		},
		{
			name:     "tabs between tokens",
			cmd:      "vim\t--noplugin",
			wantProg: "vim",
			wantArgs: []string{"--noplugin"},
			wantOk:   true,
		},
		{
			name:     "empty string rejected",
			cmd:      "",
			wantProg: "",
			wantArgs: nil,
			wantOk:   false,
		},
		{
			name:     "whitespace only rejected",
			cmd:      "   \t\n  ",
			wantProg: "",
			wantArgs: nil,
			wantOk:   false,
		},
		{
			name:     "quoted empty rejected",
			cmd:      `"" --wait`,
			wantProg: "",
			wantArgs: nil,
			wantOk:   false,
		},
		{
			name:     "unclosed quote treated as program",
			cmd:      `"C:\Program Files\Tool.exe`,
			wantProg: `C:\Program Files\Tool.exe`,
			wantArgs: nil,
			wantOk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prog, args, ok := parseEditorCommand(tt.cmd)
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
			if prog != tt.wantProg {
				t.Fatalf("prog = %q, want %q", prog, tt.wantProg)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("len(args)=%d, want %d (got %v)", len(args), len(tt.wantArgs), args)
				return
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Fatalf("args[%d]=%q, want %q", i, args[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestResolveEditor(t *testing.T) {
	// Save and restore env
	oldEditor := os.Getenv("EDITOR")
	oldVisual := os.Getenv("VISUAL")
	defer func() {
		_ = os.Setenv("EDITOR", oldEditor)
		_ = os.Setenv("VISUAL", oldVisual)
	}()

	t.Run("EDITOR wins over VISUAL", func(t *testing.T) {
		_ = os.Unsetenv("EDITOR")
		_ = os.Unsetenv("VISUAL")
		_ = os.Setenv("EDITOR", "code --wait")
		_ = os.Setenv("VISUAL", "vim")
		v, ok := resolveEditor()
		if !ok {
			t.Fatalf("expected ok")
		}
		if v != "code --wait" {
			t.Fatalf("got %q", v)
		}
	})

	t.Run("EDITOR empty falls to VISUAL", func(t *testing.T) {
		_ = os.Unsetenv("EDITOR")
		_ = os.Setenv("EDITOR", "  ")
		_ = os.Setenv("VISUAL", "nano")
		v, ok := resolveEditor()
		if !ok {
			t.Fatalf("expected ok")
		}
		if v != "nano" {
			t.Fatalf("got %q", v)
		}
	})

	t.Run("both empty not set", func(t *testing.T) {
		_ = os.Unsetenv("EDITOR")
		_ = os.Unsetenv("VISUAL")
		_ = os.Setenv("EDITOR", "")
		_ = os.Setenv("VISUAL", "")
		_, ok := resolveEditor()
		if ok {
			t.Fatalf("expected not ok")
		}
	})
}

// TestSplitArgsPreservesRawBytes guards the regression where splitArgs re-encoded
// every byte >= 0x80 as a rune, doubling it into mojibake and corrupting the
// EDITOR command handed to exec.Command. It is a separate table so the byte
// preservation contract cannot be lost when the main table is edited.
func TestSplitArgsPreservesRawBytes(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "quoted arg with U+00F1",
			input: `--open "Informe ñ.txt"`,
			want:  []string{"--open", "Informe ñ.txt"},
		},
		{
			name:  "unquoted program path with U+00E9",
			input: `C:\Users\José\edit.exe file.txt`,
			want:  []string{"C:\\Users\\José\\edit.exe", "file.txt"},
		},
		{
			name:  "4-byte UTF-8 sequence survives",
			input: `prog "€ file.txt"`,
			want:  []string{"prog", "€ file.txt"},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := splitArgs(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len(got)=%d want %d: got=%v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d]=%x (%q) want %x (%q)", i, got[i], got[i], tt.want[i], tt.want[i])
				}
			}
			// The bytes must be identical, not just the decoded runes: compare the
			// raw byte slices so a re-encoding cannot slip through as equal text.
			for i := range got {
				if !bytesEqual(got[i], tt.want[i]) {
					t.Fatalf("got[%d] bytes % x differ from want % x", i, []byte(got[i]), []byte(tt.want[i]))
				}
			}
		})
	}
}

func bytesEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSplitArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple",
			input: "foo bar baz",
			want:  []string{"foo", "bar", "baz"},
		},
		{
			name:  "double quotes",
			input: `--open "My Notes.txt"`,
			want:  []string{"--open", "My Notes.txt"},
		},
		{
			name:  "single quotes",
			input: `--open 'My Notes.txt'`,
			want:  []string{"--open", "My Notes.txt"},
		},
		{
			name:  "mixed quotes nested",
			input: `--arg="it's fine"`,
			want:  []string{"--arg=it's fine"},
		},
		{
			name:  "quoted empty string",
			input: `"" --flag`,
			want:  []string{"", "--flag"},
		},
		{
			name:  "unclosed quote treated as single token",
			input: `"unclosed arg`,
			want:  []string{`"unclosed arg`},
		},
		{
			name:  "empty input",
			input: "",
			want:  []string{},
		},
		{
			name:  "only whitespace",
			input: "   \t  ",
			want:  []string{},
		},
		{
			name:  "non-ASCII bytes preserved byte-for-byte in a quoted arg",
			input: `--open "Informe ñ.txt"`,
			want:  []string{"--open", "Informe ñ.txt"},
		},
		{
			name:  "non-ASCII bytes preserved byte-for-byte in an unquoted program",
			input: `C:\Users\José\edit.exe file.txt`,
			want:  []string{"C:\\Users\\José\\edit.exe", "file.txt"},
		},
		{
			name:  "empty quoted string embedded after content keeps order",
			input: `--flag="" C:\x.txt`,
			want:  []string{"--flag=", "", "C:\\x.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitArgs(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len(got)=%d want %d: got=%v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d]=%q want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
