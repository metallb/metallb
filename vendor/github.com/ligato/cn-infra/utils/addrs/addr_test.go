package addrs

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestMacIntToString(t *testing.T) {
	RegisterTestingT(t)

	res := MacIntToString(0)
	Expect(res).To(BeEquivalentTo("00:00:00:00:00:00"))

	res = MacIntToString(255)
	Expect(res).To(BeEquivalentTo("00:00:00:00:00:ff"))
}

func TestParseIPWithPrefix(t *testing.T) {
	RegisterTestingT(t)

	ip, isIpv6, err := ParseIPWithPrefix("127.0.0.1")
	Expect(err).To(BeNil())
	Expect(isIpv6).To(BeFalse())
	Expect(ip.IP.String()).To(BeEquivalentTo("127.0.0.1"))
	maskOnes, maskBits := ip.Mask.Size()
	Expect(maskOnes).To(BeEquivalentTo(32))
	Expect(maskBits).To(BeEquivalentTo(32))

	ip, isIpv6, err = ParseIPWithPrefix("192.168.2.100/24")
	Expect(err).To(BeNil())
	Expect(isIpv6).To(BeFalse())
	Expect(ip.IP.String()).To(BeEquivalentTo("192.168.2.100"))

	ip, isIpv6, err = ParseIPWithPrefix("2001:db9::54")
	Expect(err).To(BeNil())
	Expect(isIpv6).To(BeTrue())
	Expect(ip.IP.String()).To(BeEquivalentTo("2001:db9::54"))
	maskOnes, maskBits = ip.Mask.Size()
	Expect(maskOnes).To(BeEquivalentTo(128))
	Expect(maskBits).To(BeEquivalentTo(128))

	ip, isIpv6, err = ParseIPWithPrefix("2001:db8::68/120")
	Expect(err).To(BeNil())
	Expect(isIpv6).To(BeTrue())
	Expect(ip.IP.String()).To(BeEquivalentTo("2001:db8::68"))
	maskOnes, maskBits = ip.Mask.Size()
	Expect(maskOnes).To(BeEquivalentTo(120))
	Expect(maskBits).To(BeEquivalentTo(128))

	_, _, err = ParseIPWithPrefix("127.0.0.1/abcd")
	Expect(err).NotTo(BeNil())
}

func TestIsIPv6(t *testing.T) {
	RegisterTestingT(t)

	ipv6, err := IsIPv6("192.168.0.1")
	Expect(ipv6).ToNot(BeTrue())
	Expect(err).ToNot(HaveOccurred())

	ipv6, err = IsIPv6("2001:db9::54")
	Expect(ipv6).To(BeTrue())
	Expect(err).ToNot(HaveOccurred())

	ipv6, err = IsIPv6("n.o.t.IP")
	Expect(err).To(HaveOccurred())

	ipv6, err = IsIPv6("n:o:t:IP")
	Expect(err).To(HaveOccurred())

	ipv6, err = IsIPv6("")
	Expect(err).To(HaveOccurred())
}
