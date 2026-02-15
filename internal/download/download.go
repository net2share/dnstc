// Package download provides binary download and management for dnstc.
package download

import (
	"archive/tar"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/net2share/dnstc/internal/config"
	"github.com/ulikunitz/xz"
)

// Binary identifiers
const (
	BinarySlipstream  = "slipstream-client"
	BinaryDNSTT       = "dnstt-client"
	BinaryShadowsocks = "sslocal"
)

// Release URLs
const (
	SlipstreamReleaseURL  = "https://github.com/net2share/slipstream-rust-build/releases/download/latest"
	DNSTTReleaseURL       = "https://github.com/net2share/dnstt/releases/download/latest"
	ShadowsocksReleaseURL = "https://github.com/shadowsocks/shadowsocks-rust/releases/download/v1.21.2"
)

// BinaryConfig contains configuration for downloading a binary.
type BinaryConfig struct {
	Name         string // Binary name (gost, slipstream-client, etc.)
	ReleaseURL   string
	FilePattern  string // Pattern for binary filename
	ChecksumFile string
	EnvOverride  string // Environment variable to override binary path
}

// Checksums holds checksum information.
type Checksums struct {
	SHA256 string
}

// GetBinaryConfigs returns configurations for all binaries.
func GetBinaryConfigs() map[string]*BinaryConfig {
	arch := detectArch()

	return map[string]*BinaryConfig{
		BinarySlipstream: {
			Name:         BinarySlipstream,
			ReleaseURL:   SlipstreamReleaseURL,
			FilePattern:  fmt.Sprintf("slipstream-client-%s-%s", runtime.GOOS, arch),
			ChecksumFile: "checksums.sha256",
			EnvOverride:  "DNSTC_SLIPSTREAM_PATH",
		},
		BinaryDNSTT: {
			Name:         BinaryDNSTT,
			ReleaseURL:   DNSTTReleaseURL,
			FilePattern:  fmt.Sprintf("dnstt-client-%s-%s", runtime.GOOS, arch),
			ChecksumFile: "checksums.sha256",
			EnvOverride:  "DNSTC_DNSTT_PATH",
		},
		BinaryShadowsocks: {
			Name:         BinaryShadowsocks,
			ReleaseURL:   ShadowsocksReleaseURL,
			FilePattern:  fmt.Sprintf("shadowsocks-v1.21.2.%s-%s.tar.xz", ssArch(), runtime.GOOS),
			ChecksumFile: "shadowsocks-v1.21.2.%s-%s.tar.xz.sha256",
			EnvOverride:  "DNSTC_SSLOCAL_PATH",
		},
	}
}

func detectArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	case "arm":
		return "armv7"
	case "386":
		return "386"
	default:
		return runtime.GOARCH
	}
}

func ssArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

// GetBinaryPath returns the path to a binary, checking env override and system paths first.
func GetBinaryPath(name string) string {
	configs := GetBinaryConfigs()
	cfg, ok := configs[name]
	if !ok {
		return ""
	}

	// Check environment override
	if envPath := os.Getenv(cfg.EnvOverride); envPath != "" {
		return envPath
	}

	// Check common system paths
	systemPaths := []string{
		"/usr/local/bin/" + name,
		"/usr/local/sbin/" + name,
		"/usr/bin/" + name,
		"/usr/sbin/" + name,
	}
	for _, p := range systemPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Return path in user bin directory
	binDir := config.BinDir()
	return filepath.Join(binDir, name)
}

// IsBinaryInstalled checks if a binary is installed.
func IsBinaryInstalled(name string) bool {
	configs := GetBinaryConfigs()
	cfg, ok := configs[name]
	if !ok {
		return false
	}

	// Check environment override
	if envPath := os.Getenv(cfg.EnvOverride); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return true
		}
	}

	// Check common system paths
	systemPaths := []string{
		"/usr/local/bin/" + name,
		"/usr/local/sbin/" + name,
		"/usr/bin/" + name,
		"/usr/sbin/" + name,
	}
	for _, p := range systemPaths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}

	// Check user bin directory
	binDir := config.BinDir()
	if _, err := os.Stat(filepath.Join(binDir, name)); err == nil {
		return true
	}

	return false
}

// DownloadBinary downloads a binary with progress callback.
func DownloadBinary(name string, progressFn func(downloaded, total int64)) (string, error) {
	configs := GetBinaryConfigs()
	cfg, ok := configs[name]
	if !ok {
		return "", fmt.Errorf("unknown binary: %s", name)
	}

	url := fmt.Sprintf("%s/%s", cfg.ReleaseURL, cfg.FilePattern)

	tmpFile, err := os.CreateTemp("", name+"-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	resp, err := http.Get(url)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	var written int64
	if progressFn != nil {
		written, err = io.Copy(tmpFile, &progressReader{
			reader:     resp.Body,
			total:      resp.ContentLength,
			progressFn: progressFn,
		})
	} else {
		written, err = io.Copy(tmpFile, resp.Body)
	}

	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	if written == 0 {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("downloaded file is empty")
	}

	return tmpFile.Name(), nil
}

type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	progressFn func(downloaded, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)
	if pr.progressFn != nil {
		pr.progressFn(pr.downloaded, pr.total)
	}
	return n, err
}

// FetchChecksums fetches checksums for a binary.
func FetchChecksums(name string) (*Checksums, error) {
	configs := GetBinaryConfigs()
	cfg, ok := configs[name]
	if !ok {
		return nil, fmt.Errorf("unknown binary: %s", name)
	}

	checksums := &Checksums{}
	url := fmt.Sprintf("%s/%s", cfg.ReleaseURL, cfg.ChecksumFile)

	resp, err := http.Get(url)
	if err != nil {
		return checksums, fmt.Errorf("failed to fetch checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return checksums, fmt.Errorf("failed to fetch checksums: %s", resp.Status)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := parts[0]
			filename := parts[1]
			if filename == cfg.FilePattern || strings.HasSuffix(filename, cfg.FilePattern) {
				checksums.SHA256 = hash
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return checksums, fmt.Errorf("failed to parse checksums: %w", err)
	}

	return checksums, nil
}

// VerifyChecksums verifies a file against expected checksums.
func VerifyChecksums(filePath string, expected *Checksums) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	sha256Hash := sha256.New()
	if _, err := io.Copy(sha256Hash, file); err != nil {
		return fmt.Errorf("failed to compute checksum: %w", err)
	}

	sha256Sum := hex.EncodeToString(sha256Hash.Sum(nil))

	if expected.SHA256 != "" && sha256Sum != expected.SHA256 {
		return fmt.Errorf("SHA256 checksum mismatch: expected %s, got %s", expected.SHA256, sha256Sum)
	}

	return nil
}

// InstallBinary installs a binary to the bin directory.
func InstallBinary(tmpPath, name string) error {
	binDir := config.BinDir()
	if err := os.MkdirAll(binDir, 0750); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	destPath := filepath.Join(binDir, name)

	input, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read temp file: %w", err)
	}

	if err := os.WriteFile(destPath, input, 0755); err != nil {
		return fmt.Errorf("failed to write binary: %w", err)
	}

	os.Remove(tmpPath)
	return nil
}

// EnsureBinary ensures a binary is installed, downloading if necessary.
func EnsureBinary(name string, progressFn func(downloaded, total int64)) error {
	if IsBinaryInstalled(name) {
		return nil
	}

	tmpPath, err := DownloadBinary(name, progressFn)
	if err != nil {
		return err
	}

	configs := GetBinaryConfigs()
	cfg := configs[name]

	// Handle tar.xz archives (e.g., sslocal)
	if cfg != nil && strings.HasSuffix(cfg.FilePattern, ".tar.xz") {
		extractedPath, err := extractTarXz(tmpPath, name)
		os.Remove(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to extract %s from archive: %w", name, err)
		}
		return InstallBinary(extractedPath, name)
	}

	return InstallBinary(tmpPath, name)
}

// extractTarXz extracts a specific binary from a tar.xz archive.
func extractTarXz(archivePath, binaryName string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open archive: %w", err)
	}
	defer f.Close()

	xzReader, err := xz.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("failed to create xz reader: %w", err)
	}

	tarReader := tar.NewReader(xzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Look for the binary by name (may be in a subdirectory)
		entryName := filepath.Base(header.Name)
		if entryName == binaryName && header.Typeflag == tar.TypeReg {
			tmpFile, err := os.CreateTemp("", binaryName+"-extracted-*")
			if err != nil {
				return "", fmt.Errorf("failed to create temp file: %w", err)
			}

			if _, err := io.Copy(tmpFile, tarReader); err != nil {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
				return "", fmt.Errorf("failed to extract binary: %w", err)
			}
			tmpFile.Close()
			return tmpFile.Name(), nil
		}
	}

	return "", fmt.Errorf("binary '%s' not found in archive", binaryName)
}

// RemoveBinary removes a binary from the user bin directory.
// Note: This only removes binaries installed by dnstc, not system binaries.
func RemoveBinary(name string) error {
	binDir := config.BinDir()
	binPath := filepath.Join(binDir, name)

	// Only remove from user bin directory, not system paths
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return fmt.Errorf("%s is not installed in user directory", name)
	}

	if err := os.Remove(binPath); err != nil {
		return fmt.Errorf("failed to remove %s: %w", name, err)
	}

	return nil
}
