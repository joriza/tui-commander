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
