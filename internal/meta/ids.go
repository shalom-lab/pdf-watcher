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
	natureFileRe = regexp.MustCompile(`(?i)^s\d{4,}-\d+-\d+-\d+$`)
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
	base = strings.TrimSpace(base)
	if natureFileRe.MatchString(base) {
		return IDs{DOI: CleanDOI("10.1038/" + base)}
	}
	if m := doiRe.FindStringSubmatch(base); len(m) > 1 {
		return IDs{DOI: CleanDOI(m[1])}
	}
	// filename encodes doi with underscores
	alt := strings.ReplaceAll(base, "_", "/")
	if m := doiRe.FindStringSubmatch(alt); len(m) > 1 {
		return IDs{DOI: CleanDOI(m[1])}
	}
	return IDs{}
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

// CleanDOI fixes glued PDF extraction, e.g.
// 10.1038/s43856-026-01611-9NetworkedSIRS... → 10.1038/s43856-026-01611-9
func CleanDOI(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "doi:") {
		s = strings.TrimSpace(s[4:])
	}
	s = strings.TrimRight(s, ".;,)]}>")

	runes := []rune(s)
	for i := 0; i < len(runes)-2; i++ {
		cur, next, next2 := runes[i], runes[i+1], runes[i+2]
		// digit/)/] then TitleCase word glued on: 9Networked
		if (unicode.IsDigit(cur) || cur == ')' || cur == ']') &&
			unicode.IsUpper(next) && unicode.IsLower(next2) {
			return string(runes[:i+1])
		}
	}
	return s
}
