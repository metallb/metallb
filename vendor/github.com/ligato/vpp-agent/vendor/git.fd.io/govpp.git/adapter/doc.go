// Package adapter provides an interface between govpp core and the VPP. It is responsible for sending
// and receiving binary-encoded data to/from VPP via shared memory.
//
// The default adapter being used for connection with real VPP is called vppapiclient. It is based on the
// communication with the vppapiclient VPP library written in C via CGO.
//
// Apart from the vppapiclient adapter, mock adapter is provided for unit/integration testing where the actual
// communication with VPP is not demanded.
package adapter
