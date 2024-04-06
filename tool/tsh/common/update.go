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

package common

import (
	"fmt"
	"os"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"

	"github.com/coreos/go-semver/semver"
)

func update() (bool, error) {
	// TODO(russjones): Hook automatic updates here. Check ping response stored
	// in the Teleport client. If not hit ping again.
	//fmt.Printf("--> %v %v.\n", toolsVersion, client.WebProxyAddr)

	//fmt.Printf("--> toolsVersion: %v\n", toolsVersion)
	fmt.Printf("--> teleport.Version: %v\n", teleport.Version)
	fmt.Printf("--> teleport.SemVersion: %v\n", teleport.SemVersion)

	toolsVersion, err := semver.NewVersion(os.Getenv(toolsVersionEnvVar))
	if err != nil {
		return false, trace.Wrap(err)
	}
	if toolsVersion.Equal(teleport.SemVersion) {
		log.Debugf("TELEPORT_TOOLS_VERSION matches version of running binary: %v.", toolsVersion)
		return false, nil
	}

	return true, nil
}

func reexec() (int, error) {
	return 0, nil
}
