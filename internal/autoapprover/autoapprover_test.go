package autoapprover_test

import (
	"io/ioutil"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/telia-oss/cloudconnect"
	"github.com/telia-oss/cloudconnect/cloudconnectfakes"
	"github.com/telia-oss/cloudconnect/internal/autoapprover"
	"github.com/telia-oss/cloudconnect/internal/autoapprover/autoapproverfakes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/aws/aws-sdk-go/service/s3"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	testAttachment = &cloudconnect.Attachment{
		ID:      cloudconnect.AttachmentID("target"),
		Owner:   "target",
		Type:    "vpc",
		State:   "available",
		Created: time.Date(2019, 12, 4, 20, 1, 24, 0, time.UTC),
	}
	testRoute = &cloudconnect.Route{
		Type:  "propagated",
		State: "active",
		CIDR: &cloudconnect.CIDR{IPNet: net.IPNet{
			IP:   net.IPv4(100, 96, 4, 0).Mask(net.CIDRMask(23, 32)),
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
        id: "target"
        cidrs: 
          eu-west-1:
            - 100.96.0.0/23
    `)
)

func testAWSClientFactory(_ string) (autoapprover.S3API, cloudconnect.EC2API, error) {
	fakeS3 := &autoapproverfakes.FakeS3API{}
	fakeS3.GetObjectReturns(&s3.GetObjectOutput{
		Body: ioutil.NopCloser(strings.NewReader(testConfig)),
	}, nil)
	return fakeS3, &cloudconnectfakes.FakeEC2API{}, nil
}

func TestCLI(t *testing.T) {
	tests := []struct {
		description string
		command     []string
		attachments []*cloudconnect.Attachment
		expected    string
	}{
		{
			description: "run",
			command:     []string{"--config-bucket", "bucket-name", "--config-path", "development.yml", "--region", "eu-west-1", "--dry-run", "--local"},
			attachments: []*cloudconnect.Attachment{
				testAttachment,
				{
					ID:      cloudconnect.AttachmentID("target"),
					Owner:   "target",
					Type:    "vpc",
					State:   "pendingAcceptance",
					Created: time.Now(),
				},
				{
					ID:      cloudconnect.AttachmentID("unallocated"),
					Owner:   "unallocated",
					Type:    "vpc",
					State:   "available",
					Created: time.Now(),
				},
				{
					ID:      cloudconnect.AttachmentID("ignored"),
					Owner:   "ignored",
					Type:    "direct-connect",
					State:   "available",
					Created: time.Now(),
				},
				{
					ID:      cloudconnect.AttachmentID("outdated"),
					Owner:   "outdated",
					Type:    "vpc",
					State:   "pendingAcceptance",
					Created: time.Time{},
				},
			},
			expected: strings.TrimSpace(`
{"level":"debug","msg":"processing attachments","attachments":5,"allocations":1}
{"level":"info","msg":"planning change","id":"target","owner":"target","state":"available"}
{"level":"warn","msg":"applying destructive change","id":"target","owner":"target","state":"available","action":"DELETE","reason":"missing allocation for propagated route: 100.96.4.0/23"}
{"level":"info","msg":"apply done","id":"target","owner":"target","state":"available","action":"DELETE"}
{"level":"info","msg":"planning change","id":"target","owner":"target","state":"pendingAcceptance"}
{"level":"info","msg":"applying change","id":"target","owner":"target","state":"pendingAcceptance","action":"APPROVE","reason":"account is allocated"}
{"level":"info","msg":"apply done","id":"target","owner":"target","state":"pendingAcceptance","action":"APPROVE"}
{"level":"info","msg":"planning change","id":"unallocated","owner":"unallocated","state":"available"}
{"level":"warn","msg":"applying destructive change","id":"unallocated","owner":"unallocated","state":"available","action":"DELETE","reason":"missing allocation for account"}
{"level":"info","msg":"apply done","id":"unallocated","owner":"unallocated","state":"available","action":"DELETE"}
{"level":"info","msg":"planning change","id":"ignored","owner":"ignored","state":"available"}
{"level":"debug","msg":"skipping no-op","id":"ignored","owner":"ignored","state":"available","action":"NONE","reason":"not a vpc attachment"}
{"level":"info","msg":"planning change","id":"outdated","owner":"outdated","state":"pendingAcceptance"}
{"level":"warn","msg":"applying destructive change","id":"outdated","owner":"outdated","state":"pendingAcceptance","action":"REJECT","reason":"pending without allocation for >3 days"}
{"level":"info","msg":"apply done","id":"outdated","owner":"outdated","state":"pendingAcceptance","action":"REJECT"}
             `),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			b := &zaptest.Buffer{}
			loggerFactory := func(bool) (*zap.Logger, error) {
				c := zap.NewProductionEncoderConfig()
				c.TimeKey = ""
				e := zapcore.NewJSONEncoder(c)
				l := zap.New(zapcore.NewCore(e, zapcore.AddSync(b), zapcore.DebugLevel))
				return l, nil
			}

			m := &cloudconnectfakes.FakeManager{}
			managerFactory := func(_ cloudconnect.EC2API, _ string, _ string, _ bool) cloudconnect.Manager {
				m.ListAttachmentsReturns(tc.attachments, nil)
				m.ListAttachmentRoutesReturns([]*cloudconnect.Route{testRoute}, nil)
				return m
			}

			app := kingpin.New("test", "").Terminate(nil)
			autoapprover.Setup(app, nil, testAWSClientFactory, managerFactory, loggerFactory)

			_, err := app.Parse(tc.command)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			assert.Equal(t, tc.expected, strings.TrimSpace(b.String()))
			assert.Equal(t, 1, m.ListAttachmentsCallCount(), "list attachments")
			assert.Equal(t, 1, m.ListAttachmentRoutesCallCount(), "list routes")
			assert.Equal(t, 2, m.DeleteAttachmentCallCount(), "delete attachment")
		})
	}
}
