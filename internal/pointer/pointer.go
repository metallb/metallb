// SPDX-License-Identifier:Apache-2.0

package pointer

func Uint32Ptr(n uint32) *uint32 {
	return &n
}

func Int32Ptr(n int32) *int32 {
	return &n
}

func BoolPtr(b bool) *bool {
	return &b
}

func StrPtr(s string) *string {
	return &s
}

func IntPtr(i int) *int {
	return &i
}
