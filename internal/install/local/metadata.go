package local

type metadata struct {
	Version     string           `json:"version"`
	InstalledAt string           `json:"installed_at"`
	StateDir    string           `json:"state_dir"`
	DataDir     string           `json:"data_dir"`
	Swarm       swarmMetadata    `json:"swarm"`
	Network     networkMetadata  `json:"network"`
	Registry    registryMetadata `json:"registry"`
	Nginx       nginxMetadata    `json:"nginx"`
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

type nginxMetadata struct {
	Name        string `json:"name"`
	HTTPPort    string `json:"http_port"`
	HTTPSPort   string `json:"https_port"`
	StackPath   string `json:"stack_path"`
	ServiceName string `json:"service_name"`
	Ready       bool   `json:"ready"`
}
