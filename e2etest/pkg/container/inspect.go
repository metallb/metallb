// SPDX-License-Identifier:Apache-2.0

package container

import (
	"encoding/json"
	"fmt"

	"errors"

	"go.universe.tf/e2etest/pkg/executor"
)

type NetworkSettings struct {
	Gateway             string `json:"Gateway"`
	IPAddress           string `json:"IPAddress"`
	IPPrefixLen         int    `json:"IPPrefixLen"`
	IPv6Gateway         string `json:"IPv6Gateway"`
	GlobalIPv6Address   string `json:"GlobalIPv6Address"`
	GlobalIPv6PrefixLen int    `json:"GlobalIPv6PrefixLen"`
}

func Networks(name string) (map[string]NetworkSettings, error) {
	toParse := map[string]NetworkSettings{}
	res, err := executor.Host.Exec(executor.ContainerRuntime, "inspect", "-f", "{{json .NetworkSettings.Networks}}", name)
	if err != nil {
		return toParse, errors.Join(err, fmt.Errorf("Failed to inspect container %s %s", name, res))
	}

	err = json.Unmarshal([]byte(res), &toParse)
	if err != nil {
		return toParse, errors.Join(err, errors.New("failed to parse container inspect"))
	}

	return toParse, nil
}
