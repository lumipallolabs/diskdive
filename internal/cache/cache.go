package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/samuli/diskdive/internal/model"
)

// Cache handles saving and loading scan results
type Cache struct {
	dir string
}

// New creates a new cache in the given directory
func New(dir string) *Cache {
	return &Cache{dir: dir}
}

// DefaultDir returns the default cache directory
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".diskdive"
	}
	return filepath.Join(home, ".diskdive", "cache")
}

// Save saves a scan result for the given drive
func (c *Cache) Save(driveLetter string, root *model.Node) error {
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.json",
		driveLetter,
		time.Now().Format("2006-01-02_150405"))

	path := filepath.Join(c.dir, filename)

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

// LoadLatest loads the most recent cache for a drive
func (c *Cache) LoadLatest(driveLetter string) (*model.Node, error) {
	pattern := filepath.Join(c.dir, fmt.Sprintf("%s_*.json", driveLetter))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no cache found for drive %s", driveLetter)
	}

	// Sort to get latest (filenames include timestamp)
	sort.Strings(files)
	latest := files[len(files)-1]

	data, err := os.ReadFile(latest)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var root model.Node
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return &root, nil
}

// Timestamp returns the timestamp of the latest cache
func (c *Cache) Timestamp(driveLetter string) (time.Time, error) {
	pattern := filepath.Join(c.dir, fmt.Sprintf("%s_*.json", driveLetter))
	files, _ := filepath.Glob(pattern)
	if len(files) == 0 {
		return time.Time{}, fmt.Errorf("no cache")
	}

	sort.Strings(files)
	latest := files[len(files)-1]

	// Extract timestamp from filename
	base := filepath.Base(latest)
	base = strings.TrimSuffix(base, ".json")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid filename")
	}

	return time.Parse("2006-01-02_150405", parts[1])
}
