//go:build darwin
// +build darwin

package vnet

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gravitational/trace"
)

// configureOS configures the host OS according to [cfg]. It is safe to call repeatedly, and it is meant to be
// called with an empty [osConfig] to deconfigure anything necessary before exiting.
func configureOS(ctx context.Context, cfg *osConfig) error {
	// There is no need to remove IPs or the IPv6 route, they will automatically be cleaned up when the
	// process exits and the TUN is deleted.
	if cfg.tunIPv6 != "" {
		slog.With("device", cfg.tunName, "address", cfg.tunIPv6).InfoContext(ctx, "Setting IPv6 address for the TUN device.")
		cmd := exec.CommandContext(ctx, "ifconfig", cfg.tunName, "inet6", cfg.tunIPv6, "prefixlen", "64")
		if err := cmd.Run(); err != nil {
			return trace.Wrap(err, "running %v", cmd.Args)
		}

		slog.InfoContext(ctx, "Setting an IPv6 route for the VNet.")
		cmd = exec.CommandContext(ctx, "route", "add", "-inet6", cfg.tunIPv6, "-prefixlen", "64", "-interface", cfg.tunName)
		if err := cmd.Run(); err != nil {
			return trace.Wrap(err, "running %v", cmd.Args)
		}
	}

	if err := configureDNS(ctx, cfg.dnsAddr, cfg.dnsZones); err != nil {
		return trace.Wrap(err, "configuring DNS")
	}

	return nil
}

const resolverFileComment = "# automatically installed by Teleport VNet"

var resolverPath = filepath.Join("/", "etc", "resolver")

func configureDNS(ctx context.Context, nameserver string, zones []string) error {
	if len(nameserver) == 0 && len(zones) > 0 {
		return trace.BadParameter("empty nameserver with non-empty zones")
	}

	slog.With("nameserver", nameserver, "zones", zones).Debug("Configuring DNS.")
	if err := os.MkdirAll(resolverPath, os.FileMode(0755)); err != nil {
		return trace.Wrap(err, "creating %s", resolverPath)
	}

	managedFiles, err := vnetManagedResolverFiles()
	if err != nil {
		return trace.Wrap(err, "finding VNet managed files in /etc/resolver")
	}
	for _, zone := range zones {
		fileName := filepath.Join(resolverPath, zone)
		delete(managedFiles, fileName)
		contents := resolverFileComment + "\nnameserver " + nameserver
		if err := os.WriteFile(fileName, []byte(contents), 0644); err != nil {
			return trace.Wrap(err, "writing DNS configuration file %s", fileName)
		}
	}
	// Delete stale files.
	for fileName := range managedFiles {
		if err := os.Remove(fileName); err != nil {
			return trace.Wrap(err, "deleting VNet managed file %s", fileName)
		}
	}
	return nil
}

func vnetManagedResolverFiles() (map[string]struct{}, error) {
	entries, err := os.ReadDir(resolverPath)
	if err != nil {
		return nil, trace.Wrap(err, "reading %s", resolverPath)
	}

	matchingFiles := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(resolverPath, entry.Name())
		file, err := os.Open(filePath)
		if err != nil {
			return nil, trace.Wrap(err, "opening %s", filePath)
		}
		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			if resolverFileComment == scanner.Text() {
				matchingFiles[filePath] = struct{}{}
			}
		}
	}
	return matchingFiles, nil
}
