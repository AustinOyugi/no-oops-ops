// Package templateutil renders embedded text templates for generated artifacts.
package templateutil

import (
	"bytes"
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"text/template"
)

func Render(name, source string, data any) ([]byte, error) {
	tpl, err := template.New(name).Option("missingkey=error").Parse(source)
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("execute template %q: %w", name, err)
	}
	return out.Bytes(), nil
}

//go:embed templates/environment.env.tmpl
var environmentTemplate string

// Environment preserves raw runtime values or quotes build-context values.
func Environment(values map[string]string, quoted bool) ([]byte, error) {
	type entry struct{ Key, Value string }
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]entry, 0, len(keys))
	for _, key := range keys {
		value := values[key]
		if quoted {
			value = strconv.Quote(value)
		}
		entries = append(entries, entry{key, value})
	}
	return Render("environment.env.tmpl", environmentTemplate, entries)
}

//go:embed templates/Dockerfile.tmpl
var dockerfileTemplate string

// Dockerfile generates the release or secret-wrapper image build instructions.
func Dockerfile(image string, bootstrap bool) ([]byte, error) {
	return Render("Dockerfile.tmpl", dockerfileTemplate, struct {
		Image     string
		Bootstrap bool
	}{image, bootstrap})
}
