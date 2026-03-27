package install

type metadata struct {
	Version     string        `json:"version"`
	InstalledAt string        `json:"installed_at"`
	Swarm       swarmMetadata `json:"swarm"`
}

type swarmMetadata struct {
	Initialized    bool   `json:"initialized"`
	LocalNodeState string `json:"local_node_state"`
	ManagerAddress string `json:"manager_address"`
}
