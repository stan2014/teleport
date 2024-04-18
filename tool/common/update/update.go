/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
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

	if toolsVersion == teleport.Version {
		return "", false
	}

	return toolsVersion, true
}

// TODO(russjones): If you specify TELEPORT_TOOLS_VERSION, should that download
// to a temp location and not overide ~/.tsh/bin?
func Download(toolsVersion string) error {
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
		return 0, trace.Wrap(err)
	}

	return cmd.ProcessState.ExitCode(), nil
}

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
