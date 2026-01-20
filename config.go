package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type GameConfig struct {
	StartingResources map[string]int   `yaml:"startingResources"`
	Industries        []IndustryConfig `yaml:"industry"`
}

type IndustryConfig struct {
	Key      string         `yaml:"industry"`
	Name     string         `yaml:"name"`
	Resource string         `yaml:"resource"`
	Workers  []WorkerConfig `yaml:"workers"`
}

type WorkerConfig struct {
	Key         string         `yaml:"worker"`
	WorkerName  string         `yaml:"workerName"`
	Produces    string         `yaml:"produces"`
	ProdRate    time.Duration  `yaml:"prodRate"`
	ProdQuant   int            `yaml:"prodQuant"`
	UpgradeMult float64        `yaml:"upgradeMult"`
	AutoTier    int            `yaml:"autoTier"`
	Cost        map[string]int `yaml:"cost"`
}

func LoadConfig(path string) (GameConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GameConfig{}, fmt.Errorf("read config: %w", err)
	}

	var cfg GameConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return GameConfig{}, fmt.Errorf("parse yaml: %w", err)
	}

	if len(cfg.Industries) == 0 {
		return GameConfig{}, fmt.Errorf("no industries defined")
	}

	for i, industry := range cfg.Industries {
		if industry.Key == "" {
			return GameConfig{}, fmt.Errorf("industry %d missing key", i)
		}
		if industry.Resource == "" {
			return GameConfig{}, fmt.Errorf("industry %s missing resource", industry.Key)
		}
		if len(industry.Workers) == 0 {
			return GameConfig{}, fmt.Errorf("industry %s missing workers", industry.Key)
		}

		for j, worker := range industry.Workers {
			if worker.Key == "" {
				return GameConfig{}, fmt.Errorf("industry %s worker %d missing key", industry.Key, j)
			}
			if worker.WorkerName == "" {
				return GameConfig{}, fmt.Errorf("industry %s worker %s missing workerName", industry.Key, worker.Key)
			}
			if worker.ProdRate <= 0 {
				return GameConfig{}, fmt.Errorf("industry %s worker %s missing prodRate", industry.Key, worker.Key)
			}
			if worker.ProdQuant <= 0 {
				return GameConfig{}, fmt.Errorf("industry %s worker %s missing prodQuant", industry.Key, worker.Key)
			}
			if worker.UpgradeMult <= 0 {
				return GameConfig{}, fmt.Errorf("industry %s worker %s missing upgradeMult", industry.Key, worker.Key)
			}
			if worker.Cost == nil {
				return GameConfig{}, fmt.Errorf("industry %s worker %s missing cost", industry.Key, worker.Key)
			}
		}
	}

	return cfg, nil
}
