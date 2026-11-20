package meta

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAI-compatible chat completions client (DeepSeek / OpenAI / 通义 / …).
// Only ask the model for identifiers. Bibliographic fields always come from Crossref/PubMed.
const openaiIDPrompt = `你是文献标识符抽取器。只根据给定文本输出一个 JSON 对象，键仅为 DOI、PMID、PMCID（字符串）。找不到就留空字符串。不要编造。不要输出题录其它字段。不要输出 JSON 以外的任何字符。`

type OpenAIClient struct {
	HTTP   *http.Client
	APIURL string
	APIKey string
	Model  string
}

func NewOpenAI(apiURL, apiKey, model string, timeoutSec int) *OpenAIClient {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &OpenAIClient{
		HTTP:   &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		APIURL: apiURL,
		APIKey: apiKey,
		Model:  model,
	}
}

type idJSON struct {
	DOI   string `json:"DOI"`
	PMID  string `json:"PMID"`
	PMCID string `json:"PMCID"`
}

// ExtractIDs asks the model only for DOI/PMID/PMCID, then cleans them.
func (c *OpenAIClient) ExtractIDs(text string) (IDs, error) {
	if c.APIKey == "" {
		return IDs{}, fmt.Errorf("no api key")
	}
	payload := map[string]any{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": openaiIDPrompt},
			{"role": "user", "content": text},
		},
		"temperature": 0,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, c.APIURL, bytes.NewReader(raw))
	if err != nil {
		return IDs{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return IDs{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return IDs{}, fmt.Errorf("ai %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var wrap struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		return IDs{}, err
	}
	if len(wrap.Choices) == 0 {
		return IDs{}, fmt.Errorf("ai empty choices")
	}
	content := strings.TrimSpace(wrap.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	var rawIDs idJSON
	if err := json.Unmarshal([]byte(content), &rawIDs); err != nil {
		return IDs{}, fmt.Errorf("json: %w", err)
	}
	out := IDs{
		DOI:   CleanDOI(rawIDs.DOI),
		PMID:  strings.TrimSpace(rawIDs.PMID),
		PMCID: strings.TrimSpace(rawIDs.PMCID),
	}
	if out.PMCID != "" && !strings.HasPrefix(strings.ToUpper(out.PMCID), "PMC") {
		out.PMCID = "PMC" + out.PMCID
	}
	if out.DOI == "" && out.PMID == "" && out.PMCID == "" {
		return IDs{}, fmt.Errorf("ai no ids")
	}
	return out, nil
}
