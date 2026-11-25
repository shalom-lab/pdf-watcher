package meta

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const MinTitleNormLen = 16

var (
	// Trailing noise: 副本/附件/SI/mmc/copy/(1)/final/marked...
	filenameNoiseRe = regexp.MustCompile(`(?i)(\s*[-–_.]?\s*)(` +
		`副本|拷贝|附件|附录|补充材料|补充|图S?\d*|表S?\d*|` +
		`supplement(ary)?(\s*(information|material|data|file|figs?|tables?))?|` +
		`supporting\s*(information|material|info)|` +
		`online\s*resource|additional\s*file|` +
		`esm\d*|mmc\d*|si|s\d+|` +
		`appendix|annex|` +
		`copy(\s*\(\d+\))?|` +
		`\(\d+\)|` +
		`final|marked|highlighted|clean|draft|` +
		`full[\s_-]?text|main([\s_-]?text)?|article` +
		`)\s*$`)
	// Mid-name SI tokens: ...-mmc1-..., _SI_, -Suppl-
	// Only numbered mid tokens (…-mmc1-…); bare "supplement" is handled as trailing noise.
	filenameMidNoiseRe = regexp.MustCompile(`(?i)[-_.]+(mmc\d+|si\d+|suppl\d+|supplement\d+|esm\d+|appendix\d+|附件\d+|附录\d+)[-_.]*`)
	multiSpaceRe       = regexp.MustCompile(`\s+`)
	elsevierPIIRe      = regexp.MustCompile(`(?i)(?:^|[^A-Z0-9])(S\d{10,})(?:[^A-Z0-9]|$)`)
)

func NormalizeTitle(s string) string {
	s = StripFilenameNoise(s)
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

// StripFilenameNoise removes copy/SI/mmc/附件-style decorations so related files share a stem.
func StripFilenameNoise(s string) string {
	s = strings.TrimSpace(s)
	s = filenameMidNoiseRe.ReplaceAllString(s, " ")
	for {
		next := strings.TrimSpace(filenameNoiseRe.ReplaceAllString(s, ""))
		next = strings.TrimRight(next, " -–_.")
		if next == s {
			break
		}
		s = next
	}
	return strings.TrimSpace(multiSpaceRe.ReplaceAllString(s, " "))
}

func TitleFromFilename(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = StripFilenameNoise(name)
	name = stripPublisherFileDecorations(name)
	if name == "" {
		name = strings.TrimSuffix(base, ext)
	}
	return strings.TrimSpace(name)
}

// WeakFilenameTitle: pure numbers / too short / publisher opaque ids — prefer text-derived title.
func WeakFilenameTitle(title string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return true
	}
	if len([]rune(t)) < 8 {
		return true
	}
	digits := 0
	letters := 0
	for _, r := range t {
		switch {
		case unicode.IsDigit(r):
			digits++
		case unicode.IsLetter(r):
			letters++
		}
	}
	if letters == 0 && digits > 0 {
		return true
	}
	// Elsevier PII-like bare ids
	if regexp.MustCompile(`(?i)^S?\d{10,}$`).MatchString(t) {
		return true
	}
	if natureFileRe.MatchString(t) {
		return true
	}
	return false
}

// TitleFromText picks a plausible first-page title when filename is weak (e.g. "263").
func TitleFromText(text string) string {
	lines := strings.Split(text, "\n")
	var candidates []string
	for _, line := range lines {
		line = strings.TrimSpace(multiSpaceRe.ReplaceAllString(line, " "))
		if line == "" {
			continue
		}
		n := len([]rune(line))
		if n < 12 || n > 220 {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "doi") || strings.HasPrefix(lower, "http") ||
			strings.HasPrefix(lower, "www.") || strings.HasPrefix(lower, "supplement") ||
			strings.HasPrefix(lower, "copyright") || strings.HasPrefix(lower, "received") ||
			strings.HasPrefix(lower, "keywords") || strings.HasPrefix(lower, "abstract") {
			continue
		}
		if strings.Count(line, ",") >= 4 && n < 80 {
			continue
		}
		candidates = append(candidates, line)
		if len(candidates) >= 10 {
			break
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	best := candidates[0]
	bestScore := titleScore(best)
	for _, c := range candidates[1:] {
		if s := titleScore(c); s > bestScore {
			best, bestScore = c, s
		}
	}
	if bestScore < 2 {
		return ""
	}
	return best
}

func titleScore(s string) int {
	n := len([]rune(s))
	score := 0
	if n >= 20 && n <= 160 {
		score += 3
	} else if n >= 12 {
		score += 1
	}
	letters := 0
	cjk := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
		}
		if isCJK(r) {
			cjk++
		}
	}
	if letters >= 10 || cjk >= 6 {
		score += 2
	}
	if strings.Contains(s, ".") && n < 40 && cjk == 0 {
		score--
	}
	return score
}

func CanMatchByTitle(title, source string) bool {
	n := NormalizeTitle(title)
	if len([]rune(n)) < MinTitleNormLen {
		return false
	}
	_ = source
	return true
}
