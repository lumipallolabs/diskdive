package cache

import (
	"compress/gzip"
	"encoding/gob"
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

	filename := fmt.Sprintf("%s_%s.gob.gz",
		driveLetter,
		time.Now().Format("2006-01-02_150405"))

	path := filepath.Join(c.dir, filename)

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	// Convert to CacheNode to avoid circular references
	cacheNode := root.ToCacheNode()

	encoder := gob.NewEncoder(gzWriter)
	if err := encoder.Encode(cacheNode); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	return nil
}

// LoadLatest loads the most recent cache for a drive
func (c *Cache) LoadLatest(driveLetter string) (*model.Node, error) {
	pattern := filepath.Join(c.dir, fmt.Sprintf("%s_*.gob.gz", driveLetter))
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

	file, err := os.Open(latest)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gzReader.Close()

	var cacheNode model.CacheNode
	decoder := gob.NewDecoder(gzReader)
	if err := decoder.Decode(&cacheNode); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	// Convert back to Node (this also sets Parent links)
	return cacheNode.ToNode(nil), nil
}

// Timestamp returns the timestamp of the latest cache
func (c *Cache) Timestamp(driveLetter string) (time.Time, error) {
	pattern := filepath.Join(c.dir, fmt.Sprintf("%s_*.gob.gz", driveLetter))
	files, err := filepath.Glob(pattern)
	if err != nil {
		return time.Time{}, fmt.Errorf("glob error: %w", err)
	}
	if len(files) == 0 {
		return time.Time{}, fmt.Errorf("no cache")
	}

	sort.Strings(files)
	latest := files[len(files)-1]

	// Extract timestamp from filename
	base := filepath.Base(latest)
	base = strings.TrimSuffix(base, ".gob.gz")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid filename")
	}

	return time.Parse("2006-01-02_150405", parts[1])
}
