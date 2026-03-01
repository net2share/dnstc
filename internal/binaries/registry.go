// Package binaries defines the binary registry for dnstc.
package binaries

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/net2share/dnstc/internal/config"
	"github.com/net2share/go-corelib/binman"
)

// Binary name constants.
const (
	NameSlipstream  = "slipstream-client"
	NameDNSTT       = "dnstt-client"
	NameShadowsocks = "sslocal"
)

// AllNames returns the ordered list of all managed binaries.
func AllNames() []string {
	return []string{NameSlipstream, NameDNSTT, NameShadowsocks}
}

// Defs returns the binary definitions for all managed binaries.
func Defs() map[string]binman.BinaryDef {
	return map[string]binman.BinaryDef{
		NameSlipstream: {
			Name:          NameSlipstream,
			EnvOverride:   "DNSTC_SLIPSTREAM_PATH",
			URLPattern:    "https://github.com/net2share/slipstream-rust-build/releases/download/{version}/slipstream-client-{os}-{arch}",
			PinnedVersion: "v2026.02.22.1",
			ChecksumURL:   "https://github.com/net2share/slipstream-rust-build/releases/download/{version}/SHA256SUMS",
		},
		NameDNSTT: {
			Name:          NameDNSTT,
			EnvOverride:   "DNSTC_DNSTT_PATH",
			URLPattern:    "https://github.com/net2share/dnstt/releases/download/{version}/dnstt-client-{os}-{arch}",
			PinnedVersion: "latest",
			ChecksumURL:   "https://github.com/net2share/dnstt/releases/download/{version}/checksums.sha256",
		},
		NameShadowsocks: {
			Name:          NameShadowsocks,
			EnvOverride:   "DNSTC_SSLOCAL_PATH",
			URLPattern:    "https://github.com/shadowsocks/shadowsocks-rust/releases/download/{version}/shadowsocks-{version}.{ssarch}.tar.xz",
			PinnedVersion: "v1.24.0",
			Archive:       true,
			ChecksumURL:   "https://github.com/shadowsocks/shadowsocks-rust/releases/download/{version}/shadowsocks-{version}.{ssarch}.tar.xz.sha256",
			ArchMappings: map[string]binman.ArchMapping{
				"ssarch": {
					"linux/amd64": "x86_64-unknown-linux-gnu",
					"linux/arm64": "aarch64-unknown-linux-gnu",
					"linux/arm":   "armv7-unknown-linux-gnueabihf",
					"darwin/amd64": "x86_64-apple-darwin",
					"darwin/arm64": "aarch64-apple-darwin",
					"windows/amd64": "x86_64-pc-windows-msvc",
				},
			},
		},
	}
}

// systemPaths are common locations where binaries might be installed.
var systemPaths = []string{
	"/usr/local/bin",
	"/usr/local/sbin",
	"/usr/bin",
	"/usr/sbin",
}

// NewManager creates a binman.Manager configured for dnstc.
func NewManager() *binman.Manager {
	return binman.NewManager(config.BinDir(), binman.WithSystemPaths(systemPaths))
}

// EnvPath returns the local path from the binary's env override variable,
// or empty string if not set or the file doesn't exist.
func EnvPath(def binman.BinaryDef) string {
	if def.EnvOverride != "" {
		if p := os.Getenv(def.EnvOverride); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// CopyToBinDir copies a binary from srcPath into the managed bin directory.
func CopyToBinDir(def binman.BinaryDef, srcPath string) error {
	binDir := config.BinDir()
	if err := os.MkdirAll(binDir, 0750); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", srcPath, err)
	}

	destPath := filepath.Join(binDir, def.Name)
	if err := os.WriteFile(destPath, data, 0755); err != nil {
		return fmt.Errorf("failed to copy %s: %w", def.Name, err)
	}

	return nil
}

// AreInstalled returns true if 'dnstc install' has been run.
// It checks for the version manifest file, which is created by the install handler.
func AreInstalled() bool {
	_, err := os.Stat(config.VersionsPath())
	return err == nil
}
