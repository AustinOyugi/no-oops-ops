package local

import (
	"bytes"
	"fmt"
	"text/template"
)

func renderTemplate(name string, tplByte string, data any) ([]byte, error) {

	tpl, err := template.New(name).Parse(tplByte)
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}

	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("execute template %q: %w", name, err)
	}

	return out.Bytes(), nil
}
