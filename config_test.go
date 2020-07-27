package cloudconnect_test

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/telia-oss/cloudconnect"
	"gopkg.in/yaml.v3"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		description          string
		config               string
		expectError          bool
		expectedErrorMessage string
	}{
		{
			description: "works with a valid configuration",
			config: strings.TrimSpace(`
gateways:
  eu-west-1:
    transit_gateway_id: tgw-0123
    route_table_id: tgw-rtb-0123
supernets:
  - description: Carrier grade NAT range outlined in RFC6598.
    cidrs:
      eu-west-1:
        - 100.96.0.0/16
teams:
  - team: Public cloud
    accounts:
      - name: telia-direct-connect-dev
        id: "111122223333"
        cidrs: 
          eu-west-1:
            - 100.96.0.0/24
            `),
			expectError: false,
		},
		{
			description: "errors if account is allocated multiple times",
			config: strings.TrimSpace(`
gateways:
  eu-west-1:
    transit_gateway_id: tgw-0123
    route_table_id: tgw-rtb-0123
supernets:
  - description: Carrier grade NAT range outlined in RFC6598.
    cidrs:
      eu-west-1:
        - 100.96.0.0/16
teams:
  - team: Public cloud
    accounts:
      - name: telia-direct-connect-dev
        id: "111122223333"
        cidrs: 
          eu-west-1:
            - 100.96.0.0/23
      - name: telia-direct-connect-dev
        id: "111122223333"
        cidrs:
          eu-west-1:
            - 100.96.1.0/24
            `),
			expectError:          true,
			expectedErrorMessage: "account has duplicate entries: 111122223333",
		},
		{
			description: "errors if there are overlapping subnets",
			config: strings.TrimSpace(`
gateways:
  eu-west-1:
    transit_gateway_id: tgw-0123
    route_table_id: tgw-rtb-0123
supernets:
  - description: Carrier grade NAT range outlined in RFC6598.
    cidrs:
      eu-west-1:
        - 100.96.0.0/16
teams:
  - team: Public cloud
    accounts:
      - name: telia-direct-connect-dev
        id: "111122223333"
        cidrs: 
          eu-west-1:
            - 100.96.0.0/23
      - name: telia-direct-connect-dev
        id: "333344445555"
        cidrs: 
          eu-west-1:
            - 100.96.1.0/24
            `),
			expectError:          true,
			expectedErrorMessage: "verifying subnets: 100.96.0.0/23 overlaps with 100.96.1.0/24",
		},
		{
			description: "errors if cidr is not in approved networks",
			config: strings.TrimSpace(`
gateways:
  eu-west-1:
    transit_gateway_id: tgw-0123
    route_table_id: tgw-rtb-0123
supernets:
  - description: Carrier grade NAT range outlined in RFC6598.
    cidrs:
      eu-west-1:
        - 100.96.0.0/16
teams:
  - team: Public cloud
    accounts:
      - name: telia-direct-connect-dev
        id: "111122223333"
        cidrs:
          eu-west-1:
            - 10.24.0.0/24
            `),
			expectError:          true,
			expectedErrorMessage: "subnet is not in a known supernet: 10.24.0.0/24",
		},
		{
			description: "errors if the region is not valid",
			config: strings.TrimSpace(`
gateways:
  eu-west-1:
    transit_gateway_id: tgw-0123
    route_table_id: tgw-rtb-0123
supernets:
  - description: Carrier grade NAT range outlined in RFC6598.
    cidrs:
      eu-north-0:
        - 100.96.0.0/16
teams:
  - team: Public cloud
    accounts:
      - name: telia-direct-connect-dev
        id: "111122223333"
        cidrs:
          eu-west-1:
            - 100.96.1.0/24
            `),
			expectError:          true,
			expectedErrorMessage: "supernet 0: unknown region: eu-north-0",
		},
		{
			description: "errors if an allocation region is not valid",
			config: strings.TrimSpace(`
gateways:
  eu-west-1:
    transit_gateway_id: tgw-0123
    route_table_id: tgw-rtb-0123
supernets:
  - description: Carrier grade NAT range outlined in RFC6598.
    cidrs:
      eu-west-1:
        - 100.96.0.0/16
teams:
  - team: Public cloud
    accounts:
      - name: telia-direct-connect-dev
        id: "111122223333"
        cidrs:
          eu-north-0:
            - 100.96.1.0/24
            `),
			expectError:          true,
			expectedErrorMessage: "team: Public cloud: allocation 0: unknown region: eu-north-0",
		},
		{
			description: "errors if there is a mismatch between allocated region and supernet region",
			config: strings.TrimSpace(`
gateways:
  eu-west-1:
    transit_gateway_id: tgw-0123
    route_table_id: tgw-rtb-0123
supernets:
  - description: Carrier grade NAT range outlined in RFC6598.
    cidrs:
      eu-west-1:
        - 100.96.0.0/16
teams:
  - team: Public cloud
    accounts:
      - name: telia-direct-connect-dev
        id: "111122223333"
        cidrs:
          eu-north-1:
            - 100.96.1.0/24
            `),
			expectError:          true,
			expectedErrorMessage: "subnet must be in the same region as the containing supernet (eu-west-1): 100.96.1.0/24",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var c cloudconnect.Config

			err := yaml.Unmarshal([]byte(tc.config), &c)
			if err != nil {
				t.Fatalf("unmarshal config: %s", err)
			}
			err = c.Validate()
			if tc.expectError {
				if assert.Error(t, err) {
					assert.Equal(t, tc.expectedErrorMessage, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Note: These tests were written when using sigs.k8s.io/yaml, because
// it would change the order of object keys and put "accounts" before
// the "team" key when marshalling.
func TestConfigRoundtrip(t *testing.T) {
	tests := []struct {
		description string
		input       string
		config      cloudconnect.Config
	}{
		{
			description: "roundtrip",
			input: strings.TrimSpace(`
gateways:
  eu-west-1:
    transit_gateway_id: tgw-0123
    route_table_id: tgw-rtb-0123
supernets:
  - description: Carrier grade NAT range outlined in RFC6598.
    cidrs:
      eu-west-1:
        - 100.96.0.0/16
teams:
  - team: Public cloud
    accounts:
      - name: telia-direct-connect-dev
        id: "837914425576"
        cidrs:
          eu-west-1:
            - 100.96.0.0/24
          `),
			config: cloudconnect.Config{
				Gateways: map[string]*cloudconnect.Gateway{"eu-west-1": {
					ID:           "tgw-0123",
					RouteTableID: "tgw-rtb-0123",
				}},
				Supernets: []*cloudconnect.Supernet{
					{
						Description: "Carrier grade NAT range outlined in RFC6598.",
						CIDRs: map[string][]*cloudconnect.CIDR{
							"eu-west-1": {{net.IPNet{
								IP:   net.IPv4(100, 96, 0, 0).Mask(net.CIDRMask(16, 32)),
								Mask: net.CIDRMask(16, 32),
							}}},
						},
					},
				},
				Teams: []*cloudconnect.Team{
					{
						Team: "Public cloud",
						Accounts: []*cloudconnect.Allocation{
							{
								Name:  "telia-direct-connect-dev",
								Owner: "837914425576",
								CIDRs: map[string][]*cloudconnect.CIDR{"eu-west-1": {{net.IPNet{
									IP:   net.IPv4(100, 96, 0, 0).Mask(net.CIDRMask(24, 32)),
									Mask: net.CIDRMask(24, 32),
								}}}},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			var c cloudconnect.Config

			err := yaml.Unmarshal([]byte(tc.input), &c)
			if assert.NoError(t, err, "unmarshal input") {
				assert.Equal(t, tc.config, c)
			}

			b, err := yaml.Marshal(tc.config)
			if assert.NoError(t, err, "marshal config") {
				assert.YAMLEq(t, tc.input, string(b))
			}
		})
	}
}
