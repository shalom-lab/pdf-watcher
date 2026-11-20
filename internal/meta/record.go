package meta

type Creator struct {
	CreatorType string `json:"creatorType"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
}

type Record struct {
	Title            string    `json:"title"`
	Creators         []Creator `json:"creators"`
	Date             string    `json:"date"`
	AbstractNote     string    `json:"abstractNote"`
	DOI              string    `json:"DOI"`
	PublicationTitle string    `json:"publicationTitle"`
	PMID             string    `json:"-"`
	PMCID            string    `json:"-"`
	Source           string    `json:"-"` // crossref|pubmed|crossref+pubmed|filename (+optional +ai-id)
	Warn             string    `json:"-"` // e.g. dual-source mismatch
}

func FilenameRecord(path string) Record {
	return Record{
		Title:  TitleFromFilename(path),
		Source: "filename",
	}
}
