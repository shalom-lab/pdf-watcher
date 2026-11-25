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

func TestCleanDOIGluedVariants(t *testing.T) {
	cases := []struct{ in, want string }{
		{"10.1038/s43856-026-01611-9NetworkedSIRSmodelwithKalman", "10.1038/s43856-026-01611-9"},
		{"10.1038/s41467-026-76037-xTherolesofpre-seasonimmunity", "10.1038/s41467-026-76037-x"},
		{"10.1016/j.lanplh.2026.101488AbstractBackground", "10.1016/j.lanplh.2026.101488"},
		{"10.1038/s41591-024-01234-5中文标题粘连", "10.1038/s41591-024-01234-5"},
		{"doi:10.1000/xyz123", "10.1000/xyz123"},
		{"https://doi.org/10.1000/xyz123", "10.1000/xyz123"},
	}
	for _, c := range cases {
		if got := CleanDOI(c.in); got != c.want {
			t.Fatalf("CleanDOI(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestExtractIDsFromPathNature(t *testing.T) {
	ids := ExtractIDsFromPath(`C:\x\s43856-026-01611-9.pdf`)
	if ids.DOI != "10.1038/s43856-026-01611-9" {
		t.Fatalf("got %q", ids.DOI)
	}
}

func TestNormalizeTitleAndNoise(t *testing.T) {
	if NormalizeTitle("  Hello   World ") != "hello world" {
		t.Fatal(NormalizeTitle("  Hello   World "))
	}
	if CanMatchByTitle("short", "crossref") {
		t.Fatal("short title should not match")
	}
	long := "The roles of pre-season immunity, age, viral shedding, and community exposures"
	for _, suf := range []string{" 附件 - 副本", " - mmc1", "_SI", " supplement", " (1)", " 补充材料"} {
		if NormalizeTitle(long+suf) != NormalizeTitle(long) {
			t.Fatalf("noise %q: %q vs %q", suf, NormalizeTitle(long+suf), NormalizeTitle(long))
		}
	}
}

func TestTitleFromFilenameNoise(t *testing.T) {
	cases := map[string]string{
		`C:\x\paper title 附件 - 副本.pdf`: "paper title",
		`C:\x\paper-mmc2.pdf`:            "paper",
		`C:\x\study_SI.pdf`:              "study",
	}
	for path, want := range cases {
		if got := TitleFromFilename(path); got != want {
			t.Fatalf("%s: got %q want %q", path, got, want)
		}
	}
}

func TestFileFamilyKey(t *testing.T) {
	a := FileFamilyKey(`C:\x\1-s2.0-S2542519626000616-mmc1.pdf`)
	b := FileFamilyKey(`C:\x\1-s2.0-S2542519626000616-mmc2.pdf`)
	if a == "" || a != b {
		t.Fatalf("elsevier family %q vs %q", a, b)
	}
	c := FileFamilyKey(`C:\x\The roles of immunity 附件 - 副本.pdf`)
	d := FileFamilyKey(`C:\x\The roles of immunity.pdf`)
	if c == "" || c != d {
		t.Fatalf("stem family %q vs %q", c, d)
	}
}
