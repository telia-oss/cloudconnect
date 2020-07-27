package cloudconnect_test

import (
	"errors"
	"net"
	"testing"

	"github.com/telia-oss/cloudconnect"

	"github.com/stretchr/testify/assert"
)

func TestVerifyNoOverlap(t *testing.T) {
	tests := []struct {
		description string
		subnets     []*cloudconnect.CIDR
		expected    error
	}{
		{
			description: "errors if there is an overlap",
			subnets: []*cloudconnect.CIDR{
				{net.IPNet{
					IP:   net.IPv4(100, 96, 0, 0).Mask(net.CIDRMask(16, 32)),
					Mask: net.CIDRMask(16, 32),
				}},
				{net.IPNet{
					IP:   net.IPv4(100, 96, 0, 0).Mask(net.CIDRMask(24, 32)),
					Mask: net.CIDRMask(24, 32),
				}},
			},
			expected: errors.New("100.96.0.0/16 overlaps with 100.96.0.0/24"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			err := cloudconnect.VerifyNoOverlap(tc.subnets)
			assert.Equal(t, tc.expected, err)
		})
	}

}

func TestCIDRIncludes(t *testing.T) {
	tests := []struct {
		description string
		supernet    *cloudconnect.CIDR
		subnet      *cloudconnect.CIDR
	}{
		{
			description: "works",
			supernet: &cloudconnect.CIDR{net.IPNet{
				IP:   net.IPv4(100, 96, 0, 0).Mask(net.CIDRMask(16, 32)),
				Mask: net.CIDRMask(16, 32),
			}},
			subnet: &cloudconnect.CIDR{net.IPNet{
				IP:   net.IPv4(100, 96, 0, 0).Mask(net.CIDRMask(24, 32)),
				Mask: net.CIDRMask(24, 32),
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert.True(t, tc.supernet.Includes(tc.subnet))
			assert.False(t, tc.subnet.Includes(tc.supernet))
		})
	}

}

func TestCIDRSubnet(t *testing.T) {
	tests := []struct {
		description string
		prefix      int
		supernet    *cloudconnect.CIDR
		reserved    []*cloudconnect.CIDR
		expected    *cloudconnect.CIDR
		expectedErr error
	}{
		{
			description: "works",
			prefix:      22,
			supernet: &cloudconnect.CIDR{net.IPNet{
				IP:   net.IPv4(100, 96, 0, 0).Mask(net.CIDRMask(20, 32)),
				Mask: net.CIDRMask(20, 32),
			}},
			reserved: []*cloudconnect.CIDR{
				{net.IPNet{
					IP:   net.IPv4(100, 96, 1, 0).Mask(net.CIDRMask(24, 32)),
					Mask: net.CIDRMask(24, 32),
				}},
				{net.IPNet{
					IP:   net.IPv4(100, 96, 5, 0).Mask(net.CIDRMask(24, 32)),
					Mask: net.CIDRMask(24, 32),
				}},
			},
			expected: &cloudconnect.CIDR{net.IPNet{
				IP:   net.IPv4(100, 96, 8, 0).Mask(net.CIDRMask(22, 32)),
				Mask: net.CIDRMask(22, 32),
			}},
		},
		{
			description: "returns an error if the supernet is full",
			prefix:      22,
			supernet: &cloudconnect.CIDR{net.IPNet{
				IP:   net.IPv4(100, 96, 0, 0).Mask(net.CIDRMask(20, 32)),
				Mask: net.CIDRMask(20, 32),
			}},
			reserved: []*cloudconnect.CIDR{
				{net.IPNet{
					IP:   net.IPv4(100, 96, 0, 0).Mask(net.CIDRMask(21, 32)),
					Mask: net.CIDRMask(21, 32),
				}},
				{net.IPNet{
					IP:   net.IPv4(100, 96, 8, 0).Mask(net.CIDRMask(21, 32)),
					Mask: net.CIDRMask(21, 32),
				}},
			},
			expectedErr: errors.New("no space left in supernet"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			actual, err := tc.supernet.Subnet(tc.prefix, tc.reserved)
			assert.Equal(t, tc.expectedErr, err)
			assert.Equal(t, tc.expected, actual)
		})
	}

}

func TestCIDRAddressCount(t *testing.T) {
	tests := []struct {
		description string
		supernet    *cloudconnect.CIDR
		expected    int
	}{
		{
			description: "works",
			supernet: &cloudconnect.CIDR{net.IPNet{
				IP:   net.IPv4(10, 0, 0, 0).Mask(net.CIDRMask(25, 32)),
				Mask: net.CIDRMask(25, 32),
			}},
			expected: 128,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.supernet.AddressCount())
		})
	}
}
