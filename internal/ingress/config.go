package ingress

import (
	"fmt"
	"sort"
	"strings"
)

func RenderConfig(routes []Route) ([]byte, error) {
	byDomain := make(map[string][]Route)
	for _, route := range routes {
		if route.Domain == "" || route.Service == "" || route.Port < 1 || route.Port > 65535 {
			return nil, fmt.Errorf("invalid ingress route for %s/%s", route.Environment, route.App)
		}
		byDomain[route.Domain] = append(byDomain[route.Domain], route)
	}

	var out strings.Builder
	out.WriteString("server {\n  listen 80 default_server;\n  listen [::]:80 default_server;\n  server_name _;\n\n")
	out.WriteString("  location = /__noops/health {\n    add_header Content-Type text/plain;\n    return 200 'ok\\n';\n  }\n\n")
	out.WriteString("  location / { return 404; }\n}\n")

	domains := make([]string, 0, len(byDomain))
	for domain := range byDomain {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	for _, domain := range domains {
		routes := byDomain[domain]
		sort.Slice(routes, func(i, j int) bool { return len(routes[i].PathPrefix) > len(routes[j].PathPrefix) })
		fmt.Fprintf(&out, "\nserver {\n  listen 80;\n  listen [::]:80;\n  server_name %s;\n", domain)
		for _, route := range routes {
			fmt.Fprintf(&out, "\n  location %s {\n", route.PathPrefix)
			fmt.Fprintln(&out, "    resolver 127.0.0.11 valid=10s;")
			fmt.Fprintf(&out, "    set $upstream %s;\n", route.Service)
			fmt.Fprintf(&out, "    proxy_pass http://$upstream:%d$request_uri;\n", route.Port)
			fmt.Fprintln(&out, "    proxy_http_version 1.1;")
			fmt.Fprintln(&out, "    proxy_set_header Host $host;")
			fmt.Fprintln(&out, "    proxy_set_header X-Real-IP $remote_addr;")
			fmt.Fprintln(&out, "    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;")
			fmt.Fprintln(&out, "    proxy_set_header X-Forwarded-Proto $scheme;")
			fmt.Fprintln(&out, "    proxy_set_header Upgrade $http_upgrade;")
			fmt.Fprintln(&out, "    proxy_set_header Connection \"upgrade\";")
			fmt.Fprintln(&out, "  }")
		}
		out.WriteString("}\n")
	}
	return []byte(out.String()), nil
}
