package core

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FindMode selects the search scope.
type FindMode int

const (
	FindFilesOnly FindMode = iota // Files only
	FindDirs                    // Directories only
	FindBoth                    // Both files and directories
)

// FindOptions configures the search.
type FindOptions struct {
	Mode          FindMode
	NamePattern   string            // filepath.Match pattern (e.g., "*.go")
	MinSize       int64             // Minimum size in bytes (0 = no limit)
	MaxSize       int64             // Maximum size in bytes (0 = no limit)
	ModTimeAfter  *time.Time        // Modified after this time (nil = no limit)
	ModTimeBefore *time.Time        // Modified before this time (nil = no limit)
	Content       string            // Content substring to search (empty = no content search)
	CaseSensitive bool              // For content search
	FollowSymlinks  bool            // Always false per project policy
}

// FindResult holds a single match.
type FindResult struct {
	Path        string    // Absolute path
	RelPath     string    // Relative to search root
	Name        string
	IsDir       bool
	Size        int64
	ModTime     time.Time
	IsLink      bool
	LinkTarget  string
	ContentMatch bool     // True if content search matched
}

// FindResults holds all matches from a search.
type FindResults struct {
	Root      string
	Options   FindOptions
	Matches   []FindResult
	Total     int
	Scanned   int
	Errors    []error
}

// DefaultFindOptions returns sensible defaults.
func DefaultFindOptions() FindOptions {
	return FindOptions{
		Mode:          FindBoth,
		FollowSymlinks: false,
	}
}

// FindFiles searches for files/directories matching the options.
// It walks the tree from root and applies all filters.
func FindFiles(root string, opts FindOptions) (*FindResults, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &PathError{Op: "FindFiles", Path: root, Err: ErrNotDir}
	}

	results := &FindResults{
		Root:    root,
		Options: opts,
		Matches: make([]FindResult, 0),
		Errors:  make([]error, 0),
	}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			results.Errors = append(results.Errors, err)
			return nil // Continue walking
		}

		results.Scanned++

		// Skip root itself
		if path == root {
			return nil
		}

		// Get file info
		info, ierr := d.Info()
		if ierr != nil {
			results.Errors = append(results.Errors, ierr)
			return nil
		}

		// Apply mode filter
		if opts.Mode == FindFilesOnly && info.IsDir() {
			return nil
		}
		if opts.Mode == FindDirs && !info.IsDir() {
			return nil
		}

		// Apply name pattern
		if opts.NamePattern != "" {
			matched, merr := filepath.Match(opts.NamePattern, info.Name())
			if merr != nil || !matched {
				return nil
			}
		}

		// Apply size filters
		if !info.IsDir() {
			if opts.MinSize > 0 && info.Size() < opts.MinSize {
				return nil
			}
			if opts.MaxSize > 0 && info.Size() > opts.MaxSize {
				return nil
			}
		}

		// Apply time filters
		if opts.ModTimeAfter != nil && info.ModTime().Before(*opts.ModTimeAfter) {
			return nil
		}
		if opts.ModTimeBefore != nil && info.ModTime().After(*opts.ModTimeBefore) {
			return nil
		}

		// Symlink info
		isLink := info.Mode()&os.ModeSymlink != 0
		var linkTarget string
		if isLink {
			linkTarget, _ = os.Readlink(path)
		}

		// Content search (only for regular files, not dirs/links)
		contentMatch := false
		if opts.Content != "" && !info.IsDir() && !isLink {
			matched, err := searchContent(path, opts.Content, opts.CaseSensitive)
			if err != nil {
				results.Errors = append(results.Errors, err)
			} else if matched {
				contentMatch = true
			} else if opts.Content != "" {
				// Content search enabled but didn't match
				return nil
			}
		}

		relPath, _ := filepath.Rel(root, path)

		results.Matches = append(results.Matches, FindResult{
			Path:          path,
			RelPath:       relPath,
			Name:          info.Name(),
			IsDir:         info.IsDir(),
			Size:          info.Size(),
			ModTime:       info.ModTime(),
			IsLink:        isLink,
			LinkTarget:    linkTarget,
			ContentMatch:  contentMatch,
		})

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		results.Errors = append(results.Errors, err)
	}

	results.Total = len(results.Matches)
	return results, nil
}

// searchContent searches for a substring in a file.
// Returns true if found, false if not, error if read failed.
func searchContent(path, substr string, caseSensitive bool) (bool, error) {
	// Limit file size for content search to avoid reading huge files
	const maxContentSearchSize = 10 * 1024 * 1024 // 10 MB

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if info.Size() > maxContentSearchSize {
		return false, nil // Skip huge files
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	if caseSensitive {
		return strings.Contains(string(data), substr), nil
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(substr)), nil
}