package cloudconnect

import (
	"errors"
	"fmt"
	"net"

	"github.com/apparentlymart/go-cidr/cidr"
	"gopkg.in/yaml.v3"
)

// CIDR is a wrapper around net.IPNet that supports YAML (un)marshalling.
type CIDR struct {
	net.IPNet
}

// String for CIDR...
func (c *CIDR) String() string {
	return c.IPNet.String()
}

// MarshalYAML for CIDR...
func (c CIDR) MarshalYAML() (interface{}, error) {
	return c.String(), nil
}

// UnmarshalYAML for CIDR...
func (c *CIDR) UnmarshalYAML(value *yaml.Node) error {
	_, parsed, err := net.ParseCIDR(value.Value)
	if err != nil {
		return fmt.Errorf("parse cidr: %s", err)
	}
	c.IPNet = *parsed
	return nil
}

// Includes checks whether the CIDR includes the given subnet.
func (c *CIDR) Includes(subnet *CIDR) bool {
	first, last := cidr.AddressRange(&subnet.IPNet)
	if c.IPNet.Contains(first) && c.IPNet.Contains(last) {
		return true
	}
	return false
}

// Subnet finds the next available subnet with the desired prefix within the CIDR.
func (c *CIDR) Subnet(prefix int, reserved []*CIDR) (*CIDR, error) {
	supernetPrefix, _ := c.IPNet.Mask.Size()
	newBits := prefix - supernetPrefix
	if newBits < 0 {
		return nil, errors.New("desired prefix exceeds the supernet")
	}

	next, err := cidr.Subnet(&c.IPNet, newBits, 0)
	if err != nil {
		return nil, fmt.Errorf("new subnet: %s", err)
	}

Loop:
	for {
		for _, r := range reserved {
			if err := VerifyNoOverlap([]*CIDR{{IPNet: *next}, r}); err != nil {
				next, _ = cidr.NextSubnet(&r.IPNet, prefix)
				continue Loop
			}
		}
		break Loop
	}

	cidr := &CIDR{IPNet: *next}

	if !c.Includes(cidr) {
		return nil, errors.New("no space left in supernet")
	}
	return cidr, nil
}

// AddressCount returns the number of IP addresses in the CIDR block.
func (c *CIDR) AddressCount() int {
	count := cidr.AddressCount(&c.IPNet)
	return int(count)
}

// VerifyNoOverlap takes a list of subnets and verifies that none of them are overlapping.
// Adapted from: https://github.com/apparentlymart/go-cidr/blob/master/cidr/cidr.go#L126
func VerifyNoOverlap(subnets []*CIDR) error {
	for i, s := range subnets {
		first, last := cidr.AddressRange(&s.IPNet)
		for j := 0; j < len(subnets); j++ {
			if i == j {
				continue
			}
			if subnets[j].Contains(first) || subnets[j].Contains(last) {
				return fmt.Errorf("%s overlaps with %s", s.String(), subnets[j].String())
			}
		}
	}
	return nil
}
