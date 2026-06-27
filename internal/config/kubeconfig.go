package config

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/dave/kube-tui/internal/model"
)

func DiscoverKubeconfigs() ([]string, error) {
	var files []string

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}

	defaultPath := filepath.Join(home, ".kube", "config")
	if _, err := os.Stat(defaultPath); err == nil {
		files = append(files, defaultPath)
	}

	configsDir := filepath.Join(home, ".kube", "configs")
	if entries, err := os.ReadDir(configsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, filepath.Join(configsDir, e.Name()))
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no kubeconfig files found in ~/.kube/config or ~/.kube/configs/")
	}

	return files, nil
}

func LoadClusters(files []string) ([]model.Cluster, error) {
	var clusters []model.Cluster

	for _, path := range files {
		cfg, err := clientcmd.LoadFromFile(path)
		if err != nil {
			continue
		}

		for ctxName, ctx := range cfg.Contexts {
			cluster := cfg.Clusters[ctx.Cluster]
			server := ""
			if cluster != nil {
				server = cluster.Server
			}

			clusters = append(clusters, model.Cluster{
				Name:       fmt.Sprintf("%s / %s", filepath.Base(path), ctxName),
				Kubeconfig: path,
				Context:    ctxName,
				Server:     server,
			})
		}
	}

	return clusters, nil
}

func BuildClientConfig(cluster model.Cluster) clientcmd.ClientConfig {
	loadingRules := &clientcmd.ClientConfigLoadingRules{
		ExplicitPath: cluster.Kubeconfig,
	}
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: cluster.Context,
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	)
}

func GetNamespace(cluster model.Cluster) string {
	clientConfig := BuildClientConfig(cluster)

	raw, err := clientConfig.RawConfig()
	if err != nil {
		return "default"
	}

	ctx, ok := raw.Contexts[cluster.Context]
	if ok && ctx.Namespace != "" {
		return ctx.Namespace
	}

	return "default"
}
