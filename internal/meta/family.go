package meta

import (
	"path/filepath"
	"strings"
)

// FileFamilyKey groups a main PDF with its SI/mmc/附件 siblings.
// Empty means "no reliable family" — do not merge on this alone.
func FileFamilyKey(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}

	// Elsevier ScienceDirect PII: 1-s2.0-S2542519626000616-mmc1
	if m := elsevierPIIRe.FindStringSubmatch(base); len(m) > 1 {
		return "pii:" + strings.ToUpper(m[1])
	}
	stripped := stripPublisherFileDecorations(base)
	if m := elsevierPIIRe.FindStringSubmatch(stripped); len(m) > 1 {
		return "pii:" + strings.ToUpper(m[1])
	}

	// Nature article code shared by main + supplements when named alike
	stem := StripFilenameNoise(base)
	stem = stripPublisherFileDecorations(stem)
	if natureFileRe.MatchString(stem) {
		return "nature:" + strings.ToLower(stem)
	}

	// Long cleaned filename stem (title-like names + 附件/副本 stripped)
	n := NormalizeTitle(stem)
	if len([]rune(n)) >= MinTitleNormLen {
		return "stem:" + n
	}
	return ""
}

// IndexFamilyKey is stored in SQLite for sibling attach lookups.
func IndexFamilyKey(path string, title string) string {
	if k := FileFamilyKey(path); k != "" {
		return k
	}
	if CanMatchByTitle(title, "") {
		return "stem:" + NormalizeTitle(title)
	}
	return ""
}
