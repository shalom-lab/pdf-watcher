package meta

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PubMedClient struct {
	HTTP       *http.Client
	Tool       string
	Email      string
}

func NewPubMed(tool, email string, timeoutSec int) *PubMedClient {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &PubMedClient{
		HTTP:  &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		Tool:  tool,
		Email: email,
	}
}

func (c *PubMedClient) byQuery(db, term string) (Record, error) {
	q := url.Values{}
	q.Set("db", db)
	q.Set("term", term)
	q.Set("retmode", "json")
	q.Set("tool", c.Tool)
	if c.Email != "" {
		q.Set("email", c.Email)
	}
	searchURL := "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi?" + q.Encode()
	resp, err := c.HTTP.Get(searchURL)
	if err != nil {
		return Record{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var sr struct {
		ESearchResult struct {
			IDList []string `json:"idlist"`
		} `json:"esearchresult"`
	}
	if err := json.Unmarshal(body, &sr); err != nil {
		return Record{}, err
	}
	if len(sr.ESearchResult.IDList) == 0 {
		return Record{}, fmt.Errorf("pubmed no hit")
	}
	return c.Summary(sr.ESearchResult.IDList[0])
}

func (c *PubMedClient) ByPMID(pmid string) (Record, error) {
	return c.Summary(pmid)
}

func (c *PubMedClient) ByDOI(doi string) (Record, error) {
	return c.byQuery("pubmed", doi+"[DOI]")
}

func (c *PubMedClient) ByPMCID(pmcid string) (Record, error) {
	pmcid = strings.TrimPrefix(strings.ToUpper(pmcid), "PMC")
	return c.byQuery("pmc", "PMC"+pmcid)
}

func (c *PubMedClient) Summary(pmid string) (Record, error) {
	q := url.Values{}
	q.Set("db", "pubmed")
	q.Set("id", pmid)
	q.Set("retmode", "xml")
	q.Set("tool", c.Tool)
	if c.Email != "" {
		q.Set("email", c.Email)
	}
	u := "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/efetch.fcgi?" + q.Encode()
	resp, err := c.HTTP.Get(u)
	if err != nil {
		return Record{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return Record{}, fmt.Errorf("pubmed %d", resp.StatusCode)
	}
	return parsePubmedXML(body, pmid)
}

func parsePubmedXML(data []byte, pmid string) (Record, error) {
	type Author struct {
		LastName string `xml:"LastName"`
		ForeName string `xml:"ForeName"`
		CollectiveName string `xml:"CollectiveName"`
	}
	type Article struct {
		ArticleTitle string `xml:"ArticleTitle"`
		Abstract     struct {
			AbstractText []string `xml:"AbstractText"`
		} `xml:"Abstract"`
		Journal struct {
			Title string `xml:"Title"`
		} `xml:"Journal"`
		AuthorList struct {
			Author []Author `xml:"Author"`
		} `xml:"AuthorList"`
	}
	type Medline struct {
		PMID    string  `xml:"PMID"`
		Article Article `xml:"Article"`
		ArticleDate struct {
			Year  string `xml:"Year"`
			Month string `xml:"Month"`
			Day   string `xml:"Day"`
		} `xml:"ArticleDate"`
		DateCompleted struct {
			Year string `xml:"Year"`
		} `xml:"DateCompleted"`
		ELocationID []struct {
			EIdType string `xml:"EIdType,attr"`
			Value   string `xml:",chardata"`
		} `xml:"ELocationID"`
	}
	type pubmedArticleSet struct {
		XMLName xml.Name `xml:"PubmedArticleSet"`
		Items   []struct {
			Medline Medline `xml:"MedlineCitation"`
		} `xml:"PubmedArticle"`
	}
	var set pubmedArticleSet
	if err := xml.Unmarshal(data, &set); err != nil {
		return Record{}, err
	}
	if len(set.Items) == 0 {
		return Record{}, fmt.Errorf("pubmed empty")
	}
	m := set.Items[0].Medline
	rec := Record{Source: "pubmed", PMID: pmid}
	rec.Title = strings.TrimSpace(m.Article.ArticleTitle)
	rec.PublicationTitle = m.Article.Journal.Title
	if len(m.Article.Abstract.AbstractText) > 0 {
		rec.AbstractNote = strings.Join(m.Article.Abstract.AbstractText, " ")
	}
	if m.ArticleDate.Year != "" {
		rec.Date = m.ArticleDate.Year
	} else if m.DateCompleted.Year != "" {
		rec.Date = m.DateCompleted.Year
	}
	for _, e := range m.ELocationID {
		if strings.EqualFold(e.EIdType, "doi") {
			rec.DOI = strings.TrimSpace(e.Value)
		}
	}
	for _, a := range m.Article.AuthorList.Author {
		cr := Creator{CreatorType: "author"}
		if a.CollectiveName != "" {
			cr.LastName = a.CollectiveName
		} else {
			cr.LastName = a.LastName
			cr.FirstName = a.ForeName
			if looksChineseName(cr.LastName+cr.FirstName) {
				cr.LastName = strings.TrimSpace(cr.LastName + cr.FirstName)
				cr.FirstName = ""
			}
		}
		rec.Creators = append(rec.Creators, cr)
	}
	if rec.Title == "" {
		return Record{}, fmt.Errorf("pubmed empty title")
	}
	return rec, nil
}
