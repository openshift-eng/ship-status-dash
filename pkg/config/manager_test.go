package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ship-status-dash/pkg/testhelper"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	Value string
}

func createTestConfigFile(t *testing.T, dir, content string) string {
	configPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)
	return configPath
}

func writeConfigFile(t *testing.T, path, content string) {
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
}

func createTestManager(t *testing.T, configPath string) *Manager[testConfig] {
	loadFunc := func(path string) (*testConfig, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		content := string(data)
		value := "default"
		if len(content) > 7 && content[:7] == "value: " {
			end := len(content)
			if idx := len(content) - 1; idx >= 7 {
				value = content[7:end]
				if len(value) > 0 && value[len(value)-1] == '\n' {
					value = value[:len(value)-1]
				}
			}
		}
		return &testConfig{Value: value}, nil
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	// Use a shorter debounce delay for tests to make them run faster
	manager, err := NewManager(configPath, loadFunc, logger, 100*time.Millisecond)
	require.NoError(t, err)
	return manager
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		loadFunc      func(string) (*testConfig, error)
		wantErr       bool
		wantConfig    *testConfig
		checkHash     bool
	}{
		{
			name:          "successfully creates manager with valid config file",
			configContent: "value: test",
			loadFunc: func(path string) (*testConfig, error) {
				return &testConfig{Value: "test"}, nil
			},
			wantConfig: &testConfig{Value: "test"},
		},
		{
			name:          "initializes hash correctly",
			configContent: "value: test",
			loadFunc: func(path string) (*testConfig, error) {
				return &testConfig{Value: "test"}, nil
			},
			checkHash: true,
		},
		{
			name:          "returns error when loadFunc fails",
			configContent: "value: test",
			loadFunc: func(path string) (*testConfig, error) {
				return nil, os.ErrNotExist
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, tt.configContent)

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			manager, err := NewManager(configPath, tt.loadFunc, logger, 100*time.Millisecond)

			if tt.wantErr {
				wantErr := os.ErrNotExist
				if diff := cmp.Diff(wantErr, err, testhelper.EquateErrorMessage); diff != "" {
					t.Errorf("Error mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff((*Manager[testConfig])(nil), manager); diff != "" {
					t.Errorf("Manager mismatch (-want +got):\n%s", diff)
				}
			} else {
				var wantErr error = nil
				if diff := cmp.Diff(wantErr, err, testhelper.EquateErrorMessage); diff != "" {
					t.Errorf("Error mismatch (-want +got):\n%s", diff)
				}
				if manager == nil {
					t.Error("Expected non-nil manager but got nil")
				}

				if tt.wantConfig != nil {
					if diff := cmp.Diff(tt.wantConfig, manager.Get()); diff != "" {
						t.Errorf("Config mismatch (-want +got):\n%s", diff)
					}
				}

				if tt.checkHash {
					manager.mu.RLock()
					hash := manager.lastHash
					manager.mu.RUnlock()
					if diff := cmp.Diff("", hash); diff == "" {
						t.Error("Expected non-empty hash but got empty")
					}
				}
			}
		})
	}
}

func TestManager_OnUpdate(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		numCallbacks  int
	}{
		{
			name:          "registers callbacks correctly",
			configContent: "value: test",
			numCallbacks:  1,
		},
		{
			name:          "multiple callbacks can be registered",
			configContent: "value: test",
			numCallbacks:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, tt.configContent)
			manager := createTestManager(t, configPath)

			callbackCalled := false
			callback := func(cfg *testConfig) {
				callbackCalled = true
			}

			for i := 0; i < tt.numCallbacks; i++ {
				manager.OnUpdate(callback)
			}

			if diff := cmp.Diff(false, callbackCalled); diff != "" {
				t.Errorf("Callback expectation mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestManager_Watch_FileEvents(t *testing.T) {
	tests := []struct {
		name            string
		initialContent  string
		setupFile       func(*testing.T, string)
		waitAfterSetup  time.Duration
		wantCallback    bool
		wantConfig      *testConfig
		wantConfigValue string
	}{
		{
			name:           "Write event triggers reload",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, path string) {
				writeConfigFile(t, path, "value: updated")
			},
			waitAfterSetup: 200 * time.Millisecond,
			wantCallback:   true,
			wantConfig:     &testConfig{Value: "updated"},
		},
		{
			name:           "Create event triggers reload",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, path string) {
				os.Remove(path)
				time.Sleep(50 * time.Millisecond)
				writeConfigFile(t, path, "value: recreated")
			},
			waitAfterSetup: 200 * time.Millisecond,
			wantCallback:   true,
		},
		{
			name:           "Rename event triggers reload",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, path string) {
				tmpPath := path + ".tmp"
				writeConfigFile(t, tmpPath, "value: renamed")
				err := os.Rename(tmpPath, path)
				require.NoError(t, err)
			},
			waitAfterSetup: 200 * time.Millisecond,
			wantCallback:   true,
		},
		{
			name:           "Remove event triggers reload attempt",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, path string) {
				err := os.Remove(path)
				require.NoError(t, err)
			},
			waitAfterSetup:  600 * time.Millisecond,
			wantCallback:    false,
			wantConfigValue: "initial",
		},
		{
			name:           "file path filtering works correctly",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, configPath string) {
				tmpDir := filepath.Dir(configPath)
				otherPath := filepath.Join(tmpDir, "other.yaml")
				writeConfigFile(t, otherPath, "value: other-updated")
			},
			waitAfterSetup:  600 * time.Millisecond,
			wantCallback:    false,
			wantConfigValue: "initial",
		},
		{
			name:           "directory events trigger reload even when path doesn't match",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, configPath string) {
				tmpDir := filepath.Dir(configPath)
				otherFile := filepath.Join(tmpDir, "other.yaml")
				writeConfigFile(t, otherFile, "value: other")
				err := os.Rename(otherFile, filepath.Join(tmpDir, "renamed.yaml"))
				require.NoError(t, err)
			},
			waitAfterSetup:  600 * time.Millisecond,
			wantCallback:    false,
			wantConfigValue: "initial",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, tt.initialContent)
			manager := createTestManager(t, configPath)

			callbackCalled := false
			var callbackConfig *testConfig
			manager.OnUpdate(func(cfg *testConfig) {
				callbackCalled = true
				callbackConfig = cfg
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := manager.Watch(ctx)
			require.NoError(t, err)

			time.Sleep(50 * time.Millisecond)

			tt.setupFile(t, configPath)

			time.Sleep(tt.waitAfterSetup)

			if diff := cmp.Diff(tt.wantCallback, callbackCalled); diff != "" {
				t.Errorf("Callback call expectation mismatch (-want +got):\n%s", diff)
			}

			if tt.wantConfig != nil {
				if diff := cmp.Diff(tt.wantConfig, callbackConfig); diff != "" {
					t.Errorf("Callback config mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(tt.wantConfig, manager.Get()); diff != "" {
					t.Errorf("Manager config mismatch (-want +got):\n%s", diff)
				}
			}

			if tt.wantConfigValue != "" {
				if diff := cmp.Diff(tt.wantConfigValue, manager.Get().Value); diff != "" {
					t.Errorf("Config value mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestManager_HashValidation(t *testing.T) {
	tests := []struct {
		name           string
		initialContent string
		updateContent  string
		wantCallback   bool
		wantConfig     *testConfig
	}{
		{
			name:           "reload skipped when content unchanged",
			initialContent: "value: test",
			updateContent:  "value: test",
			wantCallback:   false,
		},
		{
			name:           "reload proceeds when content changed",
			initialContent: "value: initial",
			updateContent:  "value: updated",
			wantCallback:   true,
			wantConfig:     &testConfig{Value: "updated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, tt.initialContent)
			manager := createTestManager(t, configPath)

			callbackCalled := false
			manager.OnUpdate(func(cfg *testConfig) {
				callbackCalled = true
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := manager.Watch(ctx)
			require.NoError(t, err)

			time.Sleep(50 * time.Millisecond)

			writeConfigFile(t, configPath, tt.updateContent)

			time.Sleep(200 * time.Millisecond)

			if diff := cmp.Diff(tt.wantCallback, callbackCalled); diff != "" {
				t.Errorf("Callback expectation mismatch (-want +got):\n%s", diff)
			}

			if tt.wantConfig != nil {
				if diff := cmp.Diff(tt.wantConfig, manager.Get()); diff != "" {
					t.Errorf("Config mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestManager_Debouncing(t *testing.T) {
	tests := []struct {
		name            string
		initialContent  string
		setupFile       func(*testing.T, string)
		waitAfterSetup  time.Duration
		wantReloadCount int
	}{
		{
			name:           "multiple rapid events only trigger one reload",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, path string) {
				for i := 0; i < 5; i++ {
					writeConfigFile(t, path, "value: updated")
					time.Sleep(10 * time.Millisecond)
				}
			},
			waitAfterSetup:  600 * time.Millisecond,
			wantReloadCount: 1,
		},
		{
			name:           "debounce timer is reset on new events",
			initialContent: "value: initial",
			setupFile: func(t *testing.T, path string) {
				writeConfigFile(t, path, "value: update1")
				time.Sleep(50 * time.Millisecond)
				writeConfigFile(t, path, "value: update2")
			},
			waitAfterSetup:  200 * time.Millisecond,
			wantReloadCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, tt.initialContent)
			manager := createTestManager(t, configPath)

			reloadCount := 0
			var mu sync.Mutex
			manager.OnUpdate(func(cfg *testConfig) {
				mu.Lock()
				reloadCount++
				mu.Unlock()
			})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := manager.Watch(ctx)
			require.NoError(t, err)

			time.Sleep(50 * time.Millisecond)

			tt.setupFile(t, configPath)

			time.Sleep(tt.waitAfterSetup)

			mu.Lock()
			count := reloadCount
			mu.Unlock()

			if diff := cmp.Diff(tt.wantReloadCount, count); diff != "" {
				t.Errorf("Reload count mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestManager_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		initialConfig *testConfig
		setupManager  func(*testing.T, *Manager[testConfig], string)
		wantConfig    *testConfig
	}{
		{
			name:          "failed reload keeps existing config",
			initialConfig: &testConfig{Value: "initial"},
			setupManager: func(t *testing.T, manager *Manager[testConfig], configPath string) {
				manager.loadFunc = func(path string) (*testConfig, error) {
					return nil, os.ErrNotExist
				}
				manager.reload()
			},
			wantConfig: &testConfig{Value: "initial"},
		},
		{
			name:          "file read errors are handled gracefully",
			initialConfig: &testConfig{Value: "initial"},
			setupManager: func(t *testing.T, manager *Manager[testConfig], configPath string) {
				err := os.Remove(configPath)
				require.NoError(t, err)
				manager.reload()
			},
			wantConfig: &testConfig{Value: "initial"},
		},
		{
			name:          "invalid config from loadFunc keeps existing config",
			initialConfig: &testConfig{Value: "initial"},
			setupManager: func(t *testing.T, manager *Manager[testConfig], configPath string) {
				writeConfigFile(t, configPath, "value: updated")
				manager.loadFunc = func(path string) (*testConfig, error) {
					return nil, os.ErrInvalid
				}
				manager.reload()
			},
			wantConfig: &testConfig{Value: "initial"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := createTestConfigFile(t, tmpDir, "value: initial")

			loadFunc := func(path string) (*testConfig, error) {
				return tt.initialConfig, nil
			}
			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			manager, err := NewManager(configPath, loadFunc, logger, 100*time.Millisecond)
			require.NoError(t, err)

			tt.setupManager(t, manager, configPath)

			if diff := cmp.Diff(tt.wantConfig, manager.Get()); diff != "" {
				t.Errorf("Config mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
