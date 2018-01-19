package ndp

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRouterAdvertisementUnmarshalReservedPrf(t *testing.T) {
	// Assume that unmarshaling sets Prf to medium if reserved value received.
	const want = Medium

	b := []byte{0x0, byte(prfReserved) << 3, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}

	ra := new(RouterAdvertisement)
	if err := ra.unmarshal(b); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Assume that unmarshaling ignores any prefix bits longer than the
	// specified length.
	if diff := cmp.Diff(want, ra.RouterSelectionPreference); diff != "" {
		t.Fatalf("unexpected prf (-want +got):\n%s", diff)
	}
}
