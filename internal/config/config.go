package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const AppName = "pdf-watcher"

// AI is any OpenAI-compatible chat completions API (DeepSeek / OpenAI / 通义 / …).
// Only used to find DOI/PMID; bibliographic fields always come from Crossref/PubMed.
type AI struct {
	Enable     bool   `yaml:"enable"`
	APIKey     string `yaml:"apiKey"`
	APIURL     string `yaml:"apiUrl"` // .../v1/chat/completions
	Model      string `yaml:"model"`
	TimeoutSec int    `yaml:"timeoutSec"`
}

type Zotero struct {
	LocalAPIURL          string `yaml:"localApiUrl"`
	ServerID             string `yaml:"serverId"`
	APIKey               string `yaml:"apiKey"`
	DefaultCollectionID  string `yaml:"defaultCollectionId"`
}

type WatchDir struct {
	Path           string   `yaml:"path"`
	Enable         bool     `yaml:"enable"`
	CollectionID   string   `yaml:"collectionId"`   // Zotero key; filled after first ensure
	CollectionName string   `yaml:"collectionName"` // under pdf-watcher; assigned locally on add/reconcile
	IgnoreNames    []string `yaml:"ignoreNames"`
}

type System struct {
	AutoStart     bool `yaml:"autoStart"`
	StableWaitMs  int  `yaml:"stableWaitMs"`
	StablePollMs  int  `yaml:"stablePollMs"`
	StableTimeoutMs int `yaml:"stableTimeoutMs"`
	MaxQueue      int  `yaml:"maxQueue"`
}

type PubMed struct {
	Tool       string `yaml:"tool"`
	Email      string `yaml:"email"`
	TimeoutSec int    `yaml:"timeoutSec"`
}

type IndexCfg struct {
	RetentionDays int `yaml:"retentionDays"`
}

type UI struct {
	Language string `yaml:"language"` // "" | "zh-CN" | "en"
}

type Config struct {
	AI        AI         `yaml:"ai"`
	Zotero    Zotero     `yaml:"zotero"`
	PubMed    PubMed     `yaml:"pubmed"`
	WatchDirs []WatchDir `yaml:"watchDirs"`
	System    System     `yaml:"system"`
	Index     IndexCfg   `yaml:"index"`
	UI        UI         `yaml:"ui"`
}

type Manager struct {
	mu       sync.RWMutex
	cfg      Config
	path     string
	dir      string
	onChange []func(Config)
}

func AppDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, AppName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func Default() Config {
	return Config{
		AI: AI{
			Enable:     true,
			APIKey:     "",
			APIURL:     "https://api.deepseek.com/v1/chat/completions",
			Model:      "deepseek-chat",
			TimeoutSec: 30,
		},
		Zotero: Zotero{
			LocalAPIURL: "http://127.0.0.1:23119/api",
		},
		PubMed: PubMed{
			Tool:       AppName,
			Email:      "",
			TimeoutSec: 30,
		},
		WatchDirs: []WatchDir{
			{Path: "", Enable: true, CollectionID: ""},
		},
		System: System{
			AutoStart:       true,
			StableWaitMs:    3000,
			StablePollMs:    500,
			StableTimeoutMs: 120000,
			MaxQueue:        200,
		},
		Index: IndexCfg{RetentionDays: 0},
		UI:    UI{Language: ""},
	}
}

func LoadOrCreate() (*Manager, error) {
	dir, err := AppDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.yaml")
	m := &Manager{path: path, dir: dir}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			m.cfg = Default()
			if err := m.Save(); err != nil {
				return nil, err
			}
			return m, nil
		}
		return nil, err
	}
	cfg, err := parseConfig(data)
	if err != nil {
		return nil, err
	}
	m.cfg = cfg
	return m, nil
}

func parseConfig(data []byte) (Config, error) {
	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	migrateLegacyDeepseek(data, &cfg)
	normalize(&cfg)
	return cfg, nil
}

// migrateLegacyDeepseek copies old `deepseek:` block into `ai:` when `ai` was absent.
func migrateLegacyDeepseek(data []byte, c *Config) {
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return
	}
	if _, hasAI := root["ai"]; hasAI {
		return
	}
	if _, hasDS := root["deepseek"]; !hasDS {
		return
	}
	var leg struct {
		Deepseek AI `yaml:"deepseek"`
	}
	if err := yaml.Unmarshal(data, &leg); err != nil {
		return
	}
	c.AI = leg.Deepseek
}

func normalize(c *Config) {
	if c.AI.APIURL == "" {
		c.AI.APIURL = Default().AI.APIURL
	}
	if c.AI.Model == "" {
		c.AI.Model = Default().AI.Model
	}
	if c.AI.TimeoutSec <= 0 {
		c.AI.TimeoutSec = 30
	}
	if c.Zotero.LocalAPIURL == "" || !isLoopbackHTTPURL(c.Zotero.LocalAPIURL) {
		c.Zotero.LocalAPIURL = Default().Zotero.LocalAPIURL
	}
	if c.PubMed.Tool == "" {
		c.PubMed.Tool = AppName
	}
	if c.PubMed.TimeoutSec <= 0 {
		c.PubMed.TimeoutSec = 30
	}
	if c.System.StableWaitMs <= 0 {
		c.System.StableWaitMs = 3000
	}
	if c.System.StablePollMs <= 0 {
		c.System.StablePollMs = 500
	}
	if c.System.StableTimeoutMs <= 0 {
		c.System.StableTimeoutMs = 120000
	}
	if c.System.MaxQueue <= 0 {
		c.System.MaxQueue = 200
	}
}

func (m *Manager) Path() string { return m.path }
func (m *Manager) Dir() string  { return m.dir }

func (m *Manager) LogsDir() string {
	d := filepath.Join(m.dir, "logs")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func (m *Manager) IndexPath() string {
	return filepath.Join(m.dir, "index.db")
}

func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) OnChange(fn func(Config)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = append(m.onChange, fn)
}

func (m *Manager) Update(fn func(*Config)) error {
	m.mu.Lock()
	fn(&m.cfg)
	normalize(&m.cfg)
	cfg := m.cfg
	cbs := append([]func(Config){}, m.onChange...)
	m.mu.Unlock()
	if err := m.write(cfg); err != nil {
		return err
	}
	for _, cb := range cbs {
		cb(cfg)
	}
	return nil
}

func (m *Manager) Save() error {
	m.mu.RLock()
	cfg := m.cfg
	m.mu.RUnlock()
	return m.write(cfg)
}

func (m *Manager) write(cfg Config) error {
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

func (m *Manager) ReloadFromDisk() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}
	cfg, err := parseConfig(data)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = cfg
	cbs := append([]func(Config){}, m.onChange...)
	m.mu.Unlock()
	for _, cb := range cbs {
		cb(cfg)
	}
	return nil
}

// isLoopbackHTTPURL requires Zotero Local API to stay on this machine
// so the stored API key is never sent to a remote host by misconfig.
func isLoopbackHTTPURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

// Debounce helper for config file watcher.
func Debounce(d time.Duration, fn func()) func() {
	var mu sync.Mutex
	var t *time.Timer
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(d, fn)
	}
}
