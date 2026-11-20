package meta

import (
	"strings"

	"pdf-watcher/internal/extract"
)

type Resolver struct {
	Crossref *CrossrefClient
	PubMed   *PubMedClient
	AI       *OpenAIClient
	AIOn     bool // only to find DOI/PMID; never writes AI bibliographic fields
}

type ResolveInput struct {
	Path string
	Text string
}

// ResolveFull:
//  1. regex / path / (optional AI) → IDs only
//  2. Crossref + PubMed as sole bibliographic sources
//  3. if both sides return a hit, DOI (or long title) must agree — else refuse structured meta
//  4. filename fallback
func (r *Resolver) ResolveFull(in ResolveInput) Record {
	front := extract.FrontText(in.Text, 3000)
	ids := collectIDs(in.Path, in.Text, front)

	if rec, ok, warn := r.lookupVerified(ids); ok {
		return rec
	} else if warn != "" {
		fb := FilenameRecord(in.Path)
		fb.DOI, fb.PMID, fb.PMCID = ids.DOI, ids.PMID, ids.PMCID
		fb.Warn = warn
		// still try AI ids if enabled
	}

	if r.AIOn && in.Text != "" && r.AI != nil && r.AI.APIKey != "" {
		if aiIDs, err := r.AI.ExtractIDs(front); err == nil {
			ids = MergeIDs(ids, aiIDs)
			if rec, ok, warn := r.lookupVerified(ids); ok {
				if strings.HasPrefix(rec.Source, "crossref") {
					rec.Source = rec.Source + "+ai-id"
				} else if strings.HasPrefix(rec.Source, "pubmed") {
					rec.Source = rec.Source + "+ai-id"
				}
				return rec
			} else if warn != "" {
				fb := FilenameRecord(in.Path)
				fb.DOI, fb.PMID, fb.PMCID = ids.DOI, ids.PMID, ids.PMCID
				fb.Warn = warn
				return fb
			}
		}
	}

	// re-check after first-pass warn path without AI
	if rec, ok, warn := r.lookupVerified(ids); ok {
		return rec
	} else if warn != "" {
		fb := FilenameRecord(in.Path)
		fb.DOI, fb.PMID, fb.PMCID = ids.DOI, ids.PMID, ids.PMCID
		fb.Warn = warn
		return fb
	}

	rec := FilenameRecord(in.Path)
	rec.DOI = ids.DOI
	rec.PMID = ids.PMID
	rec.PMCID = ids.PMCID
	return rec
}

func collectIDs(path, text, front string) IDs {
	ids := ExtractIDs(front)
	if text != front {
		ids = MergeIDs(ids, ExtractIDs(text))
	}
	return MergeIDs(ids, ExtractIDsFromPath(path))
}

// lookupVerified queries Crossref and/or PubMed. Dual hits must agree.
func (r *Resolver) lookupVerified(ids IDs) (rec Record, ok bool, warn string) {
	var fromCR, fromPM *Record

	if ids.DOI != "" && r.Crossref != nil {
		if hit, err := r.Crossref.LookupDOI(ids.DOI); err == nil {
			hit.DOI = CleanDOI(ids.DOI)
			fromCR = &hit
		}
	}

	if ids.PMID != "" && r.PubMed != nil {
		if hit, err := r.PubMed.ByPMID(ids.PMID); err == nil {
			fromPM = &hit
		}
	}
	if fromPM == nil && ids.PMCID != "" && r.PubMed != nil {
		if hit, err := r.PubMed.ByPMCID(ids.PMCID); err == nil {
			fromPM = &hit
		}
	}
	if fromPM == nil && ids.DOI != "" && r.PubMed != nil {
		if hit, err := r.PubMed.ByDOI(ids.DOI); err == nil {
			if hit.DOI == "" {
				hit.DOI = CleanDOI(ids.DOI)
			}
			fromPM = &hit
		}
	}

	switch {
	case fromCR != nil && fromPM != nil:
		if !RecordsAgree(*fromCR, *fromPM) {
			return Record{}, false, "crossref/pubmed mismatch; refused structured meta"
		}
		out := mergeVerified(*fromCR, *fromPM)
		out.Source = "crossref+pubmed"
		if ids.PMID != "" {
			out.PMID = ids.PMID
		}
		if ids.PMCID != "" && out.PMCID == "" {
			out.PMCID = ids.PMCID
		}
		return out, true, ""
	case fromCR != nil:
		out := *fromCR
		out.Source = "crossref"
		if ids.PMID != "" {
			out.PMID = ids.PMID
		}
		return out, true, ""
	case fromPM != nil:
		out := *fromPM
		out.Source = "pubmed"
		return out, true, ""
	default:
		return Record{}, false, ""
	}
}

// RecordsAgree: same DOI if both have one; else normalized titles (long enough).
func RecordsAgree(a, b Record) bool {
	da, db := CleanDOI(a.DOI), CleanDOI(b.DOI)
	if da != "" && db != "" {
		return strings.EqualFold(da, db)
	}
	ta, tb := NormalizeTitle(a.Title), NormalizeTitle(b.Title)
	if len([]rune(ta)) >= MinTitleNormLen && ta == tb {
		return true
	}
	return false
}

func mergeVerified(cr, pm Record) Record {
	out := cr // Crossref fields preferred when both agree
	if out.DOI == "" {
		out.DOI = pm.DOI
	}
	if out.PMID == "" {
		out.PMID = pm.PMID
	}
	if out.PMCID == "" {
		out.PMCID = pm.PMCID
	}
	if out.AbstractNote == "" {
		out.AbstractNote = pm.AbstractNote
	}
	if out.Date == "" {
		out.Date = pm.Date
	}
	if out.PublicationTitle == "" {
		out.PublicationTitle = pm.PublicationTitle
	}
	if len(out.Creators) == 0 {
		out.Creators = pm.Creators
	}
	return out
}

func ExtractFrontIDs(text string) IDs {
	return ExtractIDs(extract.FrontText(text, 3000))
}
