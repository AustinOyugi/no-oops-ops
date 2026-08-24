package ingress

import (
	"bytes"
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

// RenderFiles renders isolated nginx snippets. External routes are grouped by
// domain because nginx requires all paths for one host to live in one server.
func RenderFiles(routes []Route) (map[string][]byte, error) {
	files := make(map[string][]byte)
	byDomain := make(map[string][]Route)
	for _, route := range routes {
		if route.Domain == "" || route.Service == "" || route.Port < 1 || route.Port > 65535 {
			return nil, fmt.Errorf("invalid ingress route for %s/%s", route.Environment, route.App)
		}
		byDomain[route.Domain] = append(byDomain[route.Domain], route)
	}
	for domain, routesForDomain := range byDomain {
		sort.Slice(routesForDomain, func(i, j int) bool { return len(routesForDomain[i].PathPrefix) > len(routesForDomain[j].PathPrefix) })
		data := configTemplateData{Domains: []domainRoutes{{Domain: domain, Routes: routesForDomain}}}
		content, err := renderTemplate(data, "external")
		if err != nil {
			return nil, err
		}
		files["external/"+safeFileName(domain)+".conf"] = content
	}
	for _, route := range routes {
		data := configTemplateData{InternalRoutes: []internalRoute{{Route: route, Path: "/" + route.Environment + "/" + route.App}}}
		content, err := renderTemplate(data, "internal")
		if err != nil {
			return nil, err
		}
		files["internal/"+safeFileName(route.Environment+"-"+route.App)+".conf"] = content
	}
	return files, nil
}

func safeFileName(value string) string {
	return strings.Map(func(char rune) rune {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' {
			return char
		}
		return '-'
	}, value)
}

func renderTemplate(data configTemplateData, mode string) ([]byte, error) {
	tpl, err := template.New("routes.conf.tmpl").Funcs(template.FuncMap{
		"tlsEnabled": func(routes []Route) bool { return len(routes) > 0 && routes[0].TLS },
	}).Parse(routesConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse nginx routes template: %w", err)
	}
	var out bytes.Buffer
	if err := tpl.ExecuteTemplate(&out, mode, data); err != nil {
		return nil, fmt.Errorf("render nginx routes template: %w", err)
	}
	return out.Bytes(), nil
}

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

	return renderTemplate(data, "all")
}
