package render

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"wg-ddns/internal/address"
	"wg-ddns/internal/model"
)

//go:embed templates/*
var templateFS embed.FS

type File struct {
	Path    string
	Content string
}

func Generate(project model.Project) ([]File, error) {
	files := []struct {
		template string
		path     string
	}{
		{template: "templates/us-wg0.conf.tmpl", path: "out/us/wg0.conf"},
		{template: "templates/hk-wg0.conf.tmpl", path: "out/hk/wg0.conf"},
		{template: "templates/hk-sing-box.json.tmpl", path: "out/hk/sing-box.json"},
	}

	var rendered []File
	for _, file := range files {
		content, err := renderTemplate(file.template, project)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, File{Path: file.path, Content: content})
	}

	return rendered, nil
}

func WriteAll(root string, files []File) error {
	for _, file := range files {
		target := filepath.Join(root, file.Path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func renderTemplate(name string, project model.Project) (string, error) {
	tplData, err := templateFS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", name, err)
	}

	tpl, err := template.New(filepath.Base(name)).Funcs(template.FuncMap{
		"cidrIP":     address.CIDRIP,
		"listenPort": address.Port,
	}).Parse(string(tplData))
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}

	var b bytes.Buffer
	if err := tpl.Execute(&b, project); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return b.String(), nil
}
