package secret

import "time"

// Metadata identifies a Docker Swarm secret without persisting its value.
type Metadata struct {
	CreatedAt   time.Time `json:"created_at"`
	Environment string    `json:"environment"`
	Key         string    `json:"key"`
	SwarmName   string    `json:"swarm_name"`
	Version     int       `json:"version"`
}
