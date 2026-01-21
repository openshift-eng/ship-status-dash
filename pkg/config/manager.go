package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// ConfigReloadedMessage is the log message emitted when a config is successfully reloaded.
const ConfigReloadedMessage = "Config reloaded successfully"

// DefaultDebounceDelay is the default debounce delay used to prevent multiple reloads when a config is updated.
const DefaultDebounceDelay = 2 * time.Second

// Manager provides thread-safe configuration management with hot-reload support.
type Manager[T any] struct {
	mu              sync.RWMutex
	config          *T
	configPath      string
	loadFunc        func(string) (*T, error)
	logger          *logrus.Logger
	watcher         *fsnotify.Watcher
	updateCallbacks []func(*T)
	debounceTimer   *time.Timer
	debounceDelay   time.Duration
	lastHash        string
}

// NewManager creates a new config manager with the specified load function.
func NewManager[T any](configPath string, loadFunc func(string) (*T, error), logger *logrus.Logger, debounceDelay time.Duration) (*Manager[T], error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	manager := &Manager[T]{
		configPath:      configPath,
		loadFunc:        loadFunc,
		logger:          logger,
		watcher:         watcher,
		updateCallbacks: make([]func(*T), 0),
		debounceDelay:   debounceDelay,
	}

	if err := manager.load(); err != nil {
		watcher.Close()
		return nil, err
	}

	// Initialize hash from initial config load
	configBytes, err := os.ReadFile(configPath)
	if err == nil {
		hash := sha256.Sum256(configBytes)
		manager.lastHash = hex.EncodeToString(hash[:])
	}

	return manager, nil
}

// Get returns the current configuration in a thread-safe manner.
func (m *Manager[T]) Get() *T {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// load loads and validates the configuration from the file.
func (m *Manager[T]) load() error {
	config, err := m.loadFunc(m.configPath)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	return nil
}

// reload attempts to reload the configuration and update callbacks if successful.
func (m *Manager[T]) reload() {
	m.mu.Lock()
	// Read the file content to compute hash
	configBytes, err := os.ReadFile(m.configPath)
	if err != nil {
		m.mu.Unlock()
		m.logger.WithFields(logrus.Fields{
			"config_path": m.configPath,
			"error":       err,
		}).Error("Failed to read config file for hash validation")
		return
	}

	// Compute hash of current content
	newHash := sha256.Sum256(configBytes)
	newHashStr := hex.EncodeToString(newHash[:])

	// Skip reload if content hasn't changed
	if newHashStr == m.lastHash {
		m.mu.Unlock()
		m.logger.WithField("config_path", m.configPath).Info("Config file content unchanged, skipping reload")
		return
	}

	// Load and validate the new configuration
	newConfig, err := m.loadFunc(m.configPath)
	if err != nil {
		m.mu.Unlock()
		m.logger.WithFields(logrus.Fields{
			"config_path": m.configPath,
			"error":       err,
		}).Error("Failed to reload config, keeping existing config")
		return
	}

	m.config = newConfig
	m.lastHash = newHashStr
	// Copy callbacks to avoid race conditions
	callbacks := make([]func(*T), len(m.updateCallbacks))
	copy(callbacks, m.updateCallbacks)

	m.logger.WithField("config_path", m.configPath).Info(ConfigReloadedMessage)

	// Release lock before calling callbacks to avoid holding lock during potentially slow operations
	m.mu.Unlock()
	for _, callback := range callbacks {
		callback(newConfig)
	}
}

// OnUpdate registers a callback function that will be called when the configuration is updated.
func (m *Manager[T]) OnUpdate(callback func(*T)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCallbacks = append(m.updateCallbacks, callback)
}

// Watch starts watching the configuration file for changes and reloads when changes are detected.
// It watches the directory containing the config file to handle Kubernetes ConfigMap volume updates
// which use symlink updates that can cause the watched inode to disappear.
func (m *Manager[T]) Watch(ctx context.Context) error {
	configDir := filepath.Dir(m.configPath)
	if err := m.watcher.Add(configDir); err != nil {
		return err
	}

	// Normalize the config path for comparison
	normalizedConfigPath := filepath.Clean(m.configPath)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-m.watcher.Events:
				if !ok {
					return
				}

				eventPath := filepath.Clean(event.Name)
				shouldReload := false

				// Kubernetes ConfigMap updates can emit Rename/Remove events on the directory
				// or symlinks (like ..data), which may not match the file path exactly.
				// These directory-level changes can affect the file, so we should reload.
				if event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
					// Directory-level changes may affect the file
					shouldReload = true
				} else if eventPath == normalizedConfigPath {
					// File-level changes (Write, Create, etc.)
					if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
						shouldReload = true
					}
				}

				if shouldReload {
					m.logger.WithFields(logrus.Fields{
						"config_path": m.configPath,
						"event":       event,
						"event_path":  eventPath,
					}).Info("Config file changed, scheduling reload")

					m.mu.Lock()
					if m.debounceTimer != nil {
						m.debounceTimer.Stop()
					}
					m.debounceTimer = time.AfterFunc(m.debounceDelay, m.reload)
					m.mu.Unlock()
				}
			case err, ok := <-m.watcher.Errors:
				if !ok {
					return
				}
				m.logger.WithFields(logrus.Fields{
					"config_path": m.configPath,
					"error":       err,
				}).Error("Error watching config file")
			}
		}
	}()

	return nil
}

// Close closes the file watcher and cleans up resources.
func (m *Manager[T]) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
	}
	return m.watcher.Close()
}
