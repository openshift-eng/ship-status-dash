package config

import "github.com/sirupsen/logrus"

// CreateTestConfigManager creates a config manager for testing purposes.
func CreateTestConfigManager[T any](cfg *T) *Manager[T] {
	loadFunc := func(string) (*T, error) {
		return cfg, nil
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	manager, _ := NewManager("", loadFunc, logger, 0)
	return manager
}
