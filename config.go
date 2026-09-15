package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type GlobalConfig struct {
	BaseURL  string `json:"base_url"`
	Email    string `json:"email"`
	Username string `json:"username"`
	UserID   int    `json:"user_id"`
	Token    string `json:"token"`
}

func getConfigPath() (string, error) {
	homeDIr, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("Can't find user's home folder")
	}

	configDIr := filepath.Join(homeDIr, ".lectern")

	if err := os.MkdirAll(configDIr, 0755); err != nil {
		return "", fmt.Errorf("Failed creating config filder: %w", err)
	}

	return filepath.Join(configDIr, "config.json"), nil
}

func LoadConfig() (*GlobalConfig, error) {
	path, err := getConfigPath()

	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)

	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{}, nil
		}
		return nil, err
	}

	var config GlobalConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("Failed reading config file: %w", err)
	}

	return &config, nil
}

func SaveConfig(config *GlobalConfig) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("Failed encoding: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("Failed writing in the config file: %w", err)
	}

	return nil
}

type ConfigStatus struct {
	Path      string
	Connected bool
	BaseURL   string
	Username  string
	Email     string
	UserID    int
}

func CheckConfig() (*ConfigStatus, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	return &ConfigStatus{
		Path:      path,
		Connected: cfg.Token != "",
		BaseURL:   cfg.BaseURL,
		Username:  cfg.Username,
		Email:     cfg.Email,
		UserID:    cfg.UserID,
	}, nil
}

func ClearConfig() error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return nil
}
