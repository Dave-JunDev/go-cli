package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/dave/kube-tui/internal/model"
)

type UIConfig struct {
	Groups map[string][]string `yaml:"groups"`
}

func LoadUIConfig() (*UIConfig, error) {
	path, err := configPath()
	if err != nil {
		return &UIConfig{Groups: make(map[string][]string)}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &UIConfig{Groups: make(map[string][]string)}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg UIConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Groups == nil {
		cfg.Groups = make(map[string][]string)
	}
	return &cfg, nil
}

func (c *UIConfig) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644)
}

func (c *UIConfig) GroupForCluster(clusterName string) string {
	for group, members := range c.Groups {
		for _, m := range members {
			if m == clusterName {
				return group
			}
		}
	}
	return ""
}

func (c *UIConfig) AssignCluster(clusterName, group string) {
	for g := range c.Groups {
		members := c.Groups[g]
		for i, m := range members {
			if m == clusterName {
				c.Groups[g] = append(members[:i], members[i+1:]...)
				if len(c.Groups[g]) == 0 {
					delete(c.Groups, g)
				}
				break
			}
		}
	}
	if group != "" {
		c.Groups[group] = append(c.Groups[group], clusterName)
	}
}

func (c *UIConfig) RemoveGroup(name string) {
	delete(c.Groups, name)
}

func (c *UIConfig) RenameGroup(oldName, newName string) {
	if members, ok := c.Groups[oldName]; ok {
		c.Groups[newName] = members
		delete(c.Groups, oldName)
	}
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "k8s-ui"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func GroupClusters(clusters []model.Cluster, cfg *UIConfig) map[string][]model.Cluster {
	grouped := make(map[string][]model.Cluster)
	for _, c := range clusters {
		g := cfg.GroupForCluster(c.Name)
		if g == "" {
			g = "ungrouped"
		}
		grouped[g] = append(grouped[g], c)
	}
	return grouped
}
