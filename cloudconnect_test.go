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

func TestPlan(t *testing.T) {
	var (
		targetOwner = "resource-owner"
		targetCIDR  = cloudconnect.CIDR{net.IPNet{
			IP:   net.IPv4(10, 1, 2, 0).Mask(net.CIDRMask(24, 32)),
			Mask: net.CIDRMask(24, 32),
		}}
	)

	tests := []struct {
		description              string
		attachment               *cloudconnect.Attachment
		expectedSearchRouteCalls int
		propagatedRoute          string
		expectedAction           cloudconnect.ChangeAction
	}{
		{
			description: "ignores non-vpc attachments",
			attachment: &cloudconnect.Attachment{
				ID:   cloudconnect.AttachmentID("ID"),
				Type: "direct-connect",
			},
			expectedSearchRouteCalls: 0,
			expectedAction:           cloudconnect.NoOp,
		},
		{
			description: "no-op when everything is ok",
			attachment: &cloudconnect.Attachment{
				ID:    cloudconnect.AttachmentID("ID"),
				Owner: targetOwner,
				Type:  "vpc",
				State: "available",
				Tags:  map[string]string{"Name": "account-name"},
			},
			expectedSearchRouteCalls: 1,
			propagatedRoute:          targetCIDR.String(),
			expectedAction:           cloudconnect.NoOp,
		},
		{
			description: "accepts pending vpc attachments for allocated accounts",
			attachment: &cloudconnect.Attachment{
				ID:      cloudconnect.AttachmentID("ID"),
				Owner:   targetOwner,
				Type:    "vpc",
				State:   "pendingAcceptance",
				Created: time.Now(),
			},
			propagatedRoute: targetCIDR.String(),
			expectedAction:  cloudconnect.ApproveAttachment,
		},
		{
			description: "rejects pending attachments after 3 days",
			attachment: &cloudconnect.Attachment{
				ID:      cloudconnect.AttachmentID("ID"),
				Owner:   "unallocated",
				Type:    "vpc",
				State:   "pendingAcceptance",
				Created: time.Now().AddDate(0, 0, -3),
			},
			expectedAction: cloudconnect.RejectAttachment,
		},
		{
			description: "deletes attachments when the allocation is removed",
			attachment: &cloudconnect.Attachment{
				ID:    cloudconnect.AttachmentID("ID"),
				Owner: "unallocated",
				Type:  "vpc",
				State: "available",
			},
			propagatedRoute: targetCIDR.String(),
			expectedAction:  cloudconnect.DeleteAttachment,
		},
		{
			description: "deletes attachments with unallocated route propagations",
			attachment: &cloudconnect.Attachment{
				ID:    cloudconnect.AttachmentID("ID"),
				Owner: targetOwner,
				Type:  "vpc",
				State: "available",
			},
			expectedSearchRouteCalls: 1,
			propagatedRoute:          "100.64.0.0/10",
			expectedAction:           cloudconnect.DeleteAttachment,
		},
		{
			description: "ignores IPv6 routes",
			attachment: &cloudconnect.Attachment{
				ID:    cloudconnect.AttachmentID("ID"),
				Owner: targetOwner,
				Type:  "vpc",
				State: "available",
				Tags:  map[string]string{"Name": "account-name"},
			},
			expectedSearchRouteCalls: 1,
			propagatedRoute:          "2a05:d018:770:b801::/56",
			expectedAction:           cloudconnect.NoOp,
		},
		{
			description: "updates tags when they do not match the allocation",
			attachment: &cloudconnect.Attachment{
				ID:    cloudconnect.AttachmentID("ID"),
				Owner: targetOwner,
				Type:  "vpc",
				State: "available",
				Tags:  map[string]string{"Name": "not-account-name"},
			},
			expectedSearchRouteCalls: 1,
			propagatedRoute:          targetCIDR.String(),
			expectedAction:           cloudconnect.TagAttachment,
		},
		{
			description: "no-op if propagation is a subnet of allocated cidr",
			attachment: &cloudconnect.Attachment{
				ID:    cloudconnect.AttachmentID("ID"),
				Owner: targetOwner,
				Type:  "vpc",
				State: "available",
				Tags:  map[string]string{"Name": "account-name"},
			},
			expectedSearchRouteCalls: 1,
			propagatedRoute:          "10.1.2.0/25",
			expectedAction:           cloudconnect.NoOp,
		}, {
			description: "no-op if propagation is rejected and no tag",
			attachment: &cloudconnect.Attachment{
				ID:    cloudconnect.AttachmentID("ID"),
				Owner: targetOwner,
				Type:  "vpc",
				State: "rejected",
				Tags:  nil,
			},
			expectedSearchRouteCalls: 0,
			propagatedRoute:          "10.1.2.0/25",
			expectedAction:           cloudconnect.NoOp,
		},
		{
			description: "no-op if propagation is deleted and no tag",
			attachment: &cloudconnect.Attachment{
				ID:    cloudconnect.AttachmentID("ID"),
				Owner: targetOwner,
				Type:  "vpc",
				State: "deleted",
				Tags:  nil,
			},
			expectedSearchRouteCalls: 0,
			propagatedRoute:          "10.1.2.0/25",
			expectedAction:           cloudconnect.NoOp,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			ec2API := &cloudconnectfakes.FakeEC2API{}

			ec2API.SearchTransitGatewayRoutesReturns(&ec2.SearchTransitGatewayRoutesOutput{
				Routes: []*ec2.TransitGatewayRoute{{
					DestinationCidrBlock: aws.String(tc.propagatedRoute),
					Type:                 aws.String("propagated"),
					State:                aws.String("active"),
				}},
			}, nil)

			allocation := &cloudconnect.Allocation{
				Name:  "account-name",
				CIDRs: map[string][]*cloudconnect.CIDR{"eu-west-1": {&targetCIDR}},
				Owner: targetOwner,
			}

			m := cloudconnect.NewManager(ec2API, "", "", false)

			change, err := cloudconnect.Plan(m, tc.attachment, []*cloudconnect.Allocation{allocation}, "eu-west-1")
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			assert.Equal(t, tc.expectedSearchRouteCalls, ec2API.SearchTransitGatewayRoutesCallCount(), "search routes")
			assert.Equal(t, tc.expectedAction, change.Action)
			assert.Equal(t, tc.attachment, change.Attachment)
		})
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		description          string
		action               cloudconnect.ChangeAction
		expectedApproveCalls int
		expectedRejectCalls  int
		expectedDeleteCalls  int
	}{
		{
			description: "no-op works",
			action:      cloudconnect.NoOp,
		},
		{
			description:          "approve works",
			action:               cloudconnect.ApproveAttachment,
			expectedApproveCalls: 1,
		},
		{
			description:         "reject works",
			action:              cloudconnect.RejectAttachment,
			expectedRejectCalls: 1,
		},
		{
			description:         "delete works",
			action:              cloudconnect.DeleteAttachment,
			expectedDeleteCalls: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			ec2API := &cloudconnectfakes.FakeEC2API{}

			m := cloudconnect.NewManager(ec2API, "", "", false)
			c := &cloudconnect.AttachmentChange{
				Action: tc.action,
				Attachment: &cloudconnect.Attachment{
					ID: cloudconnect.AttachmentID("ID"),
				},
			}

			err := cloudconnect.Apply(m, c)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			assert.Equal(t, tc.expectedApproveCalls, ec2API.AcceptTransitGatewayVpcAttachmentCallCount(), "accept")
			assert.Equal(t, tc.expectedRejectCalls, ec2API.RejectTransitGatewayVpcAttachmentCallCount(), "reject")
			assert.Equal(t, tc.expectedDeleteCalls, ec2API.DeleteTransitGatewayVpcAttachmentCallCount(), "delete")
		})
	}
}
