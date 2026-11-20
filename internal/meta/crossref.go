package meta

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CrossrefClient struct {
	HTTP    *http.Client
	UA      string
}

func NewCrossref() *CrossrefClient {
	return &CrossrefClient{
		HTTP: &http.Client{Timeout: 30 * time.Second},
		UA:   "pdf-watcher/1.0 (mailto:local@localhost)",
	}
}

func (c *CrossrefClient) LookupDOI(doi string) (Record, error) {
	doi = strings.TrimSpace(doi)
	u := "https://api.crossref.org/works/" + url.PathEscape(doi)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return Record{}, err
	}
	req.Header.Set("User-Agent", c.UA)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Record{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return Record{}, fmt.Errorf("crossref %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var raw struct {
		Message struct {
			Title     []string `json:"title"`
			Container []string `json:"container-title"`
			Abstract  string   `json:"abstract"`
			DOI       string   `json:"DOI"`
			Issued    struct {
				DateParts [][]int `json:"date-parts"`
			} `json:"issued"`
			Author []struct {
				Given  string `json:"given"`
				Family string `json:"family"`
				Name   string `json:"name"`
			} `json:"author"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Record{}, err
	}
	m := raw.Message
	rec := Record{Source: "crossref", DOI: m.DOI}
	if len(m.Title) > 0 {
		rec.Title = m.Title[0]
	}
	if len(m.Container) > 0 {
		rec.PublicationTitle = m.Container[0]
	}
	rec.AbstractNote = stripTags(m.Abstract)
	if len(m.Issued.DateParts) > 0 && len(m.Issued.DateParts[0]) > 0 {
		rec.Date = fmt.Sprintf("%d", m.Issued.DateParts[0][0])
		if len(m.Issued.DateParts[0]) > 1 {
			rec.Date = fmt.Sprintf("%d-%02d", m.Issued.DateParts[0][0], m.Issued.DateParts[0][1])
		}
		if len(m.Issued.DateParts[0]) > 2 {
			rec.Date = fmt.Sprintf("%d-%02d-%02d", m.Issued.DateParts[0][0], m.Issued.DateParts[0][1], m.Issued.DateParts[0][2])
		}
	}
	for _, a := range m.Author {
		cr := Creator{CreatorType: "author"}
		if a.Family != "" || a.Given != "" {
			cr.LastName = a.Family
			cr.FirstName = a.Given
		} else if a.Name != "" {
			cr.LastName = a.Name
		}
		if looksChineseName(cr.LastName) && cr.FirstName == "" {
			// keep whole in lastName
		} else if looksChineseName(cr.LastName+cr.FirstName) && cr.FirstName != "" {
			cr.LastName = cr.LastName + cr.FirstName
			cr.FirstName = ""
		}
		rec.Creators = append(rec.Creators, cr)
	}
	if rec.Title == "" {
		return Record{}, fmt.Errorf("crossref empty title")
	}
	return rec, nil
}

func looksChineseName(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == '<' {
			in = true
			continue
		}
		if r == '>' {
			in = false
			b.WriteByte(' ')
			continue
		}
		if !in {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
