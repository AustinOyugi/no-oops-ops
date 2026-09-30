package local

import "github.com/AustinOyugi/no-oops-ops/internal/templateutil"

func renderTemplate(name, source string, data any) ([]byte, error) {
	return templateutil.Render(name, source, data)
}
