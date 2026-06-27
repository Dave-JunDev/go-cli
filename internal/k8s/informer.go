package k8s

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/dave/kube-tui/internal/model"
)

type InformerCache struct {
	mu            sync.RWMutex
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	factory       dynamicinformer.DynamicSharedInformerFactory
	stopCh        chan struct{}
	resources     map[string][]model.K8sResource
}

func NewInformerCache(config *rest.Config) (*InformerCache, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 10*time.Minute)

	cache := &InformerCache{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		factory:       factory,
		stopCh:        make(chan struct{}),
		resources:     make(map[string][]model.K8sResource),
	}

	return cache, nil
}

func (c *InformerCache) Start() {
	c.factory.Start(c.stopCh)
}

func (c *InformerCache) Stop() {
	close(c.stopCh)
}

func (c *InformerCache) ListResources(namespace, resourceType string) []model.K8sResource {
	key := namespace + "/" + resourceType

	c.mu.RLock()
	cached, ok := c.resources[key]
	c.mu.RUnlock()
	if ok {
		return cached
	}

	gvr := schema.GroupVersionResource{
		Resource: resourceType,
	}

	informer := c.factory.ForResource(gvr)
	lister := informer.Lister()

	items, err := lister.ByNamespace(namespace).List(nil)
	if err != nil {
		return nil
	}

	var result []model.K8sResource
	for _, obj := range items {
		u, ok := obj.(interface {
			GetName() string
			GetNamespace() string
			GetLabels() map[string]string
		})
		if !ok {
			continue
		}
		result = append(result, model.K8sResource{
			Name:      u.GetName(),
			Namespace: u.GetNamespace(),
			Type:      resourceType,
			Labels:    u.GetLabels(),
		})
	}

	c.mu.Lock()
	c.resources[key] = result
	c.mu.Unlock()

	return result
}

func (c *InformerCache) Invalidate(namespace, resourceType string) {
	key := namespace + "/" + resourceType
	c.mu.Lock()
	delete(c.resources, key)
	c.mu.Unlock()
}
