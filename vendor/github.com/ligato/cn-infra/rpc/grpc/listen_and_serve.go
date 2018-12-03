// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpc

import (
	"fmt"
	"net"
	"os"
	"path"
	"strconv"

	"github.com/ligato/cn-infra/logging"
	"google.golang.org/grpc"
)

// ListenAndServe starts configured listener and serving for clients
func ListenAndServe(cfg *Config, srv *grpc.Server) (netListener net.Listener, err error) {
	switch socketType := cfg.getSocketType(); socketType {
	case "unix", "unixpacket":
		permissions, err := getUnixSocketFilePermissions(cfg.Permission)
		if err != nil {
			return nil, err
		}
		if err := checkUnixSocketFileAndDirectory(cfg.Endpoint, cfg.ForceSocketRemoval); err != nil {
			return nil, err
		}

		netListener, err = net.Listen(socketType, cfg.Endpoint)
		if err != nil {
			return nil, err
		}

		// Set permissions to the socket file
		if err := os.Chmod(cfg.Endpoint, permissions); err != nil {
			return nil, err
		}
	default:
		netListener, err = net.Listen(socketType, cfg.Endpoint)
		if err != nil {
			return nil, err
		}
	}

	go func() {
		err := srv.Serve(netListener)
		// Serve always returns non-nil error
		logging.DefaultLogger.Debugf("GRPC server Serve: %v", err)
	}()

	return netListener, nil
}

// Resolve permissions and return FileMode
func getUnixSocketFilePermissions(permissions int) (os.FileMode, error) {
	if permissions > 0 {
		if permissions > 7777 {
			return 0, fmt.Errorf("incorrect unix socket file/path permission value '%d'", permissions)
		}
		// Convert to correct mode format
		mode, err := strconv.ParseInt(strconv.Itoa(permissions), 8, 32)
		if err != nil {
			return 0, fmt.Errorf("failed to parse socket file permissions %d", permissions)
		}
		return os.FileMode(mode), nil
	}
	return os.ModePerm, nil
}

// Check old socket file/directory of the unix domain socket. Remove old socket file if exists or create the directory
// path if does not exist.
func checkUnixSocketFileAndDirectory(endpoint string, forceRemoval bool) error {
	if _, err := os.Stat(endpoint); err == nil && forceRemoval {
		// Remove old socket file if required
		return os.Remove(endpoint)
	} else if os.IsNotExist(err) {
		// Create the directory
		return os.MkdirAll(path.Dir(endpoint), os.ModePerm)
	}
	return nil
}
