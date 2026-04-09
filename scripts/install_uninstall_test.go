package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptDarwinLatest(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	releaseRoot := t.TempDir()
	assetName := "llamasitter-darwin-arm64.tar.gz"
	writeRelease(t, releaseRoot, filepath.Join("latest", "download"), assetName, map[string]fileSpec{
		"LlamaSitter.app/Contents/MacOS/LlamaSitter": {Mode: 0o755, Contents: "#!/bin/sh\necho app\n"},
		"llamasitter": {
			Mode:     0o755,
			Contents: "#!/bin/sh\nif [ \"$1\" = \"version\" ]; then\n  echo 'version\tv1.2.3'\n  exit 0\nfi\nexit 0\n",
		},
		"LICENSE": {Mode: 0o644, Contents: "test license\n"},
	})

	installRoot := t.TempDir()
	appDir := filepath.Join(installRoot, "Applications")
	binDir := filepath.Join(installRoot, "bin")
	cmd := exec.Command("sh", filepath.Join(repoRoot, "install.sh"))
	cmd.Env = append(os.Environ(),
		"LLAMASITTER_OS=darwin",
		"LLAMASITTER_ARCH=arm64",
		"LLAMASITTER_RELEASE_BASE_ROOT=file://"+releaseRoot,
		"LLAMASITTER_APP_DIR="+appDir,
		"LLAMASITTER_BIN_DIR="+binDir,
		"LLAMASITTER_NO_LAUNCH=1",
		"LLAMASITTER_FORCE_NO_SUDO=1",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(appDir, "LlamaSitter.app")); err != nil {
		t.Fatalf("expected installed app bundle: %v", err)
	}
	binary := filepath.Join(binDir, "llamasitter")
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("expected installed binary: %v", err)
	}

	versionOutput, err := exec.Command(binary, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version command failed: %v\n%s", err, versionOutput)
	}
	if !strings.Contains(string(versionOutput), "v1.2.3") {
		t.Fatalf("expected installed version output, got %s", versionOutput)
	}
}

func TestInstallScriptLinuxPinnedVersion(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	releaseRoot := t.TempDir()
	assetName := "llamasitter-linux-amd64.tar.gz"
	writeRelease(t, releaseRoot, filepath.Join("download", "v1.2.3"), assetName, map[string]fileSpec{
		"llamasitter": {
			Mode:     0o755,
			Contents: "#!/bin/sh\nif [ \"$1\" = \"version\" ]; then\n  echo 'version\tv1.2.3'\n  exit 0\nfi\nexit 0\n",
		},
		"LICENSE": {Mode: 0o644, Contents: "test license\n"},
	})

	installRoot := t.TempDir()
	binDir := filepath.Join(installRoot, "bin")
	appDir := filepath.Join(installRoot, "Applications")
	cmd := exec.Command("sh", filepath.Join(repoRoot, "install.sh"))
	cmd.Env = append(os.Environ(),
		"LLAMASITTER_OS=linux",
		"LLAMASITTER_ARCH=amd64",
		"LLAMASITTER_VERSION=1.2.3",
		"LLAMASITTER_RELEASE_BASE_ROOT=file://"+releaseRoot,
		"LLAMASITTER_APP_DIR="+appDir,
		"LLAMASITTER_BIN_DIR="+binDir,
		"LLAMASITTER_FORCE_NO_SUDO=1",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "LlamaSitter CLI installed successfully.") {
		t.Fatalf("expected linux install output, got:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(binDir, "llamasitter")); err != nil {
		t.Fatalf("expected installed binary: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "LlamaSitter.app")); !os.IsNotExist(err) {
		t.Fatalf("expected no macOS app on linux install")
	}
}

func TestInstallScriptUnsupportedOS(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	cmd := exec.Command("sh", filepath.Join(repoRoot, "install.sh"))
	cmd.Env = append(os.Environ(),
		"LLAMASITTER_OS=windows",
		"LLAMASITTER_ARCH=amd64",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected unsupported OS failure")
	}
	if !strings.Contains(string(output), "Windows is not supported") {
		t.Fatalf("unexpected unsupported OS output:\n%s", output)
	}
}

func TestUninstallScriptPreservesDataByDefaultChoice(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "Applications")
	binDir := filepath.Join(root, "bin")
	dataDir := filepath.Join(root, "AppSupport")
	logDir := filepath.Join(root, "Logs")
	mkdirFile(t, filepath.Join(appDir, "LlamaSitter.app", "Contents", "MacOS", "LlamaSitter"), "#!/bin/sh\n")
	mkdirFile(t, filepath.Join(binDir, "llamasitter"), "#!/bin/sh\n")
	mkdirFile(t, filepath.Join(dataDir, "llamasitter.yaml"), "config\n")
	mkdirFile(t, filepath.Join(logDir, "app.log"), "log\n")

	cmd := exec.Command("sh", filepath.Join(repoRoot, "uninstall.sh"))
	cmd.Env = append(os.Environ(),
		"LLAMASITTER_OS=darwin",
		"LLAMASITTER_APP_DIR="+appDir,
		"LLAMASITTER_BIN_DIR="+binDir,
		"LLAMASITTER_APP_SUPPORT_DIR="+dataDir,
		"LLAMASITTER_LOG_DIR="+logDir,
		"LLAMASITTER_YES=1",
		"LLAMASITTER_PURGE_DATA=0",
		"LLAMASITTER_SKIP_STOP=1",
		"LLAMASITTER_FORCE_NO_SUDO=1",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("uninstall failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(filepath.Join(appDir, "LlamaSitter.app")); !os.IsNotExist(err) {
		t.Fatalf("expected app bundle to be removed")
	}
	if _, err := os.Stat(filepath.Join(binDir, "llamasitter")); !os.IsNotExist(err) {
		t.Fatalf("expected binary to be removed")
	}
	if _, err := os.Stat(dataDir); err != nil {
		t.Fatalf("expected data dir to remain: %v", err)
	}
	if _, err := os.Stat(logDir); err != nil {
		t.Fatalf("expected log dir to remain: %v", err)
	}
}

func TestUninstallScriptPurgesData(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	root := t.TempDir()
	appDir := filepath.Join(root, "Applications")
	binDir := filepath.Join(root, "bin")
	configDir := filepath.Join(root, "config")
	legacyDir := filepath.Join(root, "legacy")
	stateDir := filepath.Join(root, "state")
	mkdirFile(t, filepath.Join(binDir, "llamasitter"), "#!/bin/sh\n")
	mkdirFile(t, filepath.Join(configDir, "llamasitter.yaml"), "config\n")
	mkdirFile(t, filepath.Join(legacyDir, "llamasitter.db"), "db\n")
	mkdirFile(t, filepath.Join(stateDir, "logs.txt"), "state\n")

	cmd := exec.Command("sh", filepath.Join(repoRoot, "uninstall.sh"))
	cmd.Env = append(os.Environ(),
		"LLAMASITTER_OS=linux",
		"LLAMASITTER_APP_DIR="+appDir,
		"LLAMASITTER_BIN_DIR="+binDir,
		"LLAMASITTER_CONFIG_DIR="+configDir,
		"LLAMASITTER_LEGACY_DATA_DIR="+legacyDir,
		"LLAMASITTER_STATE_DIR="+stateDir,
		"LLAMASITTER_YES=1",
		"LLAMASITTER_PURGE_DATA=1",
		"LLAMASITTER_SKIP_STOP=1",
		"LLAMASITTER_FORCE_NO_SUDO=1",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("uninstall failed: %v\n%s", err, output)
	}

	for _, path := range []string{
		filepath.Join(binDir, "llamasitter"),
		configDir,
		legacyDir,
		stateDir,
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed", path)
		}
	}
}

func TestPackageReleaseLinuxAndChecksums(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	outputDir := filepath.Join(t.TempDir(), "release")
	cmd := exec.Command("bash", filepath.Join(repoRoot, "scripts", "package-release.sh"),
		"package", "--version", "v1.2.3-test", "--target", "linux/amd64", "--output-dir", outputDir)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"GOPROXY=off",
		"GOCACHE="+filepath.Join(repoRoot, ".gocache"),
		"GOMODCACHE="+filepath.Join(repoRoot, ".gomodcache"),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("package release failed: %v\n%s", err, output)
	}

	archive := filepath.Join(outputDir, "llamasitter-linux-amd64.tar.gz")
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("expected packaged archive: %v", err)
	}

	checksumCmd := exec.Command("bash", filepath.Join(repoRoot, "scripts", "package-release.sh"),
		"checksums", "--output-dir", outputDir)
	checksumCmd.Dir = repoRoot
	checksumOutput, err := checksumCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generate checksums failed: %v\n%s", err, checksumOutput)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "SHA256SUMS")); err != nil {
		t.Fatalf("expected checksum file: %v", err)
	}
}

type fileSpec struct {
	Mode     int64
	Contents string
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func writeRelease(t *testing.T, releaseRoot, subdir, asset string, files map[string]fileSpec) {
	t.Helper()
	targetDir := filepath.Join(releaseRoot, subdir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir release dir: %v", err)
	}

	archivePath := filepath.Join(targetDir, asset)
	writeTarGz(t, archivePath, files)

	sumBytes, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive for checksum: %v", err)
	}
	sum := sha256.Sum256(sumBytes)
	checksumLine := hex.EncodeToString(sum[:]) + "  " + asset + "\n"
	if err := os.WriteFile(filepath.Join(targetDir, "SHA256SUMS"), []byte(checksumLine), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}
}

func writeTarGz(t *testing.T, path string, files map[string]fileSpec) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tar.gz: %v", err)
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, spec := range files {
		body := []byte(spec.Contents)
		header := &tar.Header{
			Name: name,
			Mode: spec.Mode,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("write tar header %s: %v", name, err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatalf("write tar body %s: %v", name, err)
		}
	}
}

func mkdirFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
