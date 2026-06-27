package model

type View int

const (
	ClusterSelect View = iota
	NamespaceSelect
	ResourceList
	ResourceDetail
	LogView
	YamlView
)

type Cluster struct {
	Name       string
	Kubeconfig string
	Context    string
	Server     string
}

type ResourceType struct {
	Name         string
	Short        string
	Plural       string
	GroupVersion string
}

var ResourceTypes = []ResourceType{
	{"Pods", "po", "pods", "v1"},
	{"Services", "svc", "services", "v1"},
	{"Deployments", "deploy", "deployments", "apps/v1"},
	{"StatefulSets", "sts", "statefulsets", "apps/v1"},
	{"DaemonSets", "ds", "daemonsets", "apps/v1"},
	{"ConfigMaps", "cm", "configmaps", "v1"},
	{"Secrets", "", "secrets", "v1"},
	{"Events", "ev", "events", "v1"},
	{"Nodes", "no", "nodes", "v1"},
	{"Ingresses", "ing", "ingresses", "networking.k8s.io/v1"},
	{"PersistentVolumes", "pv", "persistentvolumes", "v1"},
	{"PersistentVolumeClaims", "pvc", "persistentvolumeclaims", "v1"},
	{"VirtualServices", "vs", "virtualservices", "networking.istio.io/v1beta1"},
	{"Gateways", "gw", "gateways", "networking.istio.io/v1beta1"},
	{"DestinationRules", "dr", "destinationrules", "networking.istio.io/v1beta1"},
	{"ServiceEntries", "se", "serviceentries", "networking.istio.io/v1beta1"},
	{"PeerAuthentications", "pa", "peerauthentications", "security.istio.io/v1beta1"},
	{"RequestAuthentications", "ra", "requestauthentications", "security.istio.io/v1beta1"},
	{"AuthorizationPolicies", "ap", "authorizationpolicies", "security.istio.io/v1beta1"},
	{"EnvoyFilters", "", "envoyfilters", "networking.istio.io/v1alpha3"},
	{"Sidecars", "", "sidecars", "networking.istio.io/v1beta1"},
	{"Telemetries", "", "telemetries", "telemetry.istio.io/v1alpha1"},
	{"WasmPlugins", "", "wasmplugins", "extensions.istio.io/v1alpha1"},
}

type K8sResource struct {
	Name      string
	Namespace string
	Type      string
	Status    string
	Age       string
	Labels    map[string]string
}

type ClusterResource struct {
	Name      string
	Version   string
	Group     string
	Singular  string
	Plural    string
	Namespaced bool
}
