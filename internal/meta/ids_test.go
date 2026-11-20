package meta

import "testing"

func TestExtractIDs(t *testing.T) {
	text := "Something DOI: 10.1000/xyz123 and PMID: 12345678 and PMC9876543 end"
	ids := ExtractIDs(text)
	if ids.DOI != "10.1000/xyz123" {
		t.Fatalf("doi %q", ids.DOI)
	}
	if ids.PMID != "12345678" {
		t.Fatalf("pmid %q", ids.PMID)
	}
	if ids.PMCID != "PMC9876543" {
		t.Fatalf("pmcid %q", ids.PMCID)
	}
}

func TestCleanDOIGluedTitle(t *testing.T) {
	got := CleanDOI("10.1038/s43856-026-01611-9NetworkedSIRSmodelwithKalman")
	if got != "10.1038/s43856-026-01611-9" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractIDsFromPathNature(t *testing.T) {
	ids := ExtractIDsFromPath(`C:\x\s43856-026-01611-9.pdf`)
	if ids.DOI != "10.1038/s43856-026-01611-9" {
		t.Fatalf("got %q", ids.DOI)
	}
}

func TestNormalizeTitle(t *testing.T) {
	if NormalizeTitle("  Hello   World ") != "hello world" {
		t.Fatal(NormalizeTitle("  Hello   World "))
	}
	if CanMatchByTitle("short", "crossref") {
		t.Fatal("short title should not match")
	}
	if CanMatchByTitle("a sufficiently long paper title here", "filename") {
		t.Fatal("filename source should not match")
	}
}
