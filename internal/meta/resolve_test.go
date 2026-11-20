package meta

import "testing"

func TestRecordsAgreeDOI(t *testing.T) {
	a := Record{DOI: "10.1038/s43856-026-01611-9", Title: "Networked SIRS model"}
	b := Record{DOI: "10.1038/S43856-026-01611-9", Title: "Other"}
	if !RecordsAgree(a, b) {
		t.Fatal("DOI case-insensitive agree")
	}
	c := Record{DOI: "10.1000/other", Title: "Networked SIRS model with kalman"}
	if RecordsAgree(a, c) {
		t.Fatal("different DOI must disagree")
	}
}

func TestRecordsAgreeTitle(t *testing.T) {
	a := Record{Title: "Networked SIRS model with Kalman filter state estimation"}
	b := Record{Title: "Networked SIRS model with Kalman filter state estimation"}
	if !RecordsAgree(a, b) {
		t.Fatal("title agree")
	}
}
