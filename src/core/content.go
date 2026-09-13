package core

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

// RenderContent expands explicitly marked templates using shared source fragments
// and host-supplied connection text. Missing or recursive references fail closed.
func RenderContent(files fs.FS, name string, host map[string]string) ([]byte, error) {
	active := make(map[string]bool)
	var render func(string) (string, error)
	render = func(name string) (string, error) {
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") || active[name] {
			return "", fmt.Errorf("invalid or recursive content %q", name)
		}
		active[name] = true
		defer delete(active, name)
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return "", fmt.Errorf("content %q: %w", name, err)
		}
		if !strings.HasSuffix(name, ".tmpl") {
			return string(data), nil
		}
		tmpl, err := template.New(name).Option("missingkey=error").Funcs(template.FuncMap{
			"include": render,
			"host": func(key string) (string, error) {
				value, ok := host[key]
				if !ok {
					return "", fmt.Errorf("missing host fragment %q", key)
				}
				return value, nil
			},
		}).Parse(string(data))
		if err != nil {
			return "", err
		}
		var output bytes.Buffer
		if err := tmpl.Execute(&output, nil); err != nil {
			return "", err
		}
		return output.String(), nil
	}
	output, err := render(name)
	if err != nil {
		return nil, err
	}
	if strings.Contains(output, "{{") || strings.Contains(output, "}}") {
		return nil, fmt.Errorf("unexpanded content in %q", name)
	}
	return []byte(output), nil
}
