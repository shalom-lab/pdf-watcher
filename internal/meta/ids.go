package meta

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	doiRe   = regexp.MustCompile(`(?i)(?:doi[:\s]*)?(?:https?://(?:dx\.)?doi\.org/)?(10\.\d{4,9}/[-._;()/:A-Z0-9]+)`)
	pmidRe  = regexp.MustCompile(`(?i)\bPMID[:\s]*([0-9]{5,9})\b`)
	pmcidRe = regexp.MustCompile(`(?i)\bPMC([0-9]{5,})\b`)
	// Nature Portfolio style article id used as filename: s43856-026-01611-9
	natureFileRe = regexp.MustCompile(`(?i)^s\d{4,}-\d{2,}-\d+-\d+[a-z]?$`)
	// Loose "10.xxxx/..." then cut with CleanDOI — also allow spaces from PDF extract.
	doiLooseRe = regexp.MustCompile(`(?i)10\.\d{4,9}/\S+`)
)

type IDs struct {
	DOI   string
	PMID  string
	PMCID string
}

// ExtractIDs scans front text; DOI uses first match only, then cleans PDF-glue junk.
func ExtractIDs(text string) IDs {
	var ids IDs
	if m := doiRe.FindStringSubmatch(text); len(m) > 1 {
		ids.DOI = CleanDOI(m[1])
	} else if m := doiLooseRe.FindString(text); m != "" {
		ids.DOI = CleanDOI(m)
	}
	if m := pmidRe.FindStringSubmatch(text); len(m) > 1 {
		ids.PMID = m[1]
	}
	if m := pmcidRe.FindStringSubmatch(text); len(m) > 1 {
		ids.PMCID = "PMC" + m[1]
	}
	return ids
}

// ExtractIDsFromPath recovers DOI hints from filename when PDF text is glued/missing.
func ExtractIDsFromPath(path string) IDs {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = StripFilenameNoise(strings.TrimSpace(base))
	base = stripPublisherFileDecorations(base)

	if natureFileRe.MatchString(base) {
		return IDs{DOI: CleanDOI("10.1038/" + base)}
	}
	// common: 10.1038-s41467-026-76037-x or 10_1038_s41467_...
	dashed := strings.ReplaceAll(base, "-", "/")
	dashed = strings.ReplaceAll(dashed, "_", "/")
	for _, cand := range []string{base, dashed, strings.ReplaceAll(base, "_", "/")} {
		if m := doiRe.FindStringSubmatch(cand); len(m) > 1 {
			return IDs{DOI: CleanDOI(m[1])}
		}
		if m := doiLooseRe.FindString(cand); m != "" {
			if d := CleanDOI(m); d != "" {
				return IDs{DOI: d}
			}
		}
	}
	return IDs{}
}

func stripPublisherFileDecorations(base string) string {
	// Elsevier ScienceDirect: 1-s2.0-S2542...-main / -mmc1
	base = regexp.MustCompile(`(?i)^1-s2\.0-`).ReplaceAllString(base, "")
	return strings.TrimSpace(base)
}

// MergeIDs prefers primary non-empty fields.
func MergeIDs(primary, fallback IDs) IDs {
	if primary.DOI == "" {
		primary.DOI = fallback.DOI
	}
	if primary.PMID == "" {
		primary.PMID = fallback.PMID
	}
	if primary.PMCID == "" {
		primary.PMCID = fallback.PMCID
	}
	return primary
}

// CleanDOI fixes glued PDF extraction and over-greedy regex captures.
// Examples:
//
//	10.1038/s43856-026-01611-9NetworkedSIRS... → 10.1038/s43856-026-01611-9
//	10.1038/s41467-026-76037-xTherolesof...   → 10.1038/s41467-026-76037-x
//	10.1016/j.lanplh.2026.101488Abstract...   → 10.1016/j.lanplh.2026.101488
func CleanDOI(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\u00a0", "")
	lower := strings.ToLower(s)
	for _, pfx := range []string{"doi:", "https://doi.org/", "http://doi.org/", "https://dx.doi.org/", "http://dx.doi.org/"} {
		if strings.HasPrefix(lower, pfx) {
			s = strings.TrimSpace(s[len(pfx):])
			lower = strings.ToLower(s)
			break
		}
	}
	s = strings.TrimRight(s, ".;,)]}>\"'")

	// Cut at CJK / fullwidth punctuation (title in Chinese glued on).
	runes := []rune(s)
	for i, r := range runes {
		if isCJK(r) {
			s = string(runes[:i])
			runes = []rune(s)
			break
		}
	}

	for i := 0; i < len(runes)-2; i++ {
		cur, next, next2 := runes[i], runes[i+1], runes[i+2]
		// 9Networked / )Abstract
		if (unicode.IsDigit(cur) || cur == ')' || cur == ']') &&
			unicode.IsUpper(next) && unicode.IsLower(next2) {
			return string(runes[:i+1])
		}
		// xTheroles / nAbstract (letter ending DOI + TitleCase)
		if unicode.IsLower(cur) && unicode.IsUpper(next) && unicode.IsLower(next2) {
			return string(runes[:i+1])
		}
		// 8ABSTRACT (digit + ALLCAPS word of length>=4) — keep short journal codes
		if unicode.IsDigit(cur) && unicode.IsUpper(next) && unicode.IsUpper(next2) {
			j := i + 1
			for j < len(runes) && unicode.IsUpper(runes[j]) {
				j++
			}
			if j-i-1 >= 4 {
				return string(runes[:i+1])
			}
		}
	}

	s = string(runes)
	// Must still look like a DOI.
	if !strings.HasPrefix(strings.ToLower(s), "10.") || !strings.Contains(s, "/") {
		return ""
	}
	return s
}

func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF:
		return true
	case r >= 0x3400 && r <= 0x4DBF:
		return true
	case r >= 0xF900 && r <= 0xFAFF:
		return true
	case r >= 0x3000 && r <= 0x303F:
		return true
	default:
		return false
	}
}
