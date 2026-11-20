package filter

import (
	"path/filepath"
	"strings"
)

var tempExts = map[string]struct{}{
	".tmp": {}, ".part": {}, ".crdownload": {}, ".download": {},
	".ds_store": {}, ".swp": {}, ".lock": {}, ".$$$": {},
}

var allowedExts = map[string]struct{}{
	".pdf": {}, ".docx": {},
}

var blacklistDirs = map[string]struct{}{
	".nutstore": {}, "$recycle.bin": {}, "system volume information": {},
}

func ShouldProcess(path string, extraIgnore []string) bool {
	if path == "" {
		return false
	}
	base := filepath.Base(path)
	if base == "" || base == "." || base == ".." {
		return false
	}
	if strings.HasPrefix(base, ".") {
		return false
	}
	if strings.HasPrefix(base, "#") || strings.HasPrefix(base, "~$") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(base))
	if _, ok := tempExts[ext]; ok {
		return false
	}
	if _, ok := allowedExts[ext]; !ok {
		return false
	}
	parts := splitPath(path)
	for _, p := range parts {
		pl := strings.ToLower(p)
		if strings.HasPrefix(p, ".") {
			return false
		}
		if _, ok := blacklistDirs[pl]; ok {
			return false
		}
		if strings.Contains(p, "(冲突)") || strings.Contains(p, "冲突副本") ||
			strings.Contains(strings.ToLower(p), "nsconflict") {
			return false
		}
		for _, ig := range extraIgnore {
			if ig != "" && strings.EqualFold(p, ig) {
				return false
			}
		}
	}
	return true
}

func IsAllowedExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := allowedExts[ext]
	return ok
}

func splitPath(path string) []string {
	path = strings.ReplaceAll(path, "/", string(filepath.Separator))
	var out []string
	for _, p := range strings.Split(path, string(filepath.Separator)) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
