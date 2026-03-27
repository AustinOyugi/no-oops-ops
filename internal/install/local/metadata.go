package local

type metadata struct {
	Version     string           `json:"version"`
	InstalledAt string           `json:"installed_at"`
	Swarm       swarmMetadata    `json:"swarm"`
	Network     networkMetadata  `json:"network"`
	Registry    registryMetadata `json:"registry"`
}

type swarmMetadata struct {
	Initialized    bool   `json:"initialized"`
	LocalNodeState string `json:"local_node_state"`
	ManagerAddress string `json:"manager_address"`
}

type networkMetadata struct {
	Name string `json:"name"`
}

type registryMetadata struct {
	Name        string `json:"name"`
	Port        string `json:"port"`
	ConfigPath  string `json:"config_path"`
	StackPath   string `json:"stack_path"`
	DataPath    string `json:"data_path"`
	ServiceName string `json:"service_name"`
	Ready       bool   `json:"ready"`
}
