package ingress

import (
	"bytes"
	_ "embed"
	"fmt"
	"sort"
	"text/template"
)

//go:embed templates/routes.conf.tmpl
var routesConfigTemplate string

type configTemplateData struct {
	Domains        []domainRoutes
	InternalRoutes []internalRoute
}

type domainRoutes struct {
	Domain string
	Routes []Route
}

type internalRoute struct {
	Route
	Path string
}

func RenderConfig(routes []Route) ([]byte, error) {
	byDomain := make(map[string][]Route)
	for _, route := range routes {
		if route.Domain == "" || route.Service == "" || route.Port < 1 || route.Port > 65535 {
			return nil, fmt.Errorf("invalid ingress route for %s/%s", route.Environment, route.App)
		}
		byDomain[route.Domain] = append(byDomain[route.Domain], route)
	}

	domains := make([]string, 0, len(byDomain))
	for domain := range byDomain {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	data := configTemplateData{
		Domains:        make([]domainRoutes, 0, len(domains)),
		InternalRoutes: make([]internalRoute, 0, len(routes)),
	}
	for _, domain := range domains {
		routesForDomain := byDomain[domain]
		sort.Slice(routesForDomain, func(i, j int) bool {
			return len(routesForDomain[i].PathPrefix) > len(routesForDomain[j].PathPrefix)
		})
		data.Domains = append(data.Domains, domainRoutes{Domain: domain, Routes: routesForDomain})
	}
	for _, route := range routes {
		data.InternalRoutes = append(data.InternalRoutes, internalRoute{
			Route: route,
			Path:  "/" + route.Environment + "/" + route.App,
		})
	}
	sort.Slice(data.InternalRoutes, func(i, j int) bool { return data.InternalRoutes[i].Path < data.InternalRoutes[j].Path })

	tpl, err := template.New("routes.conf.tmpl").Parse(routesConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse nginx routes template: %w", err)
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("render nginx routes template: %w", err)
	}
	return out.Bytes(), nil
}
