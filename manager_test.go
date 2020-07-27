package cloudconnect_test

import (
	"net"
	"testing"
	"time"

	"github.com/telia-oss/cloudconnect"
	"github.com/telia-oss/cloudconnect/cloudconnectfakes"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
)

func TestListAttachments(t *testing.T) {
	now := time.Now()

	tests := []struct {
		description        string
		attachment         *ec2.TransitGatewayAttachment
		expectedAttachment *cloudconnect.Attachment
	}{
		{
			description: "returns the expected information",
			attachment: &ec2.TransitGatewayAttachment{
				TransitGatewayAttachmentId: aws.String("attachment-id"),
				ResourceOwnerId:            aws.String("resource-owner"),
				ResourceType:               aws.String("resource-type"),
				State:                      aws.String("state"),
				CreationTime:               aws.Time(now),
			},
			expectedAttachment: &cloudconnect.Attachment{
				ID:      cloudconnect.AttachmentID("attachment-id"),
				Owner:   "resource-owner",
				Type:    "resource-type",
				State:   "state",
				Created: now,
				Tags:    make(map[string]string),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			ec2API := &cloudconnectfakes.FakeEC2API{}

			ec2API.DescribeTransitGatewayAttachmentsReturns(&ec2.DescribeTransitGatewayAttachmentsOutput{
				TransitGatewayAttachments: []*ec2.TransitGatewayAttachment{tc.attachment},
				NextToken:                 nil,
			}, nil)

			m := cloudconnect.NewManager(ec2API, "", "", false)

			attachments, err := m.ListAttachments()
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			assert.Equal(t, 1, ec2API.DescribeTransitGatewayAttachmentsCallCount(), "list attachments")
			assert.Equal(t, 1, len(attachments))

			for _, a := range attachments {
				assert.Equal(t, tc.expectedAttachment, a)
			}
		})
	}
}

func TestListAttachmentRoutes(t *testing.T) {
	targetAttachment := &cloudconnect.Attachment{Owner: "owner-id"}

	tests := []struct {
		description   string
		route         *ec2.TransitGatewayRoute
		expectedRoute *cloudconnect.Route
	}{
		{
			description: "returns the expected information",
			route: &ec2.TransitGatewayRoute{
				DestinationCidrBlock: aws.String("10.1.2.0/24"),
				Type:                 aws.String("type"),
				State:                aws.String("state"),
			},
			expectedRoute: &cloudconnect.Route{
				Type:  "type",
				State: "state",
				CIDR: &cloudconnect.CIDR{net.IPNet{
					IP:   net.IPv4(10, 1, 2, 0).Mask(net.CIDRMask(24, 32)),
					Mask: net.CIDRMask(24, 32),
				}},
				Attachment: targetAttachment,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			ec2API := &cloudconnectfakes.FakeEC2API{}

			ec2API.SearchTransitGatewayRoutesReturns(&ec2.SearchTransitGatewayRoutesOutput{
				Routes: []*ec2.TransitGatewayRoute{tc.route},
			}, nil)

			m := cloudconnect.NewManager(ec2API, "", "", false)

			routes, err := m.ListAttachmentRoutes(targetAttachment)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			assert.Equal(t, 1, ec2API.SearchTransitGatewayRoutesCallCount(), "search routes")
			assert.Equal(t, 1, len(routes))

			for _, r := range routes {
				assert.Equal(t, tc.expectedRoute, r)
			}
		})
	}
}
