package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/dave/kube-tui/internal/model"
)

type kubeconfigEntry struct {
	Path  string
	Group string
}

func DiscoverKubeconfigs() ([]kubeconfigEntry, error) {
	var entries []kubeconfigEntry

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}

	defaultPath := filepath.Join(home, ".kube", "config")
	if _, err := os.Stat(defaultPath); err == nil {
		entries = append(entries, kubeconfigEntry{Path: defaultPath, Group: "default"})
	}

	configsDir := filepath.Join(home, ".kube", "configs")
	entries = append(entries, scanDir(configsDir, "")...)

	if len(entries) == 0 {
		return nil, fmt.Errorf("no kubeconfig files found in ~/.kube/config or ~/.kube/configs/")
	}

	return entries, nil
}

func scanDir(dir, group string) []kubeconfigEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []kubeconfigEntry
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			result = append(result, scanDir(path, e.Name())...)
		} else {
			result = append(result, kubeconfigEntry{Path: path, Group: group})
		}
	}
	return result
}

func LoadClusters(entries []kubeconfigEntry) ([]model.Cluster, error) {
	var clusters []model.Cluster

	for _, entry := range entries {
		cfg, err := clientcmd.LoadFromFile(entry.Path)
		if err != nil {
			continue
		}

		for ctxName, ctx := range cfg.Contexts {
			cluster := cfg.Clusters[ctx.Cluster]
			server := ""
			if cluster != nil {
				server = cluster.Server
			}

			group := entry.Group
			if group == "" {
				group = "other"
			}

			clusters = append(clusters, model.Cluster{
				Name:       fmt.Sprintf("%s / %s", filepath.Base(entry.Path), ctxName),
				Kubeconfig: entry.Path,
				Context:    ctxName,
				Server:     server,
				Group:      group,
			})
		}
	}

	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Group != clusters[j].Group {
			return clusters[i].Group < clusters[j].Group
		}
		return clusters[i].Name < clusters[j].Name
	})

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
