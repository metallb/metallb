//go:build arm
// +build arm

// SPDX-License-Identifier:Apache-2.0

package safeconvert

import (
	"fmt"
)

func IntToUInt32(toConvert int) (uint32, error) {
	if toConvert < 0 {
		return 0, fmt.Errorf("trying to convert negative value to uint32: %d", toConvert)
	}
	// No need to check for upper limit on arm as maxInt is lower than maxuint32
	return uint32(toConvert), nil
}
