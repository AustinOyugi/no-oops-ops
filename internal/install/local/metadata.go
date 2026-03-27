package local

type metadata struct {
	Version     string          `json:"version"`
	InstalledAt string          `json:"installed_at"`
	Swarm       swarmMetadata   `json:"swarm"`
	Network     networkMetadata `json:"network"`
}

type swarmMetadata struct {
	Initialized    bool   `json:"initialized"`
	LocalNodeState string `json:"local_node_state"`
	ManagerAddress string `json:"manager_address"`
}

type networkMetadata struct {
	Name string `json:"name"`
}
