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

	//// Exec.
	//err := syscall.Exec("/Users/rjones/Desktop/lock/bin/hello", []string{"/Users/rjones/Desktop/lock/bin/hello", "-print"}, os.Environ())
	//if err != nil {
	//	return trace.Wrap(err)
	//}

	return nil
}

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

	archiveURL, hashURL, err := urls(toolsVersion, "")
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("Archive download path: %v.", archiveURL)

	// Download the archive and validate against the hash. Download to a
	// temporary path within $TELEPORT_HOME/bin.
	hash, err := downloadHash(hashURL)
	if err != nil {
		return trace.Wrap(err)
	}
	dir, err := downloadAndExtract(archiveURL, hash)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("--> dir: %q\n", dir)

	// Update permissions.

	// Perform atomic replace. This ensures that exec will not fail.

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
		archive = "https://cdn.teleport.dev/teleport-" + toolsVersion + "-windows-amd64-bin.zip"
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

func downloadAndExtract(url string, hash string) (string, error) {
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
	defer os.Remove(f.Name())

	// TODO(russjones): Add ability to Ctrl-C cancel here.
	h := sha256.New()
	pw := &progressWriter{n: 0, limit: resp.ContentLength}
	body := io.TeeReader(io.TeeReader(resp.Body, h), pw)
	_, err = io.Copy(f, body)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if fmt.Sprintf("%x", h.Sum(nil)) != hash {
		return "", trace.BadParameter("hash of archive does not match downloaded archive")
	}

	switch runtime.GOOS {
	case "darwin":
		pkgutil, err := exec.LookPath("pkgutil")
		if err != nil {
			return "", trace.Wrap(err)
		}

		// pkgutil --expand-full tsh-14.3.13.pkg tsh-14-pkg/
		pkgPath := filepath.Join(dir, uuid.New())
		out, err := exec.Command(pkgutil, "--expand-full", f.Name(), pkgPath).Output()
		if err != nil {
			log.Debugf("Failed to run pkgutil: %v: %v.", out, err)
			return "", trace.Wrap(err)
		}
	case "linux":
	case "windows":
	default:
		return "", trace.BadParameter("unsupported runtime: %v", runtime.GOOS)
	}

	return f.Name(), nil
}

/*
func downloadDarwin() error {
	//url := fmt.Sprintf("https://cdn.teleport.dev/teleport-v%v-darwin-arm64-bin.tar.gz", toolsVersion)
	//// Create an HTTP client that follows redirects
	//client := &http.Client{
	//	CheckRedirect: func(req *http.Request, via []*http.Request) error {
	//		return nil
	//	},
	//}

	//// TODO(russjones): print progress here.
	//resp, err := client.Get(url)
	//if err != nil {
	//	return err
	//}
	//defer resp.Body.Close()

	//// TODO(russjones): fix this so the body can be used twice.
	////// Get the expected hash from the hashURL
	////expectedHashResp, err := http.Get(url + ".sha256")
	////if err != nil {
	////	return err
	////}
	////defer expectedHashResp.Body.Close()
	////expectedHash, err := ioutil.ReadAll(expectedHashResp.Body)
	////if err != nil {
	////	return err
	////}

	////expectedHashString := strings.TrimSpace(string(expectedHash))
	////parts := strings.Split(expectedHashString, " ")
	////expectedHash = []byte(parts[0])

	////// Check the hash of the file
	////hash := sha256.New()
	////if _, err := io.Copy(hash, resp.Body); err != nil {
	////	return err
	////}
	////if fmt.Sprintf("%x", hash.Sum(nil)) != string(expectedHash) {
	////	return fmt.Errorf("hash mismatch")
	////}

	//// Decompress the file
	//gzipReader, err := gzip.NewReader(resp.Body)
	//if err != nil {
	//	return err
	//}
	//tarReader := tar.NewReader(gzipReader)
	//for {
	//	header, err := tarReader.Next()
	//	if err == io.EOF {
	//		break
	//	}
	//	// TODO(russjones): tbot?
	//	if header.Name != "teleport/tctl" && header.Name != "teleport/tsh" {
	//		if _, err := io.Copy(ioutil.Discard, tarReader); err != nil {
	//			fmt.Printf("--> discard: %v\n")
	//		}
	//		continue
	//	}

	//	filename := filepath.Join(dir, strings.TrimPrefix(header.Name, "teleport/"))
	//	file, err := os.Create(filename)
	//	if err != nil {
	//		return err
	//	}
	//	_, err = io.Copy(file, tarReader)
	//	if err != nil {
	//		return err
	//	}
	//	if err := file.Chmod(os.FileMode(0755)); err != nil {
	//		return err
	//	}
	//	fmt.Printf("--> wrote %v\n", filename)
	//}
	//return nil

	return trace.BadParameter("windows not supported yet")
}

func downloadUnix() error {
	return trace.BadParameter("unix not supported yet")
}

func downloadWindows() error {
	return trace.BadParameter("windows not supported yet")
}

func Exec() (int, error) {
	path, err := toolPath()
	if err != nil {
		return 0, trace.Wrap(err)
	}

	//command := os.Args[1]
	args := os.Args[1:]

	cmd := exec.Command(path, args...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// TODO(russjones): better error code here
		fmt.Printf("--> trying to exec err: %v\n", err)
		return 0, trace.Wrap(err)
	}

	return cmd.ProcessState.ExitCode(), nil
}
*/

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
	w.n = w.n + int64(len(p))

	n := int((w.n*100)/w.limit) / 10
	bricks := strings.Repeat("▒", n) + strings.Repeat(" ", 10-n)
	fmt.Printf("\rUpdate progress: [" + bricks + "] (Ctrl-C to cancel update)")

	return len(p), nil
}

const (
	teleportToolsVersion = "TELEPORT_TOOLS_VERSION"
)

var (
	pattern = regexp.MustCompile(`(?m)Teleport v(.*) git`)
)
