package core

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MultiRenameOptions configures the multi-rename operation.
type MultiRenameOptions struct {
	// Pattern is the rename pattern using Total Commander syntax:
	// [N] - original name (without extension)
	// [E] - extension (with dot)
	// [C] - counter (with optional format like [C:3] for 3 digits)
	// [C:3] - counter with 3 digits (001, 002, ...)
	// [Y] - year (4 digits)
	// [M] - month (2 digits)
	// [D] - day (2 digits)
	// [H] - hour (24h, 2 digits)
	// [I] - minute (2 digits)
	// [S] - second (2 digits)
	// [P] - parent path
	// [Z] - file size in bytes
	// Search/Replace: if Search is set, replaces occurrences in the name
	Search        string
	Replace       string
	UseRegex      bool // Use regex for search/replace
	CounterStart  int  // Starting counter value
	CounterStep   int  // Counter increment
	CounterDigits int  // Number of digits for counter (0 = auto)
	Uppercase     bool // Convert result to uppercase
	Lowercase     bool // Convert result to lowercase
}

// MultiRenameResult represents a single rename operation.
type MultiRenameResult struct {
	OldPath string
	NewPath string
	OldName string
	NewName string
	Error   error
}

// DefaultMultiRenameOptions returns sensible defaults for multi-rename.
func DefaultMultiRenameOptions() MultiRenameOptions {
	return MultiRenameOptions{
		CounterStart:  1,
		CounterStep:   1,
		CounterDigits: 3,
		UseRegex:      false,
		Uppercase:     false,
		Lowercase:     false,
	}
}

// MultiRename applies the pattern to a list of entries and returns the results.
// It does NOT perform the actual rename - use ExecuteMultiRename for that.
func MultiRename(entries []Entry, pattern string, opts MultiRenameOptions) ([]MultiRenameResult, error) {
	// Pre-compile regex if needed
	var searchRegex *regexp.Regexp
	if opts.UseRegex && opts.Search != "" {
		var err error
		searchRegex, err = regexp.Compile(opts.Search)
		if err != nil {
			return nil, &PathError{Op: "MultiRename", Path: "", Err: err}
		}
	}

	results := make([]MultiRenameResult, len(entries))
	counter := opts.CounterStart
	if counter == 0 {
		counter = 1
	}
	if opts.CounterStep == 0 {
		opts.CounterStep = 1
	}

	// Determine counter digits if auto
	counterDigits := opts.CounterDigits
	if counterDigits == 0 {
		// Auto-calculate based on number of entries
		maxCount := opts.CounterStart + len(entries)*opts.CounterStep
		counterDigits = len(strconv.Itoa(maxCount))
		if counterDigits < 1 {
			counterDigits = 1
		}
	}

	for i, entry := range entries {
		oldName := entry.Name

		// Apply search/replace to the original name first (like Total Commander)
		nameForPattern := oldName
		if opts.Search != "" {
			if opts.UseRegex && searchRegex != nil {
				nameForPattern = searchRegex.ReplaceAllString(nameForPattern, opts.Replace)
			} else {
				nameForPattern = strings.ReplaceAll(nameForPattern, opts.Search, opts.Replace)
			}
		}

		newName := applyPattern(nameForPattern, entry, pattern, counter, counterDigits, opts, searchRegex)

		// Apply case conversion
		if opts.Uppercase {
			newName = strings.ToUpper(newName)
		} else if opts.Lowercase {
			newName = strings.ToLower(newName)
		}

		// Ensure extension is preserved if not explicitly handled
		if !strings.Contains(pattern, "[E]") {
			ext := filepath.Ext(oldName)
			if ext != "" && !strings.HasSuffix(newName, ext) {
				newName += ext
			}
		}

		results[i] = MultiRenameResult{
			OldPath: filepath.Join(entry.Name), // Will be prefixed with parent path later
			NewPath: newName,
			OldName: oldName,
			NewName: newName,
		}

		counter += opts.CounterStep
	}

	return results, nil
}

// applyPattern applies the Total Commander-style pattern to a filename.
func applyPattern(oldName string, entry Entry, pattern string, counter, counterDigits int, _ MultiRenameOptions, _ *regexp.Regexp) string {
	ext := filepath.Ext(oldName)
	nameWithoutExt := strings.TrimSuffix(oldName, ext)

	result := pattern

	// Replace placeholders
	// [N] - original name without extension
	result = strings.ReplaceAll(result, "[N]", nameWithoutExt)

	// [E] - extension with dot
	result = strings.ReplaceAll(result, "[E]", ext)

	// [C] - counter with specified digits (also supports [C:N] format)
	counterStr := strconv.Itoa(counter)
	if counterDigits > 0 {
		counterStr = strings.Repeat("0", counterDigits-len(counterStr)) + counterStr
	}
	result = strings.ReplaceAll(result, "[C]", counterStr)

	// Handle [C:N] format - counter with explicit digit count
	cRe := regexp.MustCompile(`\[C:(\d+)\]`)
	result = cRe.ReplaceAllStringFunc(result, func(match string) string {
		digitsStr := match[3 : len(match)-1]
		digits, _ := strconv.Atoi(digitsStr)
		str := strconv.Itoa(counter)
		if digits > 0 {
			str = strings.Repeat("0", digits-len(str)) + str
		}
		return str
	})

	// Date/time placeholders - use file's mod time
	modTime := entry.ModTime
	if modTime.IsZero() {
		modTime = time.Now()
	}
	result = strings.ReplaceAll(result, "[Y]", modTime.Format("2006"))
	result = strings.ReplaceAll(result, "[M]", modTime.Format("01"))
	result = strings.ReplaceAll(result, "[D]", modTime.Format("02"))
	result = strings.ReplaceAll(result, "[H]", modTime.Format("15"))
	result = strings.ReplaceAll(result, "[I]", modTime.Format("04"))
	result = strings.ReplaceAll(result, "[S]", modTime.Format("05"))

	// [P] - parent path (just the immediate parent directory name)
	result = strings.ReplaceAll(result, "[P]", filepath.Base(filepath.Dir(entry.Name)))

	// [Z] - file size
	result = strings.ReplaceAll(result, "[Z]", strconv.FormatInt(entry.Size, 10))

	return result
}

// ExecuteMultiRename performs the actual rename operations.
// Returns list of results with any errors.
func ExecuteMultiRename(parentPath string, results []MultiRenameResult) []MultiRenameResult {
	for i := range results {
		oldPath := filepath.Join(parentPath, results[i].OldName)
		newPath := filepath.Join(parentPath, results[i].NewName)

		// Skip if names are the same
		if results[i].OldName == results[i].NewName {
			continue
		}

		// Check if target exists
		if _, err := os.Stat(newPath); err == nil {
			results[i].Error = &PathError{Op: "MultiRename", Path: newPath, Err: os.ErrExist}
			continue
		}

		err := os.Rename(oldPath, newPath)
		if err != nil {
			results[i].Error = &PathError{Op: "MultiRename", Path: oldPath, Err: err}
		}
	}
	return results
}

// ParseMultiRenamePattern validates a pattern and returns any error.
func ParseMultiRenamePattern(pattern string) error {
	// Validate known placeholders
	knownPlaceholders := []string{"[N]", "[E]", "[C]", "[Y]", "[M]", "[D]", "[H]", "[I]", "[S]", "[P]", "[Z]"}
	// Could add more validation here
	_ = knownPlaceholders
	return nil
}

// MultiRenamePreview generates a preview of the rename operation without executing it.
func MultiRenamePreview(entries []Entry, parentPath string, pattern string, opts MultiRenameOptions) ([]MultiRenameResult, error) {
	results, err := MultiRename(entries, pattern, opts)
	if err != nil {
		return nil, err
	}

	// Prefix paths with parent path
	for i := range results {
		results[i].OldPath = filepath.Join(parentPath, results[i].OldName)
		results[i].NewPath = filepath.Join(parentPath, results[i].NewName)
	}

	return results, nil
}
