package tui

import (
	"regexp"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

// Recover ownership of older deployments when this workspace has no local
// deployment history. Only exact Noops naming conventions and unique catalog
// identities are eligible; arbitrary Docker services remain untracked.
func resolveCatalogServices(cfg config.Config, rows []Row) {
	apps, err := catalog.Load(cfg.Workspace)
	if err != nil {
		return
	}
	var identities []string
	for alias := range apps.Apps {
		path, err := catalog.Resolve(cfg.Workspace, alias)
		if err != nil {
			continue
		}
		services, err := manifest.Services(path)
		if err != nil {
			continue
		}
		for _, service := range services {
			m, err := manifest.LoadService(path, service)
			if err == nil {
				identities = append(identities, m.Name)
			}
		}
	}
	for i := range rows {
		if !rows[i].Untracked {
			continue
		}
		var match owner
		count := 0
		for _, name := range identities {
			environment := catalogServiceEnvironment(rows[i].Service, name)
			if environment != "" {
				match = owner{environment, name}
				count++
			}
		}
		if count == 1 {
			rows[i].Environment, rows[i].App, rows[i].Untracked = match.environment, match.app, false
		}
	}
}

func catalogServiceEnvironment(service, identity string) string {
	if identity == "" {
		return ""
	}
	candidate := regexp.MustCompile(`^([a-zA-Z0-9][a-zA-Z0-9-]*)-` + regexp.QuoteMeta(identity) + `-r[a-z0-9-]+_app$`)
	if match := candidate.FindStringSubmatch(service); match != nil {
		return match[1]
	}
	stack, suffix, ok := strings.Cut(service, "_")
	if !ok || suffix != stack {
		return ""
	}
	environment, ok := strings.CutSuffix(stack, "-"+identity)
	if ok && regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`).MatchString(environment) {
		return environment
	}
	return ""
}
