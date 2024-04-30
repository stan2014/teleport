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
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	"github.com/coreos/go-semver/semver"
)

func Check() (string, bool) {
	// Check if the user has requested a specific version of client tools.
	toolsVersion := os.Getenv("TELEPORT_TOOLS_VERSION")
	switch {
	case toolsVersion == "off":
		return "", false
	case toolsVersion != "":
		return toolsVersion, true
	default:
	}

	// TODO(russjones): Get binary name of os package here to switch to tctl
	// when needed.
	path, err := toolPath()
	if err != nil {
		// TODO(russjones): Log the error.
		return "", false
	}

	// TODO(russjones): Switch to CommandContext here.
	command := exec.Command(path, "version")
	command.Env = []string{"TELEPORT_TOOLS_VERSION=off"}
	output, err := command.Output()
	if err != nil {
		// TODO(russjones): Log the error.
		return "", false
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text() // Get the current line
		if !strings.HasPrefix(line, "Teleport") {
			continue
		}

		var re = regexp.MustCompile(`(?m)Teleport v(.*) git`)
		matches := re.FindStringSubmatch(line)
		if len(matches) != 2 {
			// TODO(russjones): Log the error.
			return "", false
		}

		toolsVersion = matches[1]
	}

	// TODO(russjones); if the requested version matches what's downloaded, don't
	// download again.
	// repro: run the following command twice.
	//
	// TELEPORT_TOOLS_VERSION=15.1.0 ./tsh.sh --proxy=localhost --user=rjones --insecure login
	if toolsVersion == teleport.Version {
		return "", false
	}

	return toolsVersion, true
}

// TODO(russjones): If you specify TELEPORT_TOOLS_VERSION, should that download
// to a temp location and not overide ~/.tsh/bin?
func Download(toolsVersion string, toolsEdition string) error {
	// TODO(russjones): What happens if binary is updated when checking for a
	// lock, does this part need to be under a lock as well?
	// TODO(russjones): Add edition check here as well.
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

	unlock, err := lock()
	defer unlock()

	dir := "/Users/rjones/.tsh/bin"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	url := fmt.Sprintf("https://cdn.teleport.dev/teleport-v%v-darwin-arm64-bin.tar.gz", toolsVersion)
	// Create an HTTP client that follows redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	// TODO(russjones): print progress here.
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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

	//expectedHashString := strings.TrimSpace(string(expectedHash))
	//parts := strings.Split(expectedHashString, " ")
	//expectedHash = []byte(parts[0])

	//// Check the hash of the file
	//hash := sha256.New()
	//if _, err := io.Copy(hash, resp.Body); err != nil {
	//	return err
	//}
	//if fmt.Sprintf("%x", hash.Sum(nil)) != string(expectedHash) {
	//	return fmt.Errorf("hash mismatch")
	//}

	// Decompress the file
	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		// TODO(russjones): tbot?
		if header.Name != "teleport/tctl" && header.Name != "teleport/tsh" {
			if _, err := io.Copy(ioutil.Discard, tarReader); err != nil {
				fmt.Printf("--> discard: %v\n")
			}
			continue
		}

		filename := filepath.Join(dir, strings.TrimPrefix(header.Name, "teleport/"))
		file, err := os.Create(filename)
		if err != nil {
			return err
		}
		_, err = io.Copy(file, tarReader)
		if err != nil {
			return err
		}
		if err := file.Chmod(os.FileMode(0755)); err != nil {
			return err
		}
		fmt.Printf("--> wrote %v\n", filename)
	}
	return nil
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

func version() (string, error) {
	path, err := toolPath()
	if err != nil {
		return "", trace.Wrap(err)
	}

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
		return version, nil
	}

	return trace.BadParameter("unable to determine version")
}

// toolPath returns the path to {tsh, tctl} in $TELEPORT_HOME/bin.
func toolPath() (string, error) {
	home := os.Getenv(types.HomeEnvVar)
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", trace.Wrap(err)
	}
	toolName := filepath.Base(executablePath)

	return filepath.Join(filepath.Clean(home), ".tsh", "bin", toolName), nil
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

const (
	teleportToolsVersion = "TELEPORT_TOOLS_VERSION"
)

var (
	pattern = regexp.MustCompile(`(?m)Teleport v(.*) git`)
)
