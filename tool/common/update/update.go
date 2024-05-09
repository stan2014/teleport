/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/google/renameio/v2"
	"github.com/google/uuid"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	"github.com/coreos/go-semver/semver"
	log "github.com/sirupsen/logrus"
)

//func Check() (string, bool) {
//	// Check if the user has requested a specific version of client tools.
//	toolsVersion := os.Getenv("TELEPORT_TOOLS_VERSION")
//	switch {
//	case toolsVersion == "off":
//		return "", false
//	case toolsVersion != "":
//		return toolsVersion, true
//	default:
//	}
//
//	// TODO(russjones): Get binary name of os package here to switch to tctl
//	// when needed.
//	path, err := toolPath()
//	if err != nil {
//		// TODO(russjones): Log the error.
//		return "", false
//	}
//
//	// TODO(russjones): Switch to CommandContext here.
//	command := exec.Command(path, "version")
//	command.Env = []string{"TELEPORT_TOOLS_VERSION=off"}
//	output, err := command.Output()
//	if err != nil {
//		// TODO(russjones): Log the error.
//		return "", false
//	}
//
//	scanner := bufio.NewScanner(bytes.NewReader(output))
//	for scanner.Scan() {
//		line := scanner.Text() // Get the current line
//		if !strings.HasPrefix(line, "Teleport") {
//			continue
//		}
//
//		var re = regexp.MustCompile(`(?m)Teleport v(.*) git`)
//		matches := re.FindStringSubmatch(line)
//		if len(matches) != 2 {
//			// TODO(russjones): Log the error.
//			return "", false
//		}
//
//		toolsVersion = matches[1]
//	}
//
//	// TODO(russjones); if the requested version matches what's downloaded, don't
//	// download again.
//	// repro: run the following command twice.
//	//
//	// TELEPORT_TOOLS_VERSION=15.1.0 ./tsh.sh --proxy=localhost --user=rjones --insecure login
//	if toolsVersion == teleport.Version {
//		return "", false
//	}
//
//	return toolsVersion, true
//}

func Download(toolsVersion string) error {
	// If the version of the running binary or the version downloaded to
	// $TELEPORT_HOME/bin is the same as the requested version of client tools,
	// nothing to be done, exit early.
	teleportVersion, err := version()
	if err != nil {
		return trace.Wrap(err)
	}
	if toolsVersion == teleport.Version || toolsVersion == teleportVersion {
		return nil
	}

	// Create $TELEPORT_HOME/bin if it does not exist.
	dir, err := toolsDir()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return trace.Wrap(err)
	}

	// Download and update {tsh, tctl} in $TELEPORT_HOME/bin.
	if err := update(toolsVersion); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

//// Exec.
//err := syscall.Exec("/Users/rjones/Desktop/lock/bin/hello", []string{"/Users/rjones/Desktop/lock/bin/hello", "-print"}, os.Environ())
//if err != nil {
//	return trace.Wrap(err)
//}

// TODO(russjones): fix this so the body can be used twice.
//// Get the expected hash from the hashURL
//expectedHashResp, err := http.Get(url + ".sha256")
//if err != nil {
//	return err
//}
//defer expectedHashResp.Body.Close()
//expectedHash, err := ioutil.ReadAll(expectedHashResp.Body)
//if err != nil {
//	return err
//}

//	var err error
//	switch runtime.GOOS {
//	case constants.DarwinOS:
//		return trace.Wrap(downloadDarwin(toolsVersion))
//	case constants.WindowsOS:
//		return trace.Wrap(downloadWindows(toolsVersion))
//	// Assume a Unix-like OS, probably Linux:
//	default:
//		return trace.Wrap(downloadUnix(toolsVersion))
//	}

// TODO(russjones): Add edition check here as well.
func update(toolsVersion string) error {
	// Lock to allow multiple concurrent {tsh, tctl} to run.
	unlock, err := lock()
	defer unlock()

	// TODO(russjones): Cleanup any partial downloads first.

	// Get platform specific download URLs.
	archiveURL, hashURL, err := urls(toolsVersion, "")
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("Archive download path: %v.", archiveURL)
	fmt.Printf("--> Archive download path: %v.", archiveURL)

	// Download the archive and validate against the hash. Download to a
	// temporary path within $TELEPORT_HOME/bin.
	hash, err := downloadHash(hashURL)
	if err != nil {
		return trace.Wrap(err)
	}
	path, err := downloadArchive(archiveURL, hash)
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.Remove(path)

	// Perform atomic replace so concurrent exec do not fail.
	if err := atomicReplace(path); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// urls returns the URL for the Teleport archive to download. The format is:
// https://cdn.teleport.dev/teleport-{, ent-}v15.3.0-{linux, darwin, windows}-{amd64,arm64,arm,386}-{fips-}bin.tar.gz
func urls(toolsVersion string, toolsEdition string) (string, string, error) {
	var archive string

	switch runtime.GOOS {
	case "darwin":
		//archive = "https://cdn.teleport.dev/teleport-" + toolsVersion + ".pkg"
		archive = "https://cdn.teleport.dev/tsh-" + toolsVersion + ".pkg"
	case "windows":
		archive = "https://cdn.teleport.dev/teleport-v" + toolsVersion + "-windows-amd64-bin.zip"
	case "linux":
		edition := ""
		if toolsEdition == "ent" || toolsEdition == "fips" {
			edition = "ent-"
		}
		fips := ""
		if toolsEdition == "fips" {
			fips = "fips-"
		}

		var b strings.Builder
		b.WriteString("https://cdn.teleport.dev/teleport-")
		if edition != "" {
			b.WriteString(edition)
		}
		b.WriteString("v" + toolsVersion + "-" + runtime.GOOS + "-" + runtime.GOARCH + "-")
		if fips != "" {
			b.WriteString(fips)
		}
		b.WriteString("bin.tar.gz")
		archive = b.String()
	default:
		return "", "", trace.BadParameter("unsupported runtime: %v", runtime.GOOS)
	}

	return archive, archive + ".sha256", nil
}

func downloadHash(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("request failed with: %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Hash is the first 64 bytes of the response.
	return string(body)[0:64], nil
}

func downloadArchive(url string, hash string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("bad status when downloading archive: %v", resp.StatusCode)
	}

	dir, err := toolsDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	f, err := os.CreateTemp(dir, "tmp-")
	if err != nil {
		return "", trace.Wrap(err)
	}
	//defer os.Remove(f.Name())

	// TODO(russjones): Add ability to Ctrl-C cancel here.
	h := sha256.New()
	pw := &progressWriter{n: 0, limit: resp.ContentLength}
	body := io.TeeReader(io.TeeReader(resp.Body, h), pw)

	// It is a little inefficient to download the file to disk and then re-load
	// it into memory to unarchive later, but this is safer as it allows {tsh,
	// tctl} to validate the hash before trying to operate on the archive.
	_, err = io.Copy(f, body)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if fmt.Sprintf("%x", h.Sum(nil)) != hash {
		return "", trace.BadParameter("hash of archive does not match downloaded archive")
	}

	return f.Name(), nil
}

func atomicReplace(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return trace.Wrap(replaceDarwin(path))
	//case "linux":
	//	return trace.Wrap(replaceLinux(path))
	//case "windows":
	//	return trace.Wrap(replaceWindows(path))
	default:
		return trace.BadParameter("unsupported runtime: %v", runtime.GOOS)
	}
}

func replaceDarwin(path string) error {
	pkgutil, err := exec.LookPath("pkgutil")
	if err != nil {
		return trace.Wrap(err)
	}

	dir, err := toolsDir()
	if err != nil {
		return trace.Wrap(err)
	}

	// pkgutil --expand-full NAME.pkg DIRECTORY/
	pkgPath := filepath.Join(dir, uuid.NewString()+"-pkg")
	args := []string{"--expand-full", path, pkgPath}
	fmt.Printf("Running pkgutil %q.\n", args)
	out, err := exec.Command(pkgutil, args...).Output()
	if err != nil {
		log.Debugf("Failed to run pkgutil: %v: %v.", out, err)
		return trace.Wrap(err)
	}

	appPath := filepath.Join(pkgPath, "Payload", "tsh.app")
	tempDir := renameio.TempDir(dir)

	err = filepath.WalkDir(appPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		prefix := filepath.Join(pkgPath, "Payload")
		dest := filepath.Join(dir, strings.TrimPrefix(path, prefix))
		if d.IsDir() {
			if err := os.MkdirAll(dest, 0755); err != nil {
				return trace.Wrap(err)
			}
		} else {
			t, err := renameio.TempFile(tempDir, dest)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := os.Chmod(t.Name(), 0755); err != nil {
				return trace.Wrap(err)
			}
			defer t.Cleanup()

			f, err := os.Open(path)
			if err != nil {
				return trace.Wrap(err)
			}
			if _, err := io.Copy(t, f); err != nil {
				return trace.Wrap(err)
			}
			return t.CloseAtomicallyReplace()
		}

		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

//func replaceLinux() error {
//	tempDir, err := os.MkdirTemp(dir, "*-tar")
//	if err != nil {
//		return "", trace.Wrap(err)
//	}
//	defer os.Remove(tempDir)
//
//	f, err := os.Open(f.Name())
//	if err != nil {
//		return "", trace.Wrap(err)
//	}
//	gzipReader, err := gzip.NewReader(f)
//	if err != nil {
//		return "", trace.Wrap(err)
//	}
//	tarReader := tar.NewReader(gzipReader)
//	for {
//		header, err := tarReader.Next()
//		if err == io.EOF {
//			break
//		}
//		// Skip over any files in the archive that are not {tsh, tctl}.
//		if header.Name != "teleport/tctl" && header.Name != "teleport/tsh" {
//			if _, err := io.Copy(ioutil.Discard, tarReader); err != nil {
//				log.Debugf("Failed to discard %v: %v.", header.Name, err)
//			}
//			continue
//		}
//
//		filename := filepath.Join(tempDir, strings.TrimPrefix(header.Name, "teleport/"))
//		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
//		if err != nil {
//			return "", trace.Wrap(err)
//		}
//		_, err = io.Copy(file, tarReader)
//		if err != nil {
//			return "", trace.Wrap(err)
//		}
//	}
//	return tempDir, nil
//}
//
//func replaceWindows() error {
//	tempDir, err := os.MkdirTemp(dir, "*-zip")
//	if err != nil {
//		return "", trace.Wrap(err)
//	}
//	defer os.Remove(tempDir)
//
//	f, err := os.Open(f.Name())
//	if err != nil {
//		return "", trace.Wrap(err)
//	}
//	zipReader, err := zip.NewReader(f, resp.ContentLength)
//	if err != nil {
//		return "", trace.Wrap(err)
//	}
//
//	for _, r := range zipReader.File {
//		if r.Name != "tsh.exe" {
//			continue
//		}
//
//		rr, err := r.Open()
//		if err != nil {
//			return "", trace.Wrap(err)
//		}
//		defer rr.Close()
//
//		filename := filepath.Join(tempDir, r.Name)
//		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
//		if err != nil {
//			return "", trace.Wrap(err)
//		}
//		_, err = io.Copy(file, rr)
//		if err != nil {
//			return "", trace.Wrap(err)
//		}
//		defer file.Close()
//	}
//	return tempDir, nil
//}

//func Exec() (int, error) {
//	path, err := toolPath()
//	if err != nil {
//		return 0, trace.Wrap(err)
//	}
//
//	//command := os.Args[1]
//	args := os.Args[1:]
//
//	cmd := exec.Command(path, args...)
//	cmd.Env = os.Environ()
//	cmd.Stdin = os.Stdin
//	cmd.Stdout = os.Stdout
//	cmd.Stderr = os.Stderr
//
//	if err := cmd.Run(); err != nil {
//		// TODO(russjones): better error code here
//		fmt.Printf("--> trying to exec err: %v\n", err)
//		return 0, trace.Wrap(err)
//	}
//
//	return cmd.ProcessState.ExitCode(), nil
//}

func lock() (func(), error) {
	// Build the path to the lock file that will be used by flock.
	dir, err := toolsDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lockFile := filepath.Join(dir, ".lock")

	// Create the advisory lock using flock.
	// TODO(russjones): Use os.CreateTemp here?
	lf, err := os.OpenFile(lockFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return nil, trace.Wrap(err)
	}

	return func() {
		if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_UN); err != nil {
			log.Debugf("Failed to unlock file: %v: %v.", lockFile, err)
		}
		if err := os.Remove(lockFile); err != nil {
			log.Debugf("Failed to remove lock file: %v: %v.", lockFile, err)
		}
		if err := lf.Close(); err != nil {
			log.Debugf("Failed to close lock file %v: %v.", lockFile, err)
		}
	}, nil
}

func version() (string, error) {
	// Find the path to the current executable.
	dir, err := toolsDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	executable, err := os.Executable()
	if err != nil {
		return "", trace.Wrap(err)
	}
	path := filepath.Join(dir, filepath.Base(executable))

	// Set a timeout to not let "{tsh, tctl} version" block forever. Allow up
	// to 10 seconds because sometimes MDM tools like Jamf cause a lot of
	// latency in launching binaries.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Execue "{tsh, tctl} version" and pass in TELEPORT_TOOLS_VERSION=off to
	// turn off all automatic updates code paths to prevent any recursion.
	command := exec.CommandContext(ctx, path, "version")
	command.Env = []string{teleportToolsVersion + "=off"}
	output, err := command.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}

	// The output for "{tsh, tctl} version" can be multiple lines. Find the
	// actual version line and extract the version.
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "Teleport") {
			continue
		}

		matches := pattern.FindStringSubmatch(line)
		if len(matches) != 2 {
			return "", trace.BadParameter("invalid version line: %v", line)
		}
		version, err := semver.NewVersion(matches[1])
		if err != nil {
			return "", trace.Wrap(err)
		}
		return version.String(), nil
	}

	return "", trace.BadParameter("unable to determine version")
}

// toolsDir returns the path to {tsh, tctl} in $TELEPORT_HOME/bin.
func toolsDir() (string, error) {
	home := os.Getenv(types.HomeEnvVar)
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	//executablePath, err := os.Executable()
	//if err != nil {
	//	return "", trace.Wrap(err)
	//}
	//toolName := filepath.Base(executablePath)

	return filepath.Join(filepath.Clean(home), ".tsh", "bin"), nil
}

//func update() (bool, error) {
//	// TODO(russjones): Hook automatic updates here. Check ping response stored
//	// in the Teleport client. If not hit ping again.
//	//fmt.Printf("--> %v %v.\n", toolsVersion, client.WebProxyAddr)
//
//	//fmt.Printf("--> toolsVersion: %v\n", toolsVersion)
//	fmt.Printf("--> teleport.Version: %v\n", teleport.Version)
//	fmt.Printf("--> teleport.SemVersion: %v\n", teleport.SemVersion)
//
//	//toolsVersion, err := semver.NewVersion(os.Getenv(toolsVersionEnvVar))
//	//if err != nil {
//	//	return trace.Wrap(err)
//	//}
//	//if toolsVersion.Equal(teleport.SemVersion) {
//	//	log.Debugf("TELEPORT_TOOLS_VERSION matches version of running binary: %v.", toolsVersion)
//	//	return nil
//	//}
//
//	return false, nil
//}
//
//func reexec() (int, error) {
//	return 0, nil
//}

type progressWriter struct {
	n     int64
	limit int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	if w.n == 0 {
		fmt.Printf("\n")
	}

	w.n = w.n + int64(len(p))

	n := int((w.n*100)/w.limit) / 10
	bricks := strings.Repeat("▒", n) + strings.Repeat(" ", 10-n)
	fmt.Printf("\rUpdate progress: [" + bricks + "] (Ctrl-C to cancel update)")

	if w.n == w.limit {
		fmt.Printf("\n")
	}

	return len(p), nil
}

const (
	teleportToolsVersion = "TELEPORT_TOOLS_VERSION"
)

var (
	pattern = regexp.MustCompile(`(?m)Teleport v(.*) git`)
)
