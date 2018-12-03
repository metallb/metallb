package struc

import (
	"encoding/binary"
	"io"
)

// Deprecated. Use PackWithOptions.
func PackWithOrder(w io.Writer, data interface{}, order binary.ByteOrder) error {
	return PackWithOptions(w, data, &Options{Order: order})
}

// Deprecated. Use UnpackWithOptions.
func UnpackWithOrder(r io.Reader, data interface{}, order binary.ByteOrder) error {
	return UnpackWithOptions(r, data, &Options{Order: order})
}
