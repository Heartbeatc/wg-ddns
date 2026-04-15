package selfupdate

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	DefaultOwner = "Heartbeatc"
	DefaultRepo  = "wg-ddns"
	DefaultRef   = "main"
)

type Options struct {
	Owner string
	Repo  string
	Ref   string
}

func (o Options) owner() string {
	if o.Owner != "" {
		return o.Owner
	}
	return DefaultOwner
}

func (o Options) repo() string {
	if o.Repo != "" {
		return o.Repo
	}
	return DefaultRepo
}

func (o Options) ref() string {
	if o.Ref != "" {
		return o.Ref
	}
	return DefaultRef
}

// Run downloads the latest source, compiles it, and replaces the running binary.
func Run(stdout io.Writer, opts Options) error {
	targetPath, err := detectTargetPath()
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "当前二进制: %s\n", targetPath)
	fmt.Fprintf(stdout, "更新来源:   %s/%s@%s\n", opts.owner(), opts.repo(), opts.ref())
	fmt.Fprintln(stdout)

	if err := checkWritable(targetPath); err != nil {
		return err
	}

	requireCmd("curl")
	requireCmd("tar")
	if err := requireCmd("go"); err != nil {
		return fmt.Errorf("需要 Go 编译器（%v）\n如果在服务器上，可使用安装脚本: bash <(curl -Ls https://raw.githubusercontent.com/%s/%s/%s/scripts/install.sh)",
			err, opts.owner(), opts.repo(), opts.ref())
	}

	tmpDir, err := os.MkdirTemp("", "wgstack-update-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archiveURL := fmt.Sprintf("https://codeload.github.com/%s/%s/tar.gz/refs/heads/%s",
		opts.owner(), opts.repo(), opts.ref())

	fmt.Fprintln(stdout, "下载源码...")
	if err := runExternal(stdout, "curl", "-fsSL", archiveURL, "-o", filepath.Join(tmpDir, "src.tar.gz")); err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}

	if err := runExternal(stdout, "tar", "-xzf", filepath.Join(tmpDir, "src.tar.gz"), "-C", tmpDir); err != nil {
		return fmt.Errorf("解压失败: %w", err)
	}

	srcDir, err := findSrcDir(tmpDir, opts.repo())
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, "编译 wgstack...")
	newBinary := filepath.Join(tmpDir, "wgstack")
	buildCmd := exec.Command("go", "build", "-o", newBinary, "./cmd/wgstack")
	buildCmd.Dir = srcDir
	buildCmd.Env = append(os.Environ(), "GO111MODULE=on", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	buildCmd.Stdout = stdout
	buildCmd.Stderr = stdout
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("编译失败: %w", err)
	}

	fmt.Fprintln(stdout, "替换二进制...")
	if err := replaceBinary(targetPath, newBinary); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "\n更新完成: %s\n", targetPath)
	return nil
}

func detectTargetPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法检测当前二进制路径: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe, nil
	}
	return resolved, nil
}

func checkWritable(path string) error {
	dir := filepath.Dir(path)
	testFile := filepath.Join(dir, ".wgstack-update-test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("目标目录不可写: %s\n请使用 sudo 运行，或设置 WGDDNS_INSTALL_DIR 后重新安装", dir)
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

func requireCmd(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("需要 %s 但未找到", name)
	}
	return nil
}

func runExternal(stdout io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	return cmd.Run()
}

func findSrcDir(tmpDir, repo string) (string, error) {
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), repo+"-") {
			return filepath.Join(tmpDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("解压后未找到源码目录")
}

func replaceBinary(target, newBinary string) error {
	info, err := os.Stat(target)
	if err != nil {
		info = nil
	}

	mode := os.FileMode(0755)
	if info != nil {
		mode = info.Mode()
	}

	newData, err := os.ReadFile(newBinary)
	if err != nil {
		return fmt.Errorf("读取新二进制失败: %w", err)
	}

	// Atomic replace: write to temp file in same dir, then rename.
	tmpTarget := target + ".new"
	if err := os.WriteFile(tmpTarget, newData, mode); err != nil {
		return fmt.Errorf("写入失败（权限不足？尝试 sudo）: %w", err)
	}
	if err := os.Rename(tmpTarget, target); err != nil {
		os.Remove(tmpTarget)
		return fmt.Errorf("替换失败: %w", err)
	}
	return nil
}
