// Package binaries defines the binary registry for dnstc.
package binaries

import (
	"os"

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
			PinnedVersion: "latest",
			ChecksumURL:   "https://github.com/net2share/slipstream-rust-build/releases/download/{version}/checksums.sha256",
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
			PinnedVersion: "v1.21.2",
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

// AreInstalled returns true if 'dnstc install' has been run.
// It checks for the version manifest file, which is created by the install handler.
func AreInstalled() bool {
	_, err := os.Stat(config.VersionsPath())
	return err == nil
}
