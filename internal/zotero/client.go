package zotero

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"pdf-watcher/internal/meta"
)

type Client struct {
	BaseURL  string
	ServerID string
	APIKey   string
	HTTP     *http.Client

	mu        sync.Mutex
	collCache map[string]string // "pdf-watcher/folder" -> collection key

	OnPersistAuth func(serverID, apiKey string) // remember:true
	OnClearKey    func()
}

type AuthResult struct {
	Key      string `json:"key"`
	Remember bool   `json:"remember"`
	Denied   bool   `json:"denied"`
}

type WriteError struct {
	Status int
	Body   string
	Denied bool
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("zotero write %d: %s", e.Status, e.Body)
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *Client) Probe() (serverID string, err error) {
	// Short timeout: status checks must fail fast when Zotero has quit.
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	} else {
		httpClient = &http.Client{Timeout: 3 * time.Second, Transport: httpClient.Transport}
	}
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/", nil)
	if err != nil {
		return "", err
	}
	c.setCommonHeaders(req, false)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 {
		return "", fmt.Errorf("local API disabled (403)")
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("probe %d: %s", resp.StatusCode, string(b))
	}
	sid := resp.Header.Get("Zotero-Server-ID")
	if sid == "" {
		return "", fmt.Errorf("missing Zotero-Server-ID")
	}
	c.mu.Lock()
	if c.ServerID != "" && c.ServerID != sid {
		c.APIKey = ""
		c.collCache = nil
		if c.OnClearKey != nil {
			c.OnClearKey()
		}
	}
	c.ServerID = sid
	c.mu.Unlock()
	return sid, nil
}

func (c *Client) EnsureAuth() error {
	c.mu.Lock()
	key := c.APIKey
	sid := c.ServerID
	c.mu.Unlock()
	if sid == "" {
		if _, err := c.Probe(); err != nil {
			return err
		}
		c.mu.Lock()
		sid = c.ServerID
		c.mu.Unlock()
	}
	if key != "" {
		return nil
	}
	return c.Authorize()
}

func (c *Client) Authorize() error {
	c.mu.Lock()
	sid := c.ServerID
	c.mu.Unlock()
	if sid == "" {
		return fmt.Errorf("no server id")
	}
	body, _ := json.Marshal(map[string]string{"appName": "pdf-watcher"})
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/local/authorize", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Zotero-Server-ID", sid)
	c.setUA(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 403 {
		var d AuthResult
		_ = json.Unmarshal(raw, &d)
		return &WriteError{Status: 403, Body: string(raw), Denied: true}
	}
	if resp.StatusCode == 429 {
		return &WriteError{Status: 429, Body: string(raw)}
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("authorize %d: %s", resp.StatusCode, string(raw))
	}
	var ar AuthResult
	if err := json.Unmarshal(raw, &ar); err != nil {
		return err
	}
	if !ar.Remember {
		return fmt.Errorf("one-shot key; need Always Allow")
	}
	c.mu.Lock()
	c.APIKey = ar.Key
	c.mu.Unlock()
	if c.OnPersistAuth != nil {
		c.OnPersistAuth(sid, ar.Key)
	}
	return nil
}

func (c *Client) setCommonHeaders(req *http.Request, write bool) {
	c.setUA(req)
	c.mu.Lock()
	sid := c.ServerID
	key := c.APIKey
	c.mu.Unlock()
	if write {
		if sid != "" {
			req.Header.Set("Zotero-Server-ID", sid)
		}
		if key != "" {
			req.Header.Set("Zotero-API-Key", key)
		}
	}
}

func (c *Client) setUA(req *http.Request) {
	req.Header.Set("User-Agent", "pdf-watcher/1.0")
	// Host must be 127.0.0.1/localhost — net/http sets from URL
}

func (c *Client) doWrite(req *http.Request) (*http.Response, error) {
	c.setCommonHeaders(req, true)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 401 {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if err := c.Authorize(); err != nil {
			return nil, err
		}
		// retry once
		c.setCommonHeaders(req, true)
		return c.HTTP.Do(req)
	}
	return resp, nil
}

type itemWriteResp []struct {
	Successful map[string]struct {
		Key string `json:"key"`
	} `json:"successful"`
	Failed map[string]any `json:"failed"`
}

func (c *Client) CreateParent(rec meta.Record, collectionID string) (string, error) {
	if err := c.EnsureAuth(); err != nil {
		return "", err
	}
	// ordered JSON: avoid map key shuffle issues on picky local validators
	type parentItem struct {
		ItemType         string              `json:"itemType"`
		Title            string              `json:"title"`
		Creators         []map[string]string `json:"creators"`
		Date             string              `json:"date"`
		AbstractNote     string              `json:"abstractNote"`
		DOI              string              `json:"DOI"`
		PublicationTitle string              `json:"publicationTitle"`
		Collections      []string            `json:"collections,omitempty"`
	}
	item := parentItem{
		ItemType:         "journalArticle",
		Title:            rec.Title,
		Creators:         creatorsJSON(rec.Creators),
		Date:             rec.Date,
		AbstractNote:     rec.AbstractNote,
		DOI:              rec.DOI,
		PublicationTitle: rec.PublicationTitle,
	}
	if collectionID != "" {
		item.Collections = []string{collectionID}
	}
	return c.postItemsRaw(item)
}

func (c *Client) CreateAttachment(parentKey, filePath string) (string, error) {
	if err := c.EnsureAuth(); err != nil {
		return "", err
	}
	name := filepath.Base(filePath)
	// linkMode MUST appear before filename/path-like fields (Zotero validates in order).
	type attItem struct {
		ItemType    string `json:"itemType"`
		LinkMode    string `json:"linkMode"`
		ParentItem  string `json:"parentItem"`
		Title       string `json:"title"`
		ContentType string `json:"contentType"`
		Filename    string `json:"filename"`
	}
	item := attItem{
		ItemType:    "attachment",
		LinkMode:    "imported_file",
		ParentItem:  parentKey,
		Title:       name,
		ContentType: contentType(filePath),
		Filename:    name,
	}
	return c.postItemsRaw(item)
}

func creatorsJSON(cs []meta.Creator) []map[string]string {
	out := make([]map[string]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, map[string]string{
			"creatorType": c.CreatorType,
			"firstName":   c.FirstName,
			"lastName":    c.LastName,
		})
	}
	return out
}

func contentType(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}

func (c *Client) postItems(items []map[string]any) (string, error) {
	return c.postItemsRaw(items)
}

func (c *Client) postItemsRaw(item any) (string, error) {
	raw, err := json.Marshal([]any{item})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/users/0/items", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.doWrite(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 403 {
		return "", &WriteError{Status: 403, Body: string(body), Denied: true}
	}
	if resp.StatusCode >= 400 {
		return "", &WriteError{Status: resp.StatusCode, Body: string(body)}
	}
	var wr struct {
		Successful map[string]struct {
			Key string `json:"key"`
		} `json:"successful"`
		Failed map[string]any `json:"failed"`
	}
	if err := json.Unmarshal(body, &wr); err != nil {
		return "", err
	}
	for _, v := range wr.Successful {
		if v.Key != "" {
			return v.Key, nil
		}
	}
	return "", fmt.Errorf("no successful item: %s", string(body))
}

func (c *Client) UploadFile(attachmentKey, filePath string) error {
	if err := c.EnsureAuth(); err != nil {
		return err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	sum := md5.Sum(data)
	md5hex := hex.EncodeToString(sum[:])
	fi, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	mtime := fi.ModTime().UnixMilli()
	name := filepath.Base(filePath)

	form := url.Values{}
	form.Set("md5", md5hex)
	form.Set("filename", name)
	form.Set("filesize", strconv.Itoa(len(data)))
	form.Set("mtime", strconv.FormatInt(mtime, 10))

	authURL := fmt.Sprintf("%s/users/0/items/%s/file", c.BaseURL, attachmentKey)
	req, err := http.NewRequest(http.MethodPost, authURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("If-None-Match", "*")
	resp, err := c.doWrite(req)
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode == 404 {
		return &WriteError{Status: 404, Body: string(body)}
	}
	if resp.StatusCode >= 400 {
		return &WriteError{Status: resp.StatusCode, Body: string(body)}
	}
	var auth struct {
		URL         string `json:"url"`
		UploadKey   string `json:"uploadKey"`
		ContentType string `json:"contentType"`
		Prefix      string `json:"prefix"`
		Suffix      string `json:"suffix"`
		Exists      int    `json:"exists"`
	}
	if err := json.Unmarshal(body, &auth); err != nil {
		return err
	}
	if auth.Exists == 1 {
		return nil
	}
	uploadURL := auth.URL
	if strings.HasPrefix(uploadURL, "/") {
		u, _ := url.Parse(c.BaseURL)
		uploadURL = u.Scheme + "://" + u.Host + auth.URL
	}
	// Web/Local API: POST prefix + bytes + suffix; Content-Type from auth (local prefix/suffix usually empty).
	payload := make([]byte, 0, len(auth.Prefix)+len(data)+len(auth.Suffix))
	payload = append(payload, []byte(auth.Prefix)...)
	payload = append(payload, data...)
	payload = append(payload, []byte(auth.Suffix)...)
	upReq, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	ct := auth.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	upReq.Header.Set("Content-Type", ct)
	upReq.ContentLength = int64(len(payload))
	c.setUA(upReq)
	upResp, err := c.HTTP.Do(upReq)
	if err != nil {
		return err
	}
	upBody, _ := io.ReadAll(upResp.Body)
	upResp.Body.Close()
	if upResp.StatusCode != 201 {
		return fmt.Errorf("upload bytes %d: %s", upResp.StatusCode, string(upBody))
	}

	reg := url.Values{}
	reg.Set("upload", auth.UploadKey)
	regReq, err := http.NewRequest(http.MethodPost, authURL, strings.NewReader(reg.Encode()))
	if err != nil {
		return err
	}
	regReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	regReq.Header.Set("If-None-Match", "*")
	regResp, err := c.doWrite(regReq)
	if err != nil {
		return err
	}
	regBody, _ := io.ReadAll(regResp.Body)
	regResp.Body.Close()
	if regResp.StatusCode != 204 {
		return &WriteError{Status: regResp.StatusCode, Body: string(regBody)}
	}
	return nil
}

func (c *Client) DeleteItem(key string) error {
	if key == "" {
		return nil
	}
	if err := c.EnsureAuth(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, c.BaseURL+"/users/0/items/"+key, nil)
	if err != nil {
		return err
	}
	resp, err := c.doWrite(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		return fmt.Errorf("delete item %s: %d", key, resp.StatusCode)
	}
	return nil
}

func (c *Client) ListCollections() ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/users/0/collections", nil)
	if err != nil {
		return nil, err
	}
	c.setCommonHeaders(req, false)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func FileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
