//go:build !arm
// +build !arm

// SPDX-License-Identifier:Apache-2.0

package safeconvert

import (
	"fmt"
	"math"
)

func IntToUInt32(toConvert int) (uint32, error) {
	if toConvert < 0 {
		return 0, fmt.Errorf("trying to convert negative value to uint32: %d", toConvert)
	}
	if toConvert > math.MaxUint32 {
		return 0, fmt.Errorf("trying to convert value to uint32: %d, would overflow", toConvert)
	}
	return uint32(toConvert), nil
}
