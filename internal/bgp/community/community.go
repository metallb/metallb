// SPDX-License-Identifier:Apache-2.0

package community

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrInvalidCommunityValue  = errors.New("invalid community value")
	ErrInvalidCommunityFormat = errors.New("invalid community format")
)

// largeBGPCommunityMarker is the prefix that shall be used to indicate that a given community value is of type large
// community. If and when extended BGP communities will be supported, the largeBGPCommunityMarker allows us to
// distinguish between extended and large communities.
const largeBGPCommunityMarker = "large"

// BGPCommunity represents a BGP community.
type BGPCommunity interface {
	LessThan(BGPCommunity) bool
	String() string
}

// New parses the provided string and returns a newly created BGPCommunity object.
// Strings are parsed according to Juniper style  syntax (https://www.juniper.net/documentation/us/en/software/\
// junos/routing-policy/bgp/topics/concept/policy-bgp-communities-extended-communities-match-conditions-overview.html
// Legacy communities are of format "<AS number>:<community value>".
// Extended communities (support of which is yet to be implemented) are of format "<type>:<administrator>:<assigned-number>".
// Large communities are of format large:<global administrator>:<localdata part 1>:<localdata part 2>.
func New(c string) (BGPCommunity, error) {
	var bgpCommunity BGPCommunity

	fs := strings.Split(c, ":")
	switch l := len(fs); l {
	case 2:
		var fields [2]uint16
		for i := 0; i < 2; i++ {
			b, err := strconv.ParseUint(fs[i], 10, 16)
			if err != nil {
				return bgpCommunity, fmt.Errorf("%w: invalid section %q of community %q, err: %q",
					ErrInvalidCommunityValue, fs[i], c, err)
			}
			fields[i] = uint16(b)
		}
		return BGPCommunityLegacy{
			upperVal: fields[0],
			lowerVal: fields[1],
		}, nil
	case 4:
		if fs[0] != largeBGPCommunityMarker {
			return bgpCommunity, fmt.Errorf("%w: invalid marker for large community, expected community to be of "+
				"format %s:<uint32>:<uint32>:<uint32> but got %q instead",
				ErrInvalidCommunityValue, largeBGPCommunityMarker, c)
		}
		var fields [3]uint32
		for i := 1; i < 4; i++ {
			b, err := strconv.ParseUint(fs[i], 10, 32)
			if err != nil {
				return bgpCommunity, fmt.Errorf("%w: invalid section %q of community %q, err: %q",
					ErrInvalidCommunityValue, fs[i], c, err)
			}
			fields[i-1] = uint32(b)
		}
		return BGPCommunityLarge{
			globalAdministrator: fields[0],
			localDataPart1:      fields[1],
			localDataPart2:      fields[2],
		}, nil
	}

	return bgpCommunity, fmt.Errorf("%w: %s", ErrInvalidCommunityFormat, c)
}

// BGPCommunityLegacy holds the internal representation of a BGP legacy community.
type BGPCommunityLegacy struct {
	upperVal uint16
	lowerVal uint16
}

// LessThan makes 2 different BGPCommunity objects comparable. For the sake of comparison, legacy communities are
// considered to be large communities in format <legacy community>:0:0 and thus can be compared with large communities.
func (b BGPCommunityLegacy) LessThan(c BGPCommunity) bool {
	return lessThan(b, c)
}

// String returns the string representation of this community. Legacy communities will be printed in new-format,
// "<AS number>:<community value>".
func (b BGPCommunityLegacy) String() string {
	return fmt.Sprintf("%d:%d", b.upperVal, b.lowerVal)
}

// ToUint32 returns the uint32 representation of this legacy community.
func (b BGPCommunityLegacy) ToUint32() uint32 {
	return (uint32(b.upperVal) << 16) + uint32(b.lowerVal)
}

// BGPCommunity holds the internal representation of a large BGP community.
type BGPCommunityLarge struct {
	globalAdministrator uint32
	localDataPart1      uint32
	localDataPart2      uint32
}

// LessThan makes 2 different BGPCommunity objects comparable. For the sake of comparison, legacy communities are
// considered to be large communities in format <legacy community>:0:0 and thus can be compared with large communities.
func (b BGPCommunityLarge) LessThan(c BGPCommunity) bool {
	return lessThan(b, c)
}

// String returns the string representation of this community. Large communities will be printed as
// ("<global administrator>:<localdata part 1>:<localdata part 2>").
func (b BGPCommunityLarge) String() string {
	return fmt.Sprintf("%d:%d:%d", b.globalAdministrator, b.localDataPart1, b.localDataPart2)
}

// IsLegacy returns true if this is a Legacy community.
func IsLegacy(c BGPCommunity) bool {
	_, ok := c.(BGPCommunityLegacy)
	return ok
}

// IsLarge returns true if this is a Legacy community.
func IsLarge(c BGPCommunity) bool {
	_, ok := c.(BGPCommunityLarge)
	return ok
}

// lessThan is a helper function that compares two communities regardless of their type.
func lessThan(b BGPCommunity, c BGPCommunity) bool {
	var bl BGPCommunityLarge
	var cl BGPCommunityLarge
	switch v := b.(type) {
	case BGPCommunityLegacy:
		bl = BGPCommunityLarge{
			globalAdministrator: v.ToUint32(),
			localDataPart1:      0,
			localDataPart2:      0,
		}
	case BGPCommunityLarge:
		bl = v
	}

	switch v := c.(type) {
	case BGPCommunityLegacy:
		cl = BGPCommunityLarge{
			globalAdministrator: v.ToUint32(),
			localDataPart1:      0,
			localDataPart2:      0,
		}
	case BGPCommunityLarge:
		cl = v
	}
	return bl.globalAdministrator < cl.globalAdministrator ||
		(bl.globalAdministrator == cl.globalAdministrator && (bl.localDataPart1 < cl.localDataPart1 ||
			(bl.localDataPart1 == cl.localDataPart1 && bl.localDataPart2 < cl.localDataPart2)))
}
