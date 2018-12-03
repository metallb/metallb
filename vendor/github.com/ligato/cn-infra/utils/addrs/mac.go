package addrs

import (
	"bytes"
	"strconv"
)

// MacIntToString converts MAC address string representation xx:xx:xx:xx:xx:xx
func MacIntToString(macInt uint64) string {
	const padding = "000000000000"
	macStr := strconv.FormatInt(int64(macInt), 16)
	padded := padding[:len(padding)-len(macStr)] + macStr
	var buffer bytes.Buffer

	for index, char := range padded {
		buffer.WriteRune(char)
		if (index%2 == 1) && index != len(padded)-1 {
			buffer.WriteRune(':')
		}
	}
	return buffer.String()
}
