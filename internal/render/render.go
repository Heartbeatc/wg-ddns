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
	if err := checkRequiredKeys(project); err != nil {
		return nil, err
	}

	files := []struct {
		template string
		path     string
	}{
		{template: "templates/entry-wg0.conf.tmpl", path: "out/entry/wg0.conf"},
		{template: "templates/exit-wg0.conf.tmpl", path: "out/exit/wg0.conf"},
		{template: "templates/exit-sing-box.json.tmpl", path: "out/exit/sing-box.json"},
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

func checkRequiredKeys(p model.Project) error {
	for _, kv := range []struct{ label, key string }{
		{"入口节点 WireGuard 私钥", p.Nodes.US.WGPrivateKey},
		{"入口节点 WireGuard 公钥", p.Nodes.US.WGPublicKey},
		{"出口节点 WireGuard 私钥", p.Nodes.HK.WGPrivateKey},
		{"出口节点 WireGuard 公钥", p.Nodes.HK.WGPublicKey},
	} {
		if kv.key == "" {
			return fmt.Errorf("渲染配置失败: %s 为空，请运行 wgstack setup 生成密钥", kv.label)
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
