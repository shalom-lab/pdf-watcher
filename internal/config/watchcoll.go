package config

import (
	"path/filepath"
	"strings"
)

// CollRename is a Zotero collection that should be renamed after local reconcile.
type CollRename struct {
	CollectionID string
	NewName      string
}

// ReconcileWatchCollections assigns collectionName for each watch dir we manage.
// Rule: unique folder basename → "test"; basename clash among enabled dirs → "A/test".
// Returns renames needed when an existing collectionId's display name changed.
func ReconcileWatchCollections(dirs []WatchDir) (out []WatchDir, renames []CollRename) {
	out = make([]WatchDir, len(dirs))
	copy(out, dirs)

	type ref struct {
		i    int
		abs  string
		base string
	}
	var enabled []ref
	for i := range out {
		if strings.TrimSpace(out[i].Path) == "" {
			continue
		}
		abs := absPath(out[i].Path)
		base := folderBase(abs)
		if out[i].Enable {
			enabled = append(enabled, ref{i: i, abs: abs, base: base})
		}
	}

	clash := map[string]int{}
	for _, e := range enabled {
		k := strings.ToLower(e.base)
		clash[k]++
	}

	for _, e := range enabled {
		want := e.base
		if clash[strings.ToLower(e.base)] > 1 {
			want = disambiguated(e.abs, e.base)
		}
		oldName := out[e.i].CollectionName
		oldID := out[e.i].CollectionID
		out[e.i].CollectionName = want
		if oldID != "" && oldName != "" && oldName != want {
			renames = append(renames, CollRename{CollectionID: oldID, NewName: want})
		}
	}

	// disabled / empty: keep name if set, else fill basename for display consistency
	for i := range out {
		if strings.TrimSpace(out[i].Path) == "" {
			continue
		}
		if out[i].CollectionName != "" {
			continue
		}
		out[i].CollectionName = folderBase(absPath(out[i].Path))
	}
	return out, renames
}

func absPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(abs)
}

func folderBase(abs string) string {
	base := strings.TrimSpace(filepath.Base(abs))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "inbox"
	}
	return base
}

func disambiguated(abs, base string) string {
	dir := filepath.Dir(abs)
	parent := filepath.Base(dir)
	vol := strings.TrimSuffix(filepath.VolumeName(abs), ":")
	if dir == abs || parent == "." || parent == "" || parent == string(filepath.Separator) ||
		strings.HasSuffix(parent, ":") {
		if vol != "" {
			return vol + "/" + base
		}
		return base
	}
	return parent + "/" + base
}

// FindWatchDir matches a filesystem path to a configured watch entry (by abs path).
func FindWatchDir(dirs []WatchDir, watchDir string) (WatchDir, bool) {
	want := strings.ToLower(absPath(watchDir))
	for _, d := range dirs {
		if d.Path == "" {
			continue
		}
		if strings.ToLower(absPath(d.Path)) == want {
			return d, true
		}
	}
	return WatchDir{}, false
}
