package k8s

import (
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/dave/kube-tui/internal/config"
	"github.com/dave/kube-tui/internal/model"
)

func NewClient(cluster model.Cluster) (*kubernetes.Clientset, *rest.Config, error) {
	clientConfig := config.BuildClientConfig(cluster)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("rest config: %w", err)
	}

	restConfig.QPS = 100
	restConfig.Burst = 200

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("clientset: %w", err)
	}

	return clientset, restConfig, nil
}

func NewRESTConfig(cluster model.Cluster) (*rest.Config, error) {
	clientConfig := config.BuildClientConfig(cluster)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("rest config: %w", err)
	}

	restConfig.QPS = 100
	restConfig.Burst = 200

	return restConfig, nil
}

func CheckConnection(clientset *kubernetes.Clientset) error {
	_, err := clientset.ServerVersion()
	return err
}

func CheckClusterHealth(cluster model.Cluster) error {
	clientConfig := config.BuildClientConfig(cluster)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	restConfig.Timeout = 5 * time.Second
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	return CheckConnection(clientset)
}
