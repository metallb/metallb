// SPDX-License-Identifier:Apache-2.0

package configmaptocrs
import (
	"testing"
)

func FuzzDecodeConfigFile(f *testing.F) {
	f.Fuzz(func(t *testing.T, input []byte) {
		_, _ = decodeConfigFile(input)
	})
}
