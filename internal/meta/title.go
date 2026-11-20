package meta

import (
	"path/filepath"
	"strings"
	"unicode"
)

const MinTitleNormLen = 16

func NormalizeTitle(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func TitleFromFilename(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return strings.TrimSpace(name)
}

func CanMatchByTitle(title, source string) bool {
	if source == "filename" {
		return false
	}
	n := NormalizeTitle(title)
	return len([]rune(n)) >= MinTitleNormLen
}
