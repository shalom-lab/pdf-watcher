package zotero

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const RootCollectionName = "pdf-watcher"

type Collection struct {
	Key    string
	Name   string
	Parent string
}

func (c *Client) ListCollectionsParsed() ([]Collection, error) {
	raw, err := c.ListCollections()
	if err != nil {
		return nil, err
	}
	var items []struct {
		Key  string `json:"key"`
		Data struct {
			Name             string `json:"name"`
			ParentCollection any    `json:"parentCollection"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	out := make([]Collection, 0, len(items))
	for _, it := range items {
		parent := ""
		if s, ok := it.Data.ParentCollection.(string); ok {
			parent = s
		}
		out = append(out, Collection{Key: it.Key, Name: it.Data.Name, Parent: parent})
	}
	return out, nil
}

func (c *Client) CreateCollection(name, parentKey string) (string, error) {
	if err := c.EnsureAuth(); err != nil {
		return "", err
	}
	item := map[string]any{"name": name}
	if parentKey == "" {
		item["parentCollection"] = false
	} else {
		item["parentCollection"] = parentKey
	}
	raw, _ := json.Marshal([]map[string]any{item})
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/users/0/collections", bytes.NewReader(raw))
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
	if resp.StatusCode >= 400 {
		return "", &WriteError{Status: resp.StatusCode, Body: string(body)}
	}
	var wr struct {
		Successful map[string]struct {
			Key string `json:"key"`
		} `json:"successful"`
	}
	if err := json.Unmarshal(body, &wr); err != nil {
		return "", err
	}
	for _, v := range wr.Successful {
		if v.Key != "" {
			return v.Key, nil
		}
	}
	return "", fmt.Errorf("create collection failed: %s", string(body))
}

func (c *Client) RenameCollection(key, name string) error {
	ver, err := c.collectionVersion(key)
	if err != nil {
		return err
	}
	if err := c.EnsureAuth(); err != nil {
		return err
	}
	patch, _ := json.Marshal(map[string]any{"name": name})
	req, err := http.NewRequest(http.MethodPatch, c.BaseURL+"/users/0/collections/"+key, bytes.NewReader(patch))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Unmodified-Since-Version", fmt.Sprintf("%d", ver))
	resp, err := c.doWrite(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return &WriteError{Status: resp.StatusCode, Body: string(body)}
	}
	return nil
}

func (c *Client) collectionVersion(key string) (int, error) {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/users/0/collections/"+key, nil)
	if err != nil {
		return 0, err
	}
	c.setCommonHeaders(req, false)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return 0, &WriteError{Status: resp.StatusCode, Body: string(body)}
	}
	var wrap struct {
		Data struct {
			Version int `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return 0, err
	}
	return wrap.Data.Version, nil
}

// EnsureChildCollection ensures top-level pdf-watcher / leafName and returns child key.
func (c *Client) EnsureChildCollection(leafName string) (string, error) {
	if leafName == "" {
		return "", fmt.Errorf("empty collection name")
	}
	cacheKey := RootCollectionName + "/" + leafName
	c.mu.Lock()
	if c.collCache == nil {
		c.collCache = map[string]string{}
	}
	if k, ok := c.collCache[cacheKey]; ok {
		c.mu.Unlock()
		return k, nil
	}
	c.mu.Unlock()

	cols, err := c.ListCollectionsParsed()
	if err != nil {
		return "", err
	}
	rootKey := ""
	for _, col := range cols {
		if col.Parent == "" && col.Name == RootCollectionName {
			rootKey = col.Key
			break
		}
	}
	if rootKey == "" {
		rootKey, err = c.CreateCollection(RootCollectionName, "")
		if err != nil {
			return "", err
		}
		cols, _ = c.ListCollectionsParsed()
		for _, col := range cols {
			if col.Parent == "" && col.Name == RootCollectionName {
				rootKey = col.Key
				break
			}
		}
	}
	childKey := ""
	for _, col := range cols {
		if col.Parent == rootKey && col.Name == leafName {
			childKey = col.Key
			break
		}
	}
	if childKey == "" {
		childKey, err = c.CreateCollection(leafName, rootKey)
		if err != nil {
			return "", err
		}
	}
	c.mu.Lock()
	c.collCache[cacheKey] = childKey
	c.mu.Unlock()
	return childKey, nil
}

func (c *Client) AddItemToCollection(itemKey, collectionKey string) error {
	if collectionKey == "" || itemKey == "" {
		return nil
	}
	if err := c.EnsureAuth(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/users/0/items/"+itemKey, nil)
	if err != nil {
		return err
	}
	c.setCommonHeaders(req, false)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return &WriteError{Status: resp.StatusCode, Body: string(body)}
	}
	var item struct {
		Data struct {
			Version     int      `json:"version"`
			Collections []string `json:"collections"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &item); err != nil {
		return err
	}
	for _, k := range item.Data.Collections {
		if k == collectionKey {
			return nil
		}
	}
	cols := append(append([]string{}, item.Data.Collections...), collectionKey)
	patch, _ := json.Marshal(map[string]any{"collections": cols})
	preq, err := http.NewRequest(http.MethodPatch, c.BaseURL+"/users/0/items/"+itemKey, bytes.NewReader(patch))
	if err != nil {
		return err
	}
	preq.Header.Set("Content-Type", "application/json")
	preq.Header.Set("If-Unmodified-Since-Version", fmt.Sprintf("%d", item.Data.Version))
	presp, err := c.doWrite(preq)
	if err != nil {
		return err
	}
	defer presp.Body.Close()
	pb, _ := io.ReadAll(presp.Body)
	if presp.StatusCode >= 400 {
		return &WriteError{Status: presp.StatusCode, Body: string(pb)}
	}
	return nil
}
