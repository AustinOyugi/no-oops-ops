package ingress

import _ "embed"

// cloudflareRealIPConfig trusts CF-Connecting-IP only from Cloudflare's
// published proxy networks. It is loaded at nginx's http scope so the real
// client address is used consistently by every generated external route.
//
// Source: https://www.cloudflare.com/ips (retrieved 2026-08-27).
//
//go:embed templates/cloudflare.conf.tmpl
var cloudflareRealIPConfig string
