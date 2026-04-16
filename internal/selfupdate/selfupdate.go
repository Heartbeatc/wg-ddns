package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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

// Run downloads a prebuilt binary for the current platform and replaces the running binary.
// If no matching binary release exists and Go is installed locally, it falls back to source build.
func Run(stdout io.Writer, opts Options) error {
	targetPath, err := detectTargetPath()
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "当前二进制: %s\n", targetPath)
	fmt.Fprintf(stdout, "更新来源:   %s/%s@%s\n", opts.owner(), opts.repo(), displayRef(opts.ref()))
	fmt.Fprintln(stdout)

	if err := checkWritable(targetPath); err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "wgstack-update-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	newBinary := filepath.Join(tmpDir, "wgstack")
	if err := downloadPrebuilt(stdout, opts, newBinary); err != nil {
		fmt.Fprintf(stdout, "预编译二进制不可用：%v\n", err)
		if requireCmd("go") != nil {
			return fmt.Errorf("当前平台没有可用的预编译二进制，且本机未安装 Go；请稍后重试或使用已发布的 Release")
		}
		fmt.Fprintln(stdout, "回退到源码编译...")
		if err := buildFromSource(stdout, opts, tmpDir, newBinary); err != nil {
			return err
		}
	}

	fmt.Fprintln(stdout, "替换二进制...")
	if err := replaceBinary(targetPath, newBinary); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "\n更新完成: %s\n", targetPath)
	return nil
}

func releaseTagForRef(ref string) string {
	if ref == "" || ref == DefaultRef {
		return "edge"
	}
	return ref
}

func displayRef(ref string) string {
	if ref == "" || ref == DefaultRef {
		return "main/latest"
	}
	return ref
}

func normalizedPlatform() (string, string, error) {
	var goos string
	switch runtime.GOOS {
	case "linux", "darwin":
		goos = runtime.GOOS
	default:
		return "", "", fmt.Errorf("暂不支持的平台: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var arch string
	switch runtime.GOARCH {
	case "amd64", "arm64":
		arch = runtime.GOARCH
	default:
		return "", "", fmt.Errorf("暂不支持的架构: %s", runtime.GOARCH)
	}
	return goos, arch, nil
}

func assetName(tag, goos, arch string) string {
	return fmt.Sprintf("wgstack_%s_%s_%s.tar.gz", tag, goos, arch)
}

func releaseURL(owner, repo, tag, asset string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, tag, asset)
}

func downloadPrebuilt(stdout io.Writer, opts Options, target string) error {
	goos, arch, err := normalizedPlatform()
	if err != nil {
		return err
	}

	tag := releaseTagForRef(opts.ref())
	asset := assetName(tag, goos, arch)
	url := releaseURL(opts.owner(), opts.repo(), tag, asset)

	fmt.Fprintf(stdout, "下载预编译二进制: %s/%s (%s/%s)\n", opts.repo(), displayRef(opts.ref()), goos, arch)
	resp, err := releaseHTTPClient().Get(url)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	if err := extractBinaryTarGz(resp.Body, "wgstack", target); err != nil {
		return fmt.Errorf("解压二进制失败: %w", err)
	}
	if err := os.Chmod(target, 0o755); err != nil {
		return fmt.Errorf("设置二进制权限失败: %w", err)
	}
	return nil
}

func buildFromSource(stdout io.Writer, opts Options, tmpDir, newBinary string) error {
	requireCmd("curl")
	requireCmd("tar")

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
	buildCmd := exec.Command("go", "build", "-o", newBinary, "./cmd/wgstack")
	buildCmd.Dir = srcDir
	buildCmd.Env = append(os.Environ(), "GO111MODULE=on", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	buildCmd.Stdout = stdout
	buildCmd.Stderr = stdout
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("编译失败: %w", err)
	}
	return nil
}

func releaseHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func extractBinaryTarGz(r io.Reader, binaryName, target string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != binaryName {
			continue
		}
		f, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("压缩包中未找到 %s", binaryName)
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

	mode := os.FileMode(0o755)
	if info != nil {
		mode = info.Mode()
	}

	newData, err := os.ReadFile(newBinary)
	if err != nil {
		return fmt.Errorf("读取新二进制失败: %w", err)
	}

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
