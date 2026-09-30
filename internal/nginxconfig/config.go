// Package nginxconfig builds the single configuration used by managed nginx.
package nginxconfig

import (
	_ "embed"

	"github.com/AustinOyugi/no-oops-ops/internal/templateutil"
)

const ContainerPath = "/etc/noops/nginx/nginx.conf"

//go:embed templates/nginx.conf.tmpl
var mainTemplate string

type Data struct {
	HTTPConfig       string
	CloudflareConfig string
	Includes         []string
	Legacy           bool
}

func Render(data Data) ([]byte, error) {
	return templateutil.Render("nginx.conf.tmpl", mainTemplate, data)
}
