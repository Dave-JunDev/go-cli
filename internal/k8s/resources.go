package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/dave/kube-tui/internal/model"
	sigsyaml "sigs.k8s.io/yaml"
)

type ResourceManager struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
	discoveredGVRs map[string]schema.GroupVersionResource
	discoveredNS   map[string]bool
}

func NewResourceManager(clientset *kubernetes.Clientset, restConfig *rest.Config) *ResourceManager {
	return &ResourceManager{
		clientset:      clientset,
		restConfig:     restConfig,
		discoveredGVRs: make(map[string]schema.GroupVersionResource),
		discoveredNS:   make(map[string]bool),
	}
}

func (rm *ResourceManager) ListNamespaces() ([]string, error) {
	nsList, err := rm.clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	names := make([]string, len(nsList.Items))
	for i, ns := range nsList.Items {
		names[i] = ns.Name
	}
	return names, nil
}

func (rm *ResourceManager) ListPods(namespace string) ([]model.K8sResource, error) {
	pods, err := rm.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(pods.Items))
	for i, p := range pods.Items {
		resources[i] = podToResource(p)
	}
	return resources, nil
}

func (rm *ResourceManager) ListServices(namespace string) ([]model.K8sResource, error) {
	svcs, err := rm.clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(svcs.Items))
	for i, s := range svcs.Items {
		resources[i] = model.K8sResource{
			Name:      s.Name,
			Namespace: s.Namespace,
			Type:      "services",
			Status:    string(s.Spec.Type),
			Age:       formatAge(s.CreationTimestamp.Time),
			Labels:    s.Labels,
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListDeployments(namespace string) ([]model.K8sResource, error) {
	deploys, err := rm.clientset.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(deploys.Items))
	for i, d := range deploys.Items {
		status := fmt.Sprintf("%d/%d", d.Status.AvailableReplicas, d.Status.Replicas)
		resources[i] = model.K8sResource{
			Name:      d.Name,
			Namespace: d.Namespace,
			Type:      "deployments",
			Status:    status,
			Age:       formatAge(d.CreationTimestamp.Time),
			Labels:    d.Labels,
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListStatefulSets(namespace string) ([]model.K8sResource, error) {
	sts, err := rm.clientset.AppsV1().StatefulSets(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(sts.Items))
	for i, s := range sts.Items {
		status := fmt.Sprintf("%d/%d", s.Status.AvailableReplicas, s.Status.Replicas)
		resources[i] = model.K8sResource{
			Name:      s.Name,
			Namespace: s.Namespace,
			Type:      "statefulsets",
			Status:    status,
			Age:       formatAge(s.CreationTimestamp.Time),
			Labels:    s.Labels,
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListDaemonSets(namespace string) ([]model.K8sResource, error) {
	ds, err := rm.clientset.AppsV1().DaemonSets(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(ds.Items))
	for i, d := range ds.Items {
		status := fmt.Sprintf("%d/%d", d.Status.NumberAvailable, d.Status.DesiredNumberScheduled)
		resources[i] = model.K8sResource{
			Name:      d.Name,
			Namespace: d.Namespace,
			Type:      "daemonsets",
			Status:    status,
			Age:       formatAge(d.CreationTimestamp.Time),
			Labels:    d.Labels,
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListConfigMaps(namespace string) ([]model.K8sResource, error) {
	cms, err := rm.clientset.CoreV1().ConfigMaps(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(cms.Items))
	for i, cm := range cms.Items {
		resources[i] = model.K8sResource{
			Name:      cm.Name,
			Namespace: cm.Namespace,
			Type:      "configmaps",
			Age:       formatAge(cm.CreationTimestamp.Time),
			Labels:    cm.Labels,
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListSecrets(namespace string) ([]model.K8sResource, error) {
	secrets, err := rm.clientset.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(secrets.Items))
	for i, s := range secrets.Items {
		resources[i] = model.K8sResource{
			Name:      s.Name,
			Namespace: s.Namespace,
			Type:      "secrets",
			Age:       formatAge(s.CreationTimestamp.Time),
			Labels:    s.Labels,
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListEvents(namespace string) ([]model.K8sResource, error) {
	events, err := rm.clientset.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(events.Items))
	for i, e := range events.Items {
		resources[i] = model.K8sResource{
			Name:      e.Name,
			Namespace: e.Namespace,
			Type:      "events",
			Status:    string(e.Type),
			Age:       formatAge(e.CreationTimestamp.Time),
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListNodes() ([]model.K8sResource, error) {
	nodes, err := rm.clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(nodes.Items))
	for i, n := range nodes.Items {
		status := "Ready"
		for _, c := range n.Status.Conditions {
			if c.Type == corev1.NodeReady && c.Status != corev1.ConditionTrue {
				status = "NotReady"
				break
			}
		}
		resources[i] = model.K8sResource{
			Name:   n.Name,
			Type:   "nodes",
			Status: status,
			Age:    formatAge(n.CreationTimestamp.Time),
			Labels: n.Labels,
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListPersistentVolumes() ([]model.K8sResource, error) {
	pvs, err := rm.clientset.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(pvs.Items))
	for i, pv := range pvs.Items {
		resources[i] = model.K8sResource{
			Name:   pv.Name,
			Type:   "persistentvolumes",
			Status: string(pv.Status.Phase),
			Age:    formatAge(pv.CreationTimestamp.Time),
			Labels: pv.Labels,
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListPersistentVolumeClaims(namespace string) ([]model.K8sResource, error) {
	pvcs, err := rm.clientset.CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(pvcs.Items))
	for i, pvc := range pvcs.Items {
		resources[i] = model.K8sResource{
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
			Type:      "persistentvolumeclaims",
			Status:    string(pvc.Status.Phase),
			Age:       formatAge(pvc.CreationTimestamp.Time),
			Labels:    pvc.Labels,
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListIngresses(namespace string) ([]model.K8sResource, error) {
	ingresses, err := rm.clientset.NetworkingV1().Ingresses(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	resources := make([]model.K8sResource, len(ingresses.Items))
	for i, ing := range ingresses.Items {
		resources[i] = model.K8sResource{
			Name:      ing.Name,
			Namespace: ing.Namespace,
			Type:      "ingresses",
			Age:       formatAge(ing.CreationTimestamp.Time),
			Labels:    ing.Labels,
		}
	}
	return resources, nil
}

func (rm *ResourceManager) ListResources(namespace, resourceType string) ([]model.K8sResource, error) {
	switch resourceType {
	case "pods":
		return rm.ListPods(namespace)
	case "services":
		return rm.ListServices(namespace)
	case "deployments":
		return rm.ListDeployments(namespace)
	case "statefulsets":
		return rm.ListStatefulSets(namespace)
	case "daemonsets":
		return rm.ListDaemonSets(namespace)
	case "configmaps":
		return rm.ListConfigMaps(namespace)
	case "secrets":
		return rm.ListSecrets(namespace)
	case "events":
		return rm.ListEvents(namespace)
	case "nodes":
		return rm.ListNodes()
	case "ingresses":
		return rm.ListIngresses(namespace)
	case "persistentvolumes":
		return rm.ListPersistentVolumes()
	case "persistentvolumeclaims":
		return rm.ListPersistentVolumeClaims(namespace)
	default:
		resources, err := rm.ListDynamic(namespace, resourceType)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", resourceType, err)
		}
		return resources, nil
	}
}

func (rm *ResourceManager) ListDynamic(namespace, resourceType string) ([]model.K8sResource, error) {
	gvr, namespaced, err := rm.resolveResource(resourceType)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(rm.restConfig)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	var list *unstructured.UnstructuredList
	if namespaced && namespace != "" {
		list, err = dynamicClient.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	} else {
		list, err = dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", resourceType, err)
	}

	resources := make([]model.K8sResource, len(list.Items))
	for i, item := range list.Items {
		age := ""
		if t := item.GetCreationTimestamp(); !t.IsZero() {
			age = formatAge(t.Time)
		}
		resources[i] = model.K8sResource{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Type:      resourceType,
			Status:    "-",
			Age:       age,
			Labels:    item.GetLabels(),
		}
	}
	return resources, nil
}

var resourceGVRs = map[string]schema.GroupVersionResource{
	"pods":                     {Version: "v1", Resource: "pods"},
	"services":                 {Version: "v1", Resource: "services"},
	"configmaps":               {Version: "v1", Resource: "configmaps"},
	"secrets":                  {Version: "v1", Resource: "secrets"},
	"events":                   {Version: "v1", Resource: "events"},
	"nodes":                    {Version: "v1", Resource: "nodes"},
	"namespaces":               {Version: "v1", Resource: "namespaces"},
	"persistentvolumes":        {Version: "v1", Resource: "persistentvolumes"},
	"persistentvolumeclaims":   {Version: "v1", Resource: "persistentvolumeclaims"},
	"deployments":              {Group: "apps", Version: "v1", Resource: "deployments"},
	"statefulsets":             {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"daemonsets":               {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"ingresses":                {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"virtualservices":          {Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"},
	"gateways":                 {Group: "networking.istio.io", Version: "v1beta1", Resource: "gateways"},
	"destinationrules":         {Group: "networking.istio.io", Version: "v1beta1", Resource: "destinationrules"},
	"serviceentries":           {Group: "networking.istio.io", Version: "v1beta1", Resource: "serviceentries"},
	"peerauthentications":      {Group: "security.istio.io", Version: "v1beta1", Resource: "peerauthentications"},
	"requestauthentications":   {Group: "security.istio.io", Version: "v1beta1", Resource: "requestauthentications"},
	"authorizationpolicies":    {Group: "security.istio.io", Version: "v1beta1", Resource: "authorizationpolicies"},
	"envoyfilters":             {Group: "networking.istio.io", Version: "v1alpha3", Resource: "envoyfilters"},
	"sidecars":                 {Group: "networking.istio.io", Version: "v1beta1", Resource: "sidecars"},
	"telemetries":              {Group: "telemetry.istio.io", Version: "v1alpha1", Resource: "telemetries"},
	"wasmplugins":              {Group: "extensions.istio.io", Version: "v1alpha1", Resource: "wasmplugins"},
}

func (rm *ResourceManager) resolveResource(resourceType string) (schema.GroupVersionResource, bool, error) {
	// Check hardcoded map first
	if gvr, ok := resourceGVRs[resourceType]; ok {
		namespaced := true
		switch resourceType {
		case "nodes", "persistentvolumes", "namespaces":
			namespaced = false
		}
		return gvr, namespaced, nil
	}

	// Check discovered cache
	if gvr, ok := rm.discoveredGVRs[resourceType]; ok {
		return gvr, rm.discoveredNS[resourceType], nil
	}

	// Fall back to discovery
	discovery := rm.clientset.DiscoveryClient
	_, apiResources, err := discovery.ServerGroupsAndResources()
	if err != nil {
		return schema.GroupVersionResource{}, false, fmt.Errorf("discovery: %w", err)
	}

	for _, list := range apiResources {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range list.APIResources {
			if r.Name == resourceType {
				gvr := schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: resourceType,
				}
				rm.discoveredGVRs[resourceType] = gvr
				rm.discoveredNS[resourceType] = r.Namespaced
				return gvr, r.Namespaced, nil
			}
		}
	}

	return schema.GroupVersionResource{}, false, fmt.Errorf("resource type %q not found via discovery", resourceType)
}

func (rm *ResourceManager) GetResourceDescribe(namespace, resourceType, name string) (string, error) {
	gvr, _, err := rm.resolveResource(resourceType)
	if err != nil {
		return "", err
	}

	dynamicClient, err := dynamic.NewForConfig(rm.restConfig)
	if err != nil {
		return "", fmt.Errorf("dynamic client: %w", err)
	}

	var obj *unstructured.Unstructured
	if namespace != "" {
		obj, err = dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		obj, err = dynamicClient.Resource(gvr).Get(context.TODO(), name, metav1.GetOptions{})
	}

	if err != nil {
		return "", fmt.Errorf("get %s/%s: %w", resourceType, name, err)
	}

	lines := formatDescribe(obj, resourceType)
	return strings.Join(lines, "\n"), nil
}

func formatDescribe(obj *unstructured.Unstructured, resourceType string) []string {
	var lines []string

	metadata := obj.Object["metadata"].(map[string]interface{})
	lines = append(lines, fmt.Sprintf("Name:\t%s", metadata["name"]))
	if ns, ok := metadata["namespace"]; ok {
		lines = append(lines, fmt.Sprintf("Namespace:\t%s", ns))
	}
	lines = append(lines, fmt.Sprintf("Type:\t%s", resourceType))

	if uid, ok := metadata["uid"]; ok {
		lines = append(lines, fmt.Sprintf("UID:\t%s", uid))
	}
	if created, ok := metadata["creationTimestamp"]; ok {
		lines = append(lines, fmt.Sprintf("Created:\t%s", created))
	}
	if gen, ok := metadata["generation"]; ok {
		lines = append(lines, fmt.Sprintf("Generation:\t%v", gen))
	}

	if labels, ok := metadata["labels"].(map[string]interface{}); ok && len(labels) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Labels:")
		for k, v := range labels {
			lines = append(lines, fmt.Sprintf("  %s=%s", k, v))
		}
	}

	if annotations, ok := metadata["annotations"].(map[string]interface{}); ok && len(annotations) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Annotations:")
		for k, v := range annotations {
			lines = append(lines, fmt.Sprintf("  %s=%s", k, v))
		}
	}

	// Status
	if status, ok := obj.Object["status"].(map[string]interface{}); ok {
		lines = append(lines, "")
		lines = append(lines, "Status:")
		formatStatusFields(status, "", &lines)
	}

	// Spec
	if spec, ok := obj.Object["spec"].(map[string]interface{}); ok {
		lines = append(lines, "")
		lines = append(lines, "Spec:")
		formatStatusFields(spec, "  ", &lines)
	}

	return lines
}

func formatStatusFields(m map[string]interface{}, indent string, lines *[]string) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			if val != "" {
				*lines = append(*lines, fmt.Sprintf("%s%s:\t%s", indent, k, val))
			}
		case float64:
			*lines = append(*lines, fmt.Sprintf("%s%s:\t%v", indent, k, val))
		case bool:
			*lines = append(*lines, fmt.Sprintf("%s%s:\t%v", indent, k, val))
		case map[string]interface{}:
			*lines = append(*lines, fmt.Sprintf("%s%s:", indent, k))
			formatStatusFields(val, indent+"  ", lines)
		case []interface{}:
			if len(val) > 0 {
				*lines = append(*lines, fmt.Sprintf("%s%s:", indent, k))
				for i, item := range val {
					switch itemVal := item.(type) {
					case map[string]interface{}:
						formatStatusFields(itemVal, indent+"  ", lines)
					default:
						*lines = append(*lines, fmt.Sprintf("%s  %d:\t%v", indent, i, itemVal))
					}
				}
			}
		}
	}
}

func (rm *ResourceManager) DiscoverResourceTypes() ([]model.ResourceType, error) {
	_, apiResources, err := rm.clientset.DiscoveryClient.ServerGroupsAndResources()
	if err != nil {
		return nil, fmt.Errorf("discovery: %w", err)
	}

	typeKey := func(plural string) string { return plural }

	seen := make(map[string]bool)
	var types []model.ResourceType

	for _, list := range apiResources {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for _, r := range list.APIResources {
			if !strings.Contains(r.Name, "/") && !seen[typeKey(r.Name)] {
				seen[typeKey(r.Name)] = true
				name := r.Kind
				if name == "" {
					name = r.SingularName
				}
				if name == "" {
					name = r.Name
				}
				gvStr := gv.Group + "/" + gv.Version
				if gv.Group == "" {
					gvStr = gv.Version
				}

				short := ""
				if len(r.ShortNames) > 0 {
					short = r.ShortNames[0]
				}

				types = append(types, model.ResourceType{
					Name:         name,
					Short:        short,
					Plural:       r.Name,
					GroupVersion: gvStr,
				})
			}
		}
	}
	return types, nil
}

func (rm *ResourceManager) GetResourceYAML(namespace, resourceType, name string) (string, error) {
	gvr, _, err := rm.resolveResource(resourceType)
	if err != nil {
		return "", err
	}

	dynamicClient, err := dynamic.NewForConfig(rm.restConfig)
	if err != nil {
		return "", fmt.Errorf("dynamic client: %w", err)
	}

	var obj interface{}
	if namespace != "" {
		obj, err = dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		obj, err = dynamicClient.Resource(gvr).Get(context.TODO(), name, metav1.GetOptions{})
	}

	if err != nil {
		return "", fmt.Errorf("get %s/%s: %w", resourceType, name, err)
	}

	yamlBytes, err := sigsyaml.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}

	return string(yamlBytes), nil
}

func podToResource(p corev1.Pod) model.K8sResource {
	status := string(p.Status.Phase)
	if len(p.Status.ContainerStatuses) > 0 {
		for _, cs := range p.Status.ContainerStatuses {
			if cs.State.Waiting != nil {
				status = cs.State.Waiting.Reason
			} else if cs.State.Terminated != nil {
				status = cs.State.Terminated.Reason
			} else if cs.Ready {
				status = "Running"
			}
		}
	}

	return model.K8sResource{
		Name:      p.Name,
		Namespace: p.Namespace,
		Type:      "pods",
		Status:    status,
		Age:       formatAge(p.CreationTimestamp.Time),
		Labels:    p.Labels,
	}
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
