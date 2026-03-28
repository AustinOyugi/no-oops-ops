package status

type ComponentStatus string

const (
	ComponentStatusReady   ComponentStatus = "ready"
	ComponentStatusMissing ComponentStatus = "missing"
)

type Component struct {
	Name    string
	Status  ComponentStatus
	Message string
}

type Metadata struct {
	Version     string `json:"version"`
	InstalledAt string `json:"installed_at"`
	Swarm       struct {
		Initialized    bool   `json:"initialized"`
		LocalNodeState string `json:"local_node_state"`
		ManagerAddress string `json:"manager_address"`
	} `json:"swarm"`
	Network struct {
		Name string `json:"name"`
	} `json:"network"`
	Registry struct {
		Name        string `json:"name"`
		Port        string `json:"port"`
		ConfigPath  string `json:"config_path"`
		StackPath   string `json:"stack_path"`
		DataPath    string `json:"data_path"`
		ServiceName string `json:"service_name"`
		Ready       bool   `json:"ready"`
	} `json:"registry"`
}

type Result struct {
	Metadata   Metadata
	Components []Component
}

func (r *Result) AddComponent(name string, status ComponentStatus, message string) {
	r.Components = append(r.Components, Component{
		Name:    name,
		Status:  status,
		Message: message,
	})
}
