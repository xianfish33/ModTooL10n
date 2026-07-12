package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ModelConfig struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type Provider struct {
	Name          string        `json:"name"`
	BaseURL       string        `json:"base_url"`
	APIKey        string        `json:"api_key"`
	Models        []ModelConfig `json:"models"`
	SelectedModel string        `json:"selected_model"`
	ModelListURL  string        `json:"model_list_url,omitempty"`
}

type Config struct {
	Providers    []Provider `json:"providers"`
	MaxChunkKeys int        `json:"max_chunk_keys"`
	Concurrency  int        `json:"concurrency"`
	MaxRetries   int        `json:"max_retries"`
	OutputDir    string     `json:"output_dir"`
}

func DefaultConfig() *Config {
	return &Config{
		Providers: []Provider{
			{
				Name:    "default",
				BaseURL: "https://api.openai.com/v1",
				APIKey:  "",
				Models: []ModelConfig{
					{Name: "gpt-4o-mini", Active: true},
				},
				SelectedModel: "gpt-4o-mini",
			},
		},
		MaxChunkKeys: 50,
		Concurrency:  2,
		MaxRetries:   3,
		OutputDir:    "output",
	}
}

func ConfigPath() string {
	return filepath.Join(".", "config.json")
}

func (c *Config) Save() error {
	path := ConfigPath()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func Load() (*Config, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if err := cfg.Save(); err != nil {
				return nil, fmt.Errorf("save default config: %w", err)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.MaxChunkKeys <= 0 {
		cfg.MaxChunkKeys = 50
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}

	return &cfg, nil
}

func (c *Config) GetSelectedProvider() (*Provider, error) {
	for i := range c.Providers {
		for _, m := range c.Providers[i].Models {
			if m.Name == c.Providers[i].SelectedModel && m.Active {
				return &c.Providers[i], nil
			}
		}
	}
	for i := range c.Providers {
		for _, m := range c.Providers[i].Models {
			if m.Active {
				return &c.Providers[i], nil
			}
		}
	}
	if len(c.Providers) > 0 {
		return &c.Providers[0], nil
	}
	return nil, fmt.Errorf("no providers configured")
}

func (p *Provider) GetActiveModel() string {
	if p.SelectedModel != "" {
		for _, m := range p.Models {
			if m.Name == p.SelectedModel && m.Active {
				return m.Name
			}
		}
	}
	for _, m := range p.Models {
		if m.Active {
			return m.Name
		}
	}
	if len(p.Models) > 0 {
		return p.Models[0].Name
	}
	return ""
}

func (c *Config) AddProvider(name, baseURL, apiKey, modelListURL string, models []string) error {
	for _, p := range c.Providers {
		if p.Name == name {
			return fmt.Errorf("provider %q already exists", name)
		}
	}
	modelConfigs := make([]ModelConfig, len(models))
	for i, m := range models {
		modelConfigs[i] = ModelConfig{Name: m, Active: true}
	}
	selectedModel := ""
	if len(models) > 0 {
		selectedModel = models[0]
	}
	c.Providers = append(c.Providers, Provider{
		Name:          name,
		BaseURL:       baseURL,
		APIKey:        apiKey,
		Models:        modelConfigs,
		SelectedModel: selectedModel,
		ModelListURL:  modelListURL,
	})
	return c.Save()
}

func (c *Config) RemoveProvider(name string) error {
	idx := -1
	for i, p := range c.Providers {
		if p.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("provider %q not found", name)
	}
	c.Providers = append(c.Providers[:idx], c.Providers[idx+1:]...)
	return c.Save()
}

func (c *Config) UpdateProvider(name, baseURL, apiKey string) error {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			if baseURL != "" {
				c.Providers[i].BaseURL = baseURL
			}
			if apiKey != "" {
				c.Providers[i].APIKey = apiKey
			}
			return c.Save()
		}
	}
	return fmt.Errorf("provider %q not found", name)
}

func (c *Config) SetProviderModels(name string, models []string) error {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			modelConfigs := make([]ModelConfig, len(models))
			for j, m := range models {
				active := true
				for _, existing := range c.Providers[i].Models {
					if existing.Name == m {
						active = existing.Active
						break
					}
				}
				modelConfigs[j] = ModelConfig{Name: m, Active: active}
			}
			c.Providers[i].Models = modelConfigs
			if c.Providers[i].SelectedModel == "" && len(models) > 0 {
				c.Providers[i].SelectedModel = models[0]
			}
			return c.Save()
		}
	}
	return fmt.Errorf("provider %q not found", name)
}

func (c *Config) RemoveModel(providerName, modelName string) error {
	for i := range c.Providers {
		if c.Providers[i].Name == providerName {
			idx := -1
			for j, m := range c.Providers[i].Models {
				if m.Name == modelName {
					idx = j
					break
				}
			}
			if idx == -1 {
				return fmt.Errorf("model %q not found in provider %q", modelName, providerName)
			}
			c.Providers[i].Models = append(c.Providers[i].Models[:idx], c.Providers[i].Models[idx+1:]...)
			if c.Providers[i].SelectedModel == modelName {
				c.Providers[i].SelectedModel = ""
			}
			return c.Save()
		}
	}
	return fmt.Errorf("provider %q not found", providerName)
}

func (c *Config) ToggleModelActive(providerName, modelName string) error {
	for i := range c.Providers {
		if c.Providers[i].Name == providerName {
			for j := range c.Providers[i].Models {
				if c.Providers[i].Models[j].Name == modelName {
					c.Providers[i].Models[j].Active = !c.Providers[i].Models[j].Active
					if !c.Providers[i].Models[j].Active && c.Providers[i].SelectedModel == modelName {
						c.Providers[i].SelectedModel = ""
					}
					return c.Save()
				}
			}
			return fmt.Errorf("model %q not found in provider %q", modelName, providerName)
		}
	}
	return fmt.Errorf("provider %q not found", providerName)
}

func (c *Config) SelectModel(providerName, modelName string) error {
	for i := range c.Providers {
		if c.Providers[i].Name == providerName {
			for _, m := range c.Providers[i].Models {
				if m.Name == modelName && m.Active {
					for j := range c.Providers {
						c.Providers[j].SelectedModel = ""
					}
					c.Providers[i].SelectedModel = modelName
					return c.Save()
				}
			}
			return fmt.Errorf("model %q not found or not active", modelName)
		}
	}
	return fmt.Errorf("provider %q not found", providerName)
}
