package features

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	FlagCLIFileMount = "cli-file-mount"

	FlagMachines = "machines"
)

const DefaultTTL = 5 * time.Minute

const FetchTimeout = 2 * time.Second

const cacheFilename = "flags-cache.json"

var Defaults = map[string]bool{
	FlagCLIFileMount: false,
	FlagMachines:     false,
}

type Fetcher interface {
	Get(ctx context.Context, path string, query url.Values, respTarget interface{}) error
	APIKey() string
}

type Flags struct {
	values map[string]bool
	source string // "cache", "fetch", "stale-cache", "defaults"
}

func NewFlagsForTest(values map[string]bool) *Flags {
	return &Flags{values: values, source: "test"}
}

func (f *Flags) IsEnabled(key string) bool {
	if f == nil {
		if v, ok := Defaults[key]; ok {
			return v
		}
		return false
	}
	if v, ok := f.values[key]; ok {
		return v
	}
	if v, ok := Defaults[key]; ok {
		return v
	}
	return false
}

func (f *Flags) Source() string {
	if f == nil {
		return "defaults"
	}
	return f.source
}

type cacheEntry struct {
	FetchedAt time.Time       `json:"fetchedAt"`
	Values    map[string]bool `json:"values"`
}

type cacheFile struct {
	Entries map[string]cacheEntry `json:"entries"`
}

func fingerprint(apiURL, apiKey string) string {
	sum := sha256.Sum256([]byte(apiURL + "\x00" + apiKey))
	return hex.EncodeToString(sum[:8])
}

func DefaultCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".idapt", cacheFilename), nil
}

func loadCache(path, fp string) (cacheEntry, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cacheEntry{}, false, nil
		}
		return cacheEntry{}, false, err
	}
	if len(data) == 0 {
		return cacheEntry{}, false, nil
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return cacheEntry{}, false, nil
	}
	entry, ok := cf.Entries[fp]
	return entry, ok, nil
}

func saveCache(path, fp string, entry cacheEntry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	cf := cacheFile{Entries: map[string]cacheEntry{}}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, &cf)
		if cf.Entries == nil {
			cf.Entries = map[string]cacheEntry{}
		}
	}
	cf.Entries[fp] = entry

	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

type Loader struct {
	Client    Fetcher
	CachePath string
	TTL       time.Duration
	Now       func() time.Time // injectable for tests
}

func NewLoader(client Fetcher) *Loader {
	path, _ := DefaultCachePath() // empty path → cache disabled
	return &Loader{
		Client:    client,
		CachePath: path,
		TTL:       DefaultTTL,
		Now:       time.Now,
	}
}

func (l *Loader) Load(ctx context.Context) (*Flags, error) {
	if l == nil {
		return &Flags{values: Defaults, source: "defaults"}, nil
	}
	if l.Now == nil {
		l.Now = time.Now
	}

	apiKey := ""
	apiURL := ""
	if l.Client != nil {
		apiKey = l.Client.APIKey()
	}
	fp := fingerprint(apiURL, apiKey)

	if l.CachePath != "" {
		if entry, ok, _ := loadCache(l.CachePath, fp); ok {
			if l.Now().Sub(entry.FetchedAt) < l.TTL {
				return &Flags{values: entry.Values, source: "cache"}, nil
			}
		}
	}

	if l.Client != nil {
		fetchCtx, cancel := context.WithTimeout(ctx, FetchTimeout)
		defer cancel()

		var resp map[string]bool
		err := l.Client.Get(fetchCtx, "/api/feature-flags/me", nil, &resp)
		if err == nil && resp != nil {
			entry := cacheEntry{FetchedAt: l.Now(), Values: resp}
			if l.CachePath != "" {
				_ = saveCache(l.CachePath, fp, entry) // best-effort
			}
			return &Flags{values: resp, source: "fetch"}, nil
		}

		if l.CachePath != "" {
			if entry, ok, _ := loadCache(l.CachePath, fp); ok {
				return &Flags{values: entry.Values, source: "stale-cache"}, nil
			}
		}
	}

	return &Flags{values: Defaults, source: "defaults"}, nil
}

func LoadFromCache(cachePath, apiKey string) *Flags {
	if cachePath == "" {
		return nil
	}
	fp := fingerprint("", apiKey)
	entry, ok, _ := loadCache(cachePath, fp)
	if !ok {
		return nil
	}
	return &Flags{values: entry.Values, source: "cache"}
}

func Invalidate(cachePath, apiKey string) error {
	if cachePath == "" {
		return nil
	}
	fp := fingerprint("", apiKey)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil // corrupt — let next save overwrite
	}
	if cf.Entries == nil {
		return nil
	}
	delete(cf.Entries, fp)

	out, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(cachePath, out, 0600)
}
