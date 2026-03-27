package local

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

func renderTemplate(path string, data any) ([]byte, error) {
	tplBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %q: %w", path, err)
	}

	tpl, err := template.New(path).Parse(string(tplBytes))
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", path, err)
	}

	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("execute template %q: %w", path, err)
	}

	return out.Bytes(), nil
}
