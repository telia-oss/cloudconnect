package cloudconnect

import (
	"fmt"
	"net"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// EC2API wraps the interface for the API and provides a mocked implementation.
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . EC2API
type EC2API interface {
	CreateTags(*ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error)
	SearchTransitGatewayRoutes(*ec2.SearchTransitGatewayRoutesInput) (*ec2.SearchTransitGatewayRoutesOutput, error)
	DescribeTransitGatewayAttachments(*ec2.DescribeTransitGatewayAttachmentsInput) (*ec2.DescribeTransitGatewayAttachmentsOutput, error)
	AcceptTransitGatewayVpcAttachment(*ec2.AcceptTransitGatewayVpcAttachmentInput) (*ec2.AcceptTransitGatewayVpcAttachmentOutput, error)
	RejectTransitGatewayVpcAttachment(*ec2.RejectTransitGatewayVpcAttachmentInput) (*ec2.RejectTransitGatewayVpcAttachmentOutput, error)
	DeleteTransitGatewayVpcAttachment(*ec2.DeleteTransitGatewayVpcAttachmentInput) (*ec2.DeleteTransitGatewayVpcAttachmentOutput, error)
}

// Manager provides an easy API for managing transit gateway.
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Manager
type Manager interface {
	ListAttachments() ([]*Attachment, error)
	ListAttachmentRoutes(a *Attachment) ([]*Route, error)
	SetAttachmentTags(a *Attachment, tags map[string]string) error
	ApprovePendingAttachment(a *Attachment) error
	RejectPendingAttachment(a *Attachment) error
	DeleteAttachment(a *Attachment) error
}

// NewManager returns a new transit gateway manager.
func NewManager(client EC2API, gatewayID, routeTableID string, dryRun bool) Manager {
	return &manager{
		client:       client,
		gatewayID:    gatewayID,
		routeTableID: routeTableID,
		dryRun:       dryRun,
	}
}

type manager struct {
	client       EC2API
	gatewayID    string
	routeTableID string
	dryRun       bool
}

// ListAttachments for the given transit gateway.
func (m *manager) ListAttachments() ([]*Attachment, error) {
	var (
		attachments []*Attachment
		nextToken   *string
	)

	for {
		out, err := m.client.DescribeTransitGatewayAttachments(&ec2.DescribeTransitGatewayAttachmentsInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("transit-gateway-id"),
					Values: []*string{aws.String(m.gatewayID)},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}

		for _, a := range out.TransitGatewayAttachments {
			tags := make(map[string]string, len(a.Tags))
			for _, tag := range a.Tags {
				tags[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
			}
			attachments = append(attachments, &Attachment{
				ID:      AttachmentID(aws.StringValue(a.TransitGatewayAttachmentId)),
				Owner:   aws.StringValue(a.ResourceOwnerId),
				Type:    aws.StringValue(a.ResourceType),
				State:   aws.StringValue(a.State),
				Created: aws.TimeValue(a.CreationTime),
				Tags:    tags,
			})
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}

	return attachments, nil
}

// ListAttachmentRoutes for the given attachment.
func (m *manager) ListAttachmentRoutes(attachment *Attachment) ([]*Route, error) {
	out, err := m.client.SearchTransitGatewayRoutes(&ec2.SearchTransitGatewayRoutesInput{
		TransitGatewayRouteTableId: aws.String(m.routeTableID),
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("attachment.transit-gateway-attachment-id"),
				Values: []*string{aws.String(string(attachment.ID))},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	var routes []*Route
	for _, r := range out.Routes {
		_, c, err := net.ParseCIDR(aws.StringValue(r.DestinationCidrBlock))
		if err != nil {
			return nil, fmt.Errorf("parse cidr: %s", err)
		}
		routes = append(routes, &Route{
			CIDR:       &CIDR{IPNet: *c},
			Type:       aws.StringValue(r.Type),
			State:      aws.StringValue(r.State),
			Attachment: attachment,
		})
	}
	return routes, nil
}

// SetAttachmentTags creates or updates the tags for a transit gateway attachment.
func (m *manager) SetAttachmentTags(attachment *Attachment, tags map[string]string) error {
	ec2Tags := make([]*ec2.Tag, len(tags))

	for k, v := range tags {
		ec2Tags = append(ec2Tags, &ec2.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	_, err := m.client.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{aws.String(string(attachment.ID))},
		Tags:      ec2Tags,
		DryRun:    aws.Bool(m.dryRun),
	})
	if err != nil && !isDryRunError(err) {
		return err
	}
	return nil
}

// RejectPendingAttachment to transit gateway.
func (m *manager) RejectPendingAttachment(attachment *Attachment) error {
	_, err := m.client.RejectTransitGatewayVpcAttachment(&ec2.RejectTransitGatewayVpcAttachmentInput{
		TransitGatewayAttachmentId: aws.String(string(attachment.ID)),
		DryRun:                     aws.Bool(m.dryRun),
	})
	if err != nil && !isDryRunError(err) {
		return err
	}
	return nil
}

// ApprovePendingAttachment to transit gateway.
func (m *manager) ApprovePendingAttachment(attachment *Attachment) error {
	_, err := m.client.AcceptTransitGatewayVpcAttachment(&ec2.AcceptTransitGatewayVpcAttachmentInput{
		TransitGatewayAttachmentId: aws.String(string(attachment.ID)),
		DryRun:                     aws.Bool(m.dryRun),
	})
	if err != nil && !isDryRunError(err) {
		return err
	}
	return nil
}

// DeleteAttachment for transit gateway.
func (m *manager) DeleteAttachment(attachment *Attachment) error {
	_, err := m.client.DeleteTransitGatewayVpcAttachment(&ec2.DeleteTransitGatewayVpcAttachmentInput{
		TransitGatewayAttachmentId: aws.String(string(attachment.ID)),
		DryRun:                     aws.Bool(m.dryRun),
	})
	if err != nil && !isDryRunError(err) {
		return err
	}
	return nil
}

func isDryRunError(err error) bool {
	e, ok := err.(awserr.Error)
	if !ok {
		return false
	}
	if e.Code() == "DryRunOperation" {
		return true
	}
	return false
}
