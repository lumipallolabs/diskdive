package stats

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Stats holds persistent statistics
type Stats struct {
	FreedLifetime int64  `json:"freed_lifetime"`
	DefaultDrive  string `json:"default_drive,omitempty"` // Path of default drive to scan on startup
}

// Manager handles loading and saving stats
type Manager struct {
	path         string
	stats        Stats
	mu           sync.RWMutex
	dirty        bool
	saveTimer    *time.Timer
	saveDuration time.Duration
}

// NewManager creates a new stats manager
func NewManager() *Manager {
	return &Manager{
		path:         defaultPath(),
		saveDuration: 2 * time.Second, // Debounce saves
	}
}

// defaultPath returns the default stats file path
func defaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".diskdive-stats.json"
	}
	return filepath.Join(home, ".diskdive", "stats.json")
}

// Load loads stats from disk
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			// No stats file yet, start fresh
			m.stats = Stats{}
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &m.stats)
}

// Save saves stats to disk immediately
func (m *Manager) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.saveLocked()
}

// saveLocked saves stats without acquiring the lock (caller must hold lock)
func (m *Manager) saveLocked() error {
	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m.stats, "", "  ")
	if err != nil {
		return err
	}

	m.dirty = false
	return os.WriteFile(m.path, data, 0644)
}

// FreedLifetime returns the lifetime freed bytes
func (m *Manager) FreedLifetime() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats.FreedLifetime
}

// DefaultDrive returns the default drive path
func (m *Manager) DefaultDrive() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats.DefaultDrive
}

// SetDefaultDrive sets the default drive path and saves
func (m *Manager) SetDefaultDrive(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stats.DefaultDrive == path {
		return
	}

	m.stats.DefaultDrive = path
	m.dirty = true

	// Cancel any pending save timer
	if m.saveTimer != nil {
		m.saveTimer.Stop()
	}

	// Schedule a debounced save
	m.saveTimer = time.AfterFunc(m.saveDuration, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.dirty {
			_ = m.saveLocked()
		}
	})
}

// AddFreed adds to the lifetime freed counter and schedules a debounced save
func (m *Manager) AddFreed(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.FreedLifetime += bytes
	m.dirty = true

	// Cancel any pending save timer
	if m.saveTimer != nil {
		m.saveTimer.Stop()
	}

	// Schedule a debounced save
	m.saveTimer = time.AfterFunc(m.saveDuration, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.dirty {
			_ = m.saveLocked() // Ignore errors for background save
		}
	})
}

// Close ensures any pending saves are written
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.saveTimer != nil {
		m.saveTimer.Stop()
		m.saveTimer = nil
	}

	if m.dirty {
		return m.saveLocked()
	}
	return nil
}
