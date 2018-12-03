package filedb

import (
	"github.com/ligato/cn-infra/datasync"
)

// Implementation of BytesWatchResp (generic response)
type watchResp struct {
	Op               datasync.Op
	Key              string
	Value, PrevValue []byte
	Rev              int64
}

func (wr *watchResp) GetValue() []byte {
	return wr.Value
}

func (wr *watchResp) GetPrevValue() []byte {
	return wr.PrevValue
}

func (wr *watchResp) GetKey() string {
	return wr.Key
}

func (wr *watchResp) GetChangeType() datasync.Op {
	return wr.Op
}

func (wr *watchResp) GetRevision() (rev int64) {
	return wr.Rev
}
