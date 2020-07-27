package cli_test

import (
	"bytes"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/telia-oss/cloudconnect"
	"github.com/telia-oss/cloudconnect/cloudconnectfakes"
	"github.com/telia-oss/cloudconnect/internal/cli"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	testAttachment = &cloudconnect.Attachment{
		ID:      cloudconnect.AttachmentID("tgw-123456789"),
		Owner:   "01234567891",
		Type:    "vpc",
		State:   "available",
		Created: time.Date(2019, 12, 4, 20, 1, 24, 0, time.UTC),
		Tags:    map[string]string{"Name": "account-x"},
	}
	testRoute = &cloudconnect.Route{
		Type:  "propagated",
		State: "active",
		CIDR: &cloudconnect.CIDR{IPNet: net.IPNet{
			IP:   net.IPv4(100, 96, 0, 0).Mask(net.CIDRMask(23, 32)),
			Mask: net.CIDRMask(23, 32),
		}},
		Attachment: testAttachment,
	}
	testConfig = strings.TrimSpace(`
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
    `)
)

func testEC2Factory(_ string) cloudconnect.EC2API {
	return &cloudconnectfakes.FakeEC2API{}
}

func testManagerFactory(_ cloudconnect.EC2API, _ string, _ string, _ bool) cloudconnect.Manager {
	m := &cloudconnectfakes.FakeManager{}
	m.ListAttachmentsReturns([]*cloudconnect.Attachment{testAttachment}, nil)
	m.ListAttachmentRoutesReturns([]*cloudconnect.Route{testRoute}, nil)
	return m
}

func newTestConfigFile(t *testing.T) string {
	c, err := ioutil.TempFile("", "allocations.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write([]byte(testConfig)); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	return c.Name()
}

func TestCLI(t *testing.T) {
	fakeConfig := newTestConfigFile(t)
	defer os.Remove(fakeConfig)

	tests := []struct {
		description string
		command     []string
		expected    string
	}{
		{
			description: "format",
			command:     []string{"format", fakeConfig},
			expected:    strings.TrimSpace(``),
		},
		{
			description: "validate",
			command:     []string{"validate", fakeConfig},
			expected:    strings.TrimSpace(``),
		},
		{
			description: "list attachments",
			command:     []string{"list", "attachments", fakeConfig, "--region", "eu-west-1"},
			expected: strings.TrimSpace(`
| Attachment ID | Name      | Owner       | Type | State     | Created              |
| ------------- | ----      | -----       | ---- | -----     | -------              |
| tgw-123456789 | account-x | 01234567891 | vpc  | available | 2019-12-04T20:01:24Z |
             `),
		},
		{
			description: "list routes",
			command:     []string{"list", "routes", fakeConfig, "--region", "eu-west-1"},
			expected: strings.TrimSpace(`
| Attachment ID | Name      | Owner       | Type | State     | Created              | Routes        |            |        |
| ------------- | ----      | -----       | ---- | -----     | -------              | ------        | ---        | ---    |
| tgw-123456789 | account-x | 01234567891 | vpc  | available | 2019-12-04T20:01:24Z | 100.96.0.0/23 | propagated | active |
             `),
		},
		{
			description: "list supernets",
			command:     []string{"list", "supernets", fakeConfig},
			expected: strings.TrimSpace(`
| Supernet      | # of allocations | IPs (used/total) |
| --------      | ---------------- | ---------------- |
| 100.96.0.0/16 | 1                | 512/65536        |
             `),
		},
		{
			description: "plan",
			command:     []string{"plan", fakeConfig, "--region", "eu-west-1"},
			expected: strings.TrimSpace(`
| Attachment ID | Name      | Owner       | Type | State     | Created              | Action | Reason                         |
| ------------- | ----      | -----       | ---- | -----     | -------              | ------ | ------                         |
| tgw-123456789 | account-x | 01234567891 | vpc  | available | 2019-12-04T20:01:24Z | DELETE | missing allocation for account |
             `),
		},
		{
			description: "apply",
			command:     []string{"apply", fakeConfig, "--auto-approve", "--region", "eu-west-1"},
			expected: strings.TrimSpace(`
| Attachment ID | Name      | Owner       | Type | State     | Created              | Action | Reason                         |
| ------------- | ----      | -----       | ---- | -----     | -------              | ------ | ------                         |
| tgw-123456789 | account-x | 01234567891 | vpc  | available | 2019-12-04T20:01:24Z | DELETE | missing allocation for account |

Applying change: DELETE   on attachment: tgw-123456789 - SUCCESS
             `),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {

			var b bytes.Buffer

			app := kingpin.New("test", "").Terminate(nil)
			cli.Setup(app, testEC2Factory, testManagerFactory, &b)

			_, err := app.Parse(tc.command)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			assert.Equal(t, tc.expected, strings.TrimSpace(b.String()))
		})
	}

}
