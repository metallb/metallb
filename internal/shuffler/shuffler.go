// SPDX-License-Identifier:Apache-2.0

package shuffler

import (
	"math/rand"
	"reflect"
)

func Shuffle(i interface{}, rnd *rand.Rand) {
	t := reflect.TypeOf(i)
	k := t.Kind()
	if k == reflect.Slice {
		s := reflect.ValueOf(i)
		swap := reflect.Swapper(i)
		rnd.Shuffle(s.Len(), func(i, j int) {
			swap(i, j)
		})

		for i := 0; i < s.Len(); i++ {
			Shuffle(s.Index(i).Interface(), rnd)
		}
		return
	}
	if k == reflect.Struct {
		typ := reflect.TypeOf(i)
		v := reflect.ValueOf(i)
		for i := 0; i < v.NumField(); i++ {
			if !typ.Field(i).IsExported() {
				continue
			}
			Shuffle(v.Field(i).Interface(), rnd)
		}
	}
}
