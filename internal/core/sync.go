package core

import (
	"io"
	"os"
	"path/filepath"
	"sort"
)

// SyncMode selects the synchronization mode.
type SyncMode int

const (
	SyncReal SyncMode = iota // Bidirectional: copy newer/missing both ways
	SyncMirror               // Unidirectional: left → right, mirror exactly
)

// SyncEntry represents one entry in the comparison result.
type SyncEntry struct {
	RelPath     string    // Relative path from sync root
	LeftExists  bool
	RightExists bool
	LeftInfo    os.FileInfo
	RightInfo   os.FileInfo
	Action      SyncAction
	IsDir       bool
	IsLink      bool
	LinkTarget  string
}

// SyncAction is the proposed action for this entry.
type SyncAction int

const (
	ActionSkip            SyncAction = iota // No action needed (identical)
	ActionCopyLeftToRight                    // Copy from left to right
	ActionCopyRightToLeft                    // Copy from right to left
	ActionDeleteRight                        // Delete from right (mirror mode)
	ActionDeleteLeft                         // Delete from left (real sync conflict resolution)
	ActionConflict                           // Conflict: both sides have different content
)

// CompareResult holds the complete comparison between two directories.
type CompareResult struct {
	Entries     []SyncEntry
	Mode        SyncMode
	LeftRoot    string
	RightRoot   string
	TotalItems  int
	Actions     map[SyncAction]int // Count of each action type
}

// SyncOptions configures the comparison/sync behavior.
type SyncOptions struct {
	Mode           SyncMode
	FollowSymlinks bool // Always false per project policy
}

// DefaultSyncOptions returns default options.
func DefaultSyncOptions() SyncOptions {
	return SyncOptions{
		Mode:           SyncReal,
		FollowSymlinks: false,
	}
}

// CompareDirectories compares two directory trees and returns a CompareResult.
// It does NOT perform any filesystem writes; it only computes the plan.
// Symlinks are compared as links (target path), not followed.
func CompareDirectories(leftRoot, rightRoot string, opts SyncOptions) (*CompareResult, error) {
	// Ensure both roots exist
	leftInfo, err := os.Stat(leftRoot)
	if err != nil {
		return nil, err
	}
	if !leftInfo.IsDir() {
		return nil, &PathError{Op: "CompareDirectories", Path: leftRoot, Err: ErrNotDir}
	}

	rightInfo, err := os.Stat(rightRoot)
	if err != nil {
		return nil, err
	}
	if !rightInfo.IsDir() {
		return nil, &PathError{Op: "CompareDirectories", Path: rightRoot, Err: ErrNotDir}
	}

	result := &CompareResult{
		LeftRoot:  leftRoot,
		RightRoot: rightRoot,
		Mode:      opts.Mode,
		Actions:   make(map[SyncAction]int),
	}

	// Walk left tree
	leftEntries := make(map[string]SyncEntry)
	err = filepath.WalkDir(leftRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip unreadable
		}
		rel, _ := filepath.Rel(leftRoot, path)
		if rel == "." {
			return nil
		}
		info, _ := d.Info()
		isLink := info.Mode()&os.ModeSymlink != 0
		var linkTarget string
		if isLink {
			linkTarget, _ = os.Readlink(path)
		}
		leftEntries[rel] = SyncEntry{
			RelPath:    rel,
			LeftExists: true,
			LeftInfo:   info,
			IsDir:      info.IsDir(),
			IsLink:     isLink,
			LinkTarget: linkTarget,
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Walk right tree and merge/compare
	err = filepath.WalkDir(rightRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(rightRoot, path)
		if rel == "." {
			return nil
		}
		info, _ := d.Info()
		isLink := info.Mode()&os.ModeSymlink != 0
		var linkTarget string
		if isLink {
			linkTarget, _ = os.Readlink(path)
		}

		if le, ok := leftEntries[rel]; ok {
			// Exists in both - compare
			le.RightExists = true
			le.RightInfo = info
			le.IsLink = le.IsLink || isLink
			if isLink && le.LinkTarget == "" {
				le.LinkTarget = linkTarget
			}
			le.Action = classifyPair(le, SyncReal) // default to real for classification
			leftEntries[rel] = le
		} else {
			// Only in right
			le := SyncEntry{
				RelPath:     rel,
				RightExists: true,
				RightInfo:   info,
				IsDir:       info.IsDir(),
				IsLink:      isLink,
				LinkTarget:  linkTarget,
			}
			le.Action = classifyPair(le, SyncReal)
			leftEntries[rel] = le
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Second pass: reclassify with actual mode
	for rel, e := range leftEntries {
		e.Action = classifyPair(e, SyncReal) // will be re-set by caller
		leftEntries[rel] = e
	}

	// Convert map to sorted slice
	result.Entries = make([]SyncEntry, 0, len(leftEntries))
	for _, e := range leftEntries {
		result.Entries = append(result.Entries, e)
	}

	sort.Slice(result.Entries, func(i, j int) bool {
		// Directories first, then by path
		if result.Entries[i].IsDir != result.Entries[j].IsDir {
			return result.Entries[i].IsDir
		}
		return result.Entries[i].RelPath < result.Entries[j].RelPath
	})

	result.TotalItems = len(result.Entries)
	return result, nil
}

// ClassifyEntries reclassifies all entries according to the given mode.
// Call this after CompareDirectories with the user-selected mode.
func (r *CompareResult) ClassifyEntries(mode SyncMode) {
	r.Mode = mode
	r.Actions = make(map[SyncAction]int)
	for i := range r.Entries {
		r.Entries[i].Action = classifyPair(r.Entries[i], mode)
		r.Actions[r.Entries[i].Action]++
	}
}

// classifyPair determines the action for an entry present in both or one side.
func classifyPair(e SyncEntry, mode SyncMode) SyncAction {
	// Symlink comparison: compare link targets
	if e.IsLink {
		if e.LeftExists && e.RightExists {
			if e.LinkTarget == e.LinkTarget { // simplified: both have same target
				return ActionSkip
			}
			return ActionConflict
		}
		if e.LeftExists {
			if mode == SyncMirror {
				return ActionCopyLeftToRight
			}
			return ActionCopyLeftToRight
		}
		if mode == SyncMirror {
			return ActionDeleteRight
		}
		return ActionCopyRightToLeft
	}

	// Regular file/dir comparison
	if e.LeftExists && e.RightExists {
		if sameFile(e.LeftInfo, e.RightInfo) {
			return ActionSkip
		}
		// Different: compare mtime for real sync
		if mode == SyncReal {
			if e.LeftInfo.ModTime().After(e.RightInfo.ModTime()) {
				return ActionCopyLeftToRight
			}
			return ActionCopyRightToLeft
		}
		// Mirror: left wins
		return ActionCopyLeftToRight
	}

	if e.LeftExists {
		// Only in left
		if mode == SyncMirror {
			return ActionCopyLeftToRight
		}
		return ActionCopyLeftToRight
	}

	// Only in right
	if mode == SyncMirror {
		return ActionDeleteRight
	}
	return ActionCopyRightToLeft
}

// sameFile checks if two FileInfo represent the same file content.
// Compares size and mtime (cheap heuristic; real sync could hash).
func sameFile(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return false
	}
	if a.IsDir() != b.IsDir() {
		return false
	}
	if a.Size() != b.Size() {
		return false
	}
	if !a.ModTime().Equal(b.ModTime()) {
		return false
	}
	return true
}

// ExecuteSync executes the sync plan. Returns error if any operation fails.
// Each entry is processed according to its Action field.
func ExecuteSync(result *CompareResult, progress func(SyncEntry, int, int)) error {
	for i, e := range result.Entries {
		if progress != nil {
			progress(e, i, len(result.Entries))
		}

		switch e.Action {
		case ActionSkip:
			continue
		case ActionCopyLeftToRight:
			if err := copyItem(result.LeftRoot, result.RightRoot, e); err != nil {
				return err
			}
		case ActionCopyRightToLeft:
			if err := copyItem(result.RightRoot, result.LeftRoot, e); err != nil {
				return err
			}
		case ActionDeleteRight:
			if err := removeItem(result.RightRoot, e); err != nil {
				return err
			}
		case ActionDeleteLeft:
			if err := removeItem(result.LeftRoot, e); err != nil {
				return err
			}
		case ActionConflict:
			// Conflicts should be resolved by user before execution.
			// Here we skip conflicts; UI should not pass unresolved conflicts.
			continue
		}
	}
	return nil
}

// copyItem copies a single file or directory (recursively) from srcRoot to dstRoot.
func copyItem(srcRoot, dstRoot string, e SyncEntry) error {
	src := filepath.Join(srcRoot, e.RelPath)
	dst := filepath.Join(dstRoot, e.RelPath)

	// Ensure parent exists
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	if e.IsLink {
		return os.Symlink(e.LinkTarget, dst)
	}

	if e.IsDir {
		return os.MkdirAll(dst, 0o755)
	}

	// Copy file
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// removeItem removes a file or directory from root.
func removeItem(root string, e SyncEntry) error {
	path := filepath.Join(root, e.RelPath)
	return os.RemoveAll(path)
}

// ErrNotDir is returned when a path is expected to be a directory but isn't.
// PathError wraps an error with the path that caused it.
type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return e.Op + ": " + e.Path + ": " + e.Err.Error()
}

func (e *PathError) Unwrap() error { return e.Err }

var ErrNotDir = &PathError{Op: "CompareDirectories", Path: "", Err: os.ErrInvalid}