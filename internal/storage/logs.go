package storage

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"strconv"
)

// BoundLogs adds bounded defaults without replacing explicit Compose logging.
func BoundLogs(data []byte, p Policy) ([]byte, error) {
	if p.LogMaxSize == "" {
		return data, nil
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("empty stack document")
	}
	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "services" {
			continue
		}
		services := root.Content[i+1]
		for j := 1; j < len(services.Content); j += 2 {
			service := services.Content[j]
			found := false
			for k := 0; k+1 < len(service.Content); k += 2 {
				if service.Content[k].Value == "logging" {
					found = true
					break
				}
			}
			if found {
				continue
			}
			var logging yaml.Node
			content, err := yaml.Marshal(map[string]any{"driver": "json-file", "options": map[string]string{"max-size": p.LogMaxSize, "max-file": strconv.Itoa(p.LogMaxFiles)}})
			if err != nil {
				return nil, err
			}
			if err := yaml.Unmarshal(content, &logging); err != nil {
				return nil, err
			}
			service.Content = append(service.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "logging"}, logging.Content[0])
		}
	}
	return yaml.Marshal(&doc)
}
