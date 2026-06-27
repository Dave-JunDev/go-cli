package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/client-go/kubernetes"

	"github.com/dave/kube-tui/internal/k8s"
	"github.com/dave/kube-tui/internal/model"
	"github.com/dave/kube-tui/internal/tui/components"
	"github.com/dave/kube-tui/internal/tui/theme"
	"github.com/dave/kube-tui/internal/tui/views"
)

type AppModel struct {
	width  int
	height int

	currentView  model.View
	previousView model.View

	clusterModel   *views.ClusterModel
	namespaceModel *views.NamespaceModel
	resourceModel  *views.ResourceModel
	detailModel    *views.DetailModel
	logModel       *views.LogModel
	yamlModel      *views.YamlModel

	statusBar *components.StatusBar

	cluster     model.Cluster
	namespace   string
	resourceMgr *k8s.ResourceManager
	clientset   *kubernetes.Clientset

	err error
}

func NewAppModel() *AppModel {
	sb := components.NewStatusBar()

	cm := views.NewClusterModel()
	nm := views.NewNamespaceModel()
	rm := views.NewResourceModel()
	dm := views.NewDetailModel()
	lm := views.NewLogModel()
	ym := views.NewYamlModel()

	cm.SetStatusBar(sb)
	nm.SetStatusBar(sb)
	rm.SetStatusBar(sb)
	dm.SetStatusBar(sb)
	lm.SetStatusBar(sb)

	return &AppModel{
		currentView:  model.ClusterSelect,
		clusterModel: cm,
		namespaceModel: nm,
		resourceModel: rm,
		detailModel: dm,
		logModel:    lm,
		yamlModel:   ym,
		statusBar:   sb,
	}
}

func (m *AppModel) Init() tea.Cmd {
	return m.clusterModel.Init()
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch m.currentView {
	case model.ClusterSelect:
		return m.updateClusters(msg)
	case model.NamespaceSelect:
		return m.updateNamespaces(msg)
	case model.ResourceList:
		return m.updateResources(msg)
	case model.ResourceDetail:
		return m.updateDetail(msg)
	case model.LogView:
		return m.updateLogs(msg)
	case model.YamlView:
		return m.updateYaml(msg)
	}

	return m, nil
}

func (m *AppModel) updateClusters(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.clusterModel.Update(msg)

	if sel, ok := msg.(views.ClusterSelectedMsg); ok {
		m.cluster = sel.Cluster
		return m, m.connectToCluster()
	}

	if cm, ok := updated.(*views.ClusterModel); ok {
		m.clusterModel = cm
	}

	return m, cmd
}

type connectedMsg struct{}

func (m *AppModel) connectToCluster() tea.Cmd {
	return func() tea.Msg {
		cs, restCfg, err := k8s.NewClient(m.cluster)
		if err != nil {
			return errMsg{err: fmt.Errorf("connection failed: %w", err)}
		}

		if err := k8s.CheckConnection(cs); err != nil {
			return errMsg{err: fmt.Errorf("cluster unreachable: %w", err)}
		}

		m.clientset = cs
		m.resourceMgr = k8s.NewResourceManager(cs, restCfg)

		m.namespaceModel.SetResourceManager(m.resourceMgr)
		m.namespaceModel.SetCluster(m.cluster)
		m.namespaceModel.ResetView()
		m.namespaceModel.SetSize(m.width, m.height)
		m.resourceModel.SetResourceManager(m.resourceMgr)
		m.resourceModel.SetCluster(m.cluster)

		types, discErr := m.resourceMgr.DiscoverResourceTypes()
		if discErr == nil && len(types) > 0 {
			m.resourceModel.SetResourceTypes(types)
		}
		m.resourceModel.ResetView()
		m.resourceModel.SetSize(m.width, m.height)
		m.detailModel.SetResourceManager(m.resourceMgr)
		m.logModel.SetClientset(cs)

		m.currentView = model.NamespaceSelect
		m.statusBar.SetCluster(m.cluster.Name)
		m.statusBar.SetMode("namespace")
		m.statusBar.ClearError()

		return connectedMsg{}
	}
}

func (m *AppModel) updateNamespaces(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case connectedMsg:
		return m, m.namespaceModel.Init()
	}

	updated, cmd := m.namespaceModel.Update(msg)

	if ns, ok := msg.(views.NamespaceSelectedMsg); ok {
		m.namespace = ns.Namespace
		m.resourceModel.ResetView()
		m.resourceModel.SetNamespace(ns.Namespace)
		m.resourceModel.SetSize(m.width, m.height)
		m.currentView = model.ResourceList
		m.statusBar.SetNamespace(ns.Namespace)
		m.statusBar.SetMode("resources")
		m.statusBar.ClearError()
		return m, nil
	}

	if _, ok := msg.(views.PopViewMsg); ok {
		m.clusterModel.ResetView()
		m.clusterModel.SetSize(m.width, m.height)
		m.currentView = model.ClusterSelect
		m.statusBar.SetMode("clusters")
		m.statusBar.SetNamespace("")
		m.statusBar.ClearError()
		return m, nil
	}

	if nm, ok := updated.(*views.NamespaceModel); ok {
		m.namespaceModel = nm
	}

	return m, cmd
}

func (m *AppModel) updateResources(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.resourceModel.Update(msg)

	if res, ok := msg.(views.ResourceSelectedMsg); ok {
		m.detailModel.SetResource(res.Resource)
		m.detailModel.SetWidth(m.width)
		m.detailModel.SetHeight(m.height)
		m.logModel.SetResource(res.Resource)
		m.currentView = model.ResourceDetail
		m.statusBar.SetMode("detail")
		m.statusBar.SetResource(res.Resource.Name)
		m.statusBar.ClearError()
		return m, m.detailModel.Init()
	}

	if lg, ok := msg.(views.ShowLogMsg); ok {
		m.logModel.SetResource(lg.Resource)
		m.logModel.SetSize(m.width, m.height)
		m.currentView = model.LogView
		m.statusBar.SetMode("logs")
		m.statusBar.ClearError()
		return m, m.logModel.Init()
	}

	if _, ok := msg.(views.PopViewMsg); ok {
		m.namespaceModel.SetSize(m.width, m.height)
		m.currentView = model.NamespaceSelect
		m.statusBar.SetMode("namespace")
		m.statusBar.ClearError()
		return m, nil
	}

	if rm, ok := updated.(*views.ResourceModel); ok {
		m.resourceModel = rm
	}

	return m, cmd
}

func (m *AppModel) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.detailModel.Update(msg)

	if ys, ok := msg.(views.ShowYamlMsg); ok {
		m.yamlModel.SetYAML(ys.YAML, ys.Resource)
		m.yamlModel.SetSize(m.width, m.height)
		m.yamlModel.SetMode("YAML")
		m.currentView = model.YamlView
		m.statusBar.SetMode("yaml")
		m.statusBar.ClearError()
		return m, nil
	}

	if ds, ok := msg.(views.ShowDescribeMsg); ok {
		m.yamlModel.SetYAML(ds.Describe, ds.Resource)
		m.yamlModel.SetSize(m.width, m.height)
		m.yamlModel.SetMode("Describe")
		m.currentView = model.YamlView
		m.statusBar.SetMode("describe")
		m.statusBar.ClearError()
		return m, nil
	}

	if lg, ok := msg.(views.ShowLogMsg); ok {
		m.logModel.SetResource(lg.Resource)
		m.logModel.SetSize(m.width, m.height)
		m.currentView = model.LogView
		m.statusBar.SetMode("logs")
		m.statusBar.ClearError()
		return m, m.logModel.Init()
	}

	if _, ok := msg.(views.PopViewMsg); ok {
		m.resourceModel.SetSize(m.width, m.height)
		m.currentView = model.ResourceList
		m.statusBar.SetMode("resources")
		m.statusBar.ClearError()
		return m, nil
	}

	if dm, ok := updated.(*views.DetailModel); ok {
		m.detailModel = dm
	}

	return m, cmd
}

func (m *AppModel) updateLogs(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.logModel.Update(msg)

	if _, ok := msg.(views.PopViewMsg); ok {
		m.resourceModel.SetSize(m.width, m.height)
		m.currentView = model.ResourceList
		m.statusBar.SetMode("resources")
		m.statusBar.ClearError()
		return m, nil
	}

	if lm, ok := updated.(*views.LogModel); ok {
		m.logModel = lm
	}

	return m, cmd
}

func (m *AppModel) updateYaml(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.yamlModel.Update(msg)

	if _, ok := msg.(views.PopViewMsg); ok {
		m.detailModel.SetWidth(m.width)
		m.detailModel.SetHeight(m.height)
		m.currentView = model.ResourceDetail
		m.statusBar.SetMode("detail")
		m.statusBar.ClearError()
		return m, nil
	}

	if ym, ok := updated.(*views.YamlModel); ok {
		m.yamlModel = ym
	}

	return m, cmd
}

func (m *AppModel) View() string {
	var content string

	switch m.currentView {
	case model.ClusterSelect:
		content = m.clusterModel.View()
	case model.NamespaceSelect:
		content = m.namespaceModel.View()
	case model.ResourceList:
		content = m.resourceModel.View()
	case model.ResourceDetail:
		content = m.detailModel.View()
	case model.LogView:
		content = m.logModel.View()
	case model.YamlView:
		content = m.yamlModel.View()
	}

	sb := m.statusBar.View(theme.StatusBarWidth(m.width))

	return fmt.Sprintf("%s\n%s", content, sb)
}

type errMsg struct {
	err error
}
