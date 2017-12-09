package farm

import "testing"

var res32 uint32
var res64 uint64
var res64lo, res64hi uint64

// 256-bytes random string
var buf = []byte("RMVx)@MLxH9M.WeGW-ktWwR3Cy1XS.,K~i@n-Y+!!yx4?AB%cM~l/#0=2:BOn7HPipG&o/6Qe<hU;$w1-~bU4Q7N&yk/8*Zz.Yg?zl9bVH/pXs6Bq^VdW#Z)NH!GcnH-UesRd@gDij?luVQ3;YHaQ<~SBm17G9;RWvGlsV7tpe*RCe=,?$nE1u9zvjd+rBMu7_Rg4)2AeWs^aaBr&FkC#rcwQ.L->I+Da7Qt~!C^cB2wq(^FGyB?kGQpd(G8I.A7")

func BenchmarkHash32(b *testing.B) {
	var r uint32
	for i := 0; i < b.N; i++ {
		// record the result to prevent the compiler eliminating the function call
		r = Hash32(buf)
	}
	// store the result to a package level variable so the compiler cannot eliminate the Benchmark itself
	res32 = r
}

func BenchmarkHash64(b *testing.B) {
	var r uint64
	for i := 0; i < b.N; i++ {
		r = Hash64(buf)
	}
	res64 = r
}

func BenchmarkHash128(b *testing.B) {
	var rlo, rhi uint64
	for i := 0; i < b.N; i++ {
		rlo, rhi = Hash128(buf)
	}
	res64lo = rlo
	res64hi = rhi
}
