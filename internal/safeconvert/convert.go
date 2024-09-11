// SPDX-License-Identifier:Apache-2.0

package safeconvert

import (
	"fmt"
	"math"
)

func IntToUInt8(toConvert int) (uint8, error) {
	if toConvert < 0 {
		return 0, fmt.Errorf("trying to convert negative value to uint8: %d", toConvert)
	}
	if toConvert > math.MaxUint8 {
		return 0, fmt.Errorf("trying to convert value to uint8: %d, would overflow", toConvert)
	}
	return uint8(toConvert), nil
}

func IntToUInt16(toConvert int) (uint16, error) {
	if toConvert < 0 {
		return 0, fmt.Errorf("trying to convert negative value to uint16: %d", toConvert)
	}
	if toConvert > math.MaxUint16 {
		return 0, fmt.Errorf("trying to convert value to uint16: %d, would overflow", toConvert)
	}
	return uint16(toConvert), nil
}

func Uint32ToInt16(toConvert uint32) (uint16, error) {
	if toConvert > math.MaxUint16 {
		return 0, fmt.Errorf("trying to convert value to uint16: %d, too high", toConvert)
	}
	return uint16(toConvert), nil
}

func IntToInt32(toConvert int) (int32, error) {
	if toConvert < math.MinInt32 {
		return 0, fmt.Errorf("trying to convert value to int32: %d, too low", toConvert)
	}
	if toConvert > math.MaxInt32 {
		return 0, fmt.Errorf("trying to convert value to int32: %d, too high", toConvert)
	}
	return int32(toConvert), nil
}
