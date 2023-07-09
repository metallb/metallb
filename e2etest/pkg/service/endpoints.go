// SPDX-License-Identifier:Apache-2.0

package service

import (
	"fmt"

	"go.universe.tf/e2etest/pkg/executor"
)

// Relies on the endpoint being an agnhost netexec pod.
func GetEndpointHostName(ep string, exec executor.Executor) (string, error) {
	res, err := exec.Exec("wget", "-O-", "-q", fmt.Sprintf("http://%s/hostname", ep))
	if err != nil {
		return "", err
	}

	return res, nil
}
