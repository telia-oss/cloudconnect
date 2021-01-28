package cloudconnect

import (
	"fmt"
	"strings"
	"time"
)

// AttachmentID for a transit gateway attachment.
type AttachmentID string

// Attachment represents an attachment in transit gateway.
type Attachment struct {
	// ID is the AttachmentID for the attachment.
	ID AttachmentID

	// Owner is the account ID of the owning account.
	Owner string

	// Type of attachment, e.g. VPC or direct-connect.
	Type string

	// State of the attachment. E.g. pendingApproval, or available.
	State string

	// Created time for the attachment.
	Created time.Time

	// Tags contains a map of the tags for the attachment.
	Tags map[string]string
}

// Route represents a route in a transit gateway.
type Route struct {
	// CIDR represents the destination for the route.
	CIDR *CIDR

	// Type of route, e.g. static or propagated.
	Type string

	// State of the route.
	State string

	// Attachment is the embedded attachment that the route points to.
	*Attachment
}

// AttachmentChange represents a planned change for a transit gateway attachment.
type AttachmentChange struct {
	// Action denotes the change that is being made.
	Action ChangeAction

	// Reason contains additional information about the reason for the change.
	Reason string

	// Allocation holds a pointer to an allocation if it exists, as such
	// a nil check should be performed before using this value.
	Allocation *Allocation

	// Attachment is an embedded pointer to the attachment subject to change.
	*Attachment
}

type ChangeAction string

const (
	ApproveAttachment ChangeAction = "APPROVE"
	RejectAttachment  ChangeAction = "REJECT"
	DeleteAttachment  ChangeAction = "DELETE"
	TagAttachment     ChangeAction = "TAG"
	NoOp              ChangeAction = "NONE"
)

// Plan changes for a single attachment based on the supplied allocations.
func Plan(m Manager, a *Attachment, allocations []*Allocation, region string) (*AttachmentChange, error) {
	allocation, _ := getAllocationByOwner(a.Owner, allocations)
	action, cause, err := getChangeAction(m, a, allocation, region)
	if err != nil {
		return nil, err
	}
	change := &AttachmentChange{
		Action:     action,
		Reason:     cause,
		Allocation: allocation,
		Attachment: a,
	}
	return change, nil
}

// PlanAll generates a plan for all the given attachments.
func PlanAll(m Manager, attachments []*Attachment, allocations []*Allocation, region string) ([]*AttachmentChange, error) {
	var changes []*AttachmentChange
	for _, a := range attachments {
		change, err := Plan(m, a, allocations, region)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, nil
}

// Apply a change to an attachment.
func Apply(m Manager, change *AttachmentChange) error {
	switch change.Action {
	case ApproveAttachment:
		return m.ApprovePendingAttachment(change.Attachment)
	case RejectAttachment:
		return m.RejectPendingAttachment(change.Attachment)
	case DeleteAttachment:
		return m.DeleteAttachment(change.Attachment)
	case TagAttachment:
		tags := map[string]string{"Name": change.Allocation.Name}
		return m.SetAttachmentTags(change.Attachment, tags)
	case NoOp:
		return nil
	default:
		return fmt.Errorf("invalid action: %s", change.Action)
	}
}

// ApplyAll performs an Apply for all the provided changes.
func ApplyAll(m Manager, changes []*AttachmentChange) error {
	for _, c := range changes {
		if err := Apply(m, c); err != nil {
			return err
		}
	}
	return nil
}

func getChangeAction(m Manager, a *Attachment, allocation *Allocation, currentRegion string) (ChangeAction, string, error) {
	if a.Type != "vpc" {
		return NoOp, "not a vpc attachment", nil
	}
	switch a.State {
	case "pendingAcceptance":
		if allocation != nil {
			return ApproveAttachment, "account is allocated", nil
		}

		// This rejects (deletes) all pending allowed tgw-attachments, that have not been completed within 3 days, to attempt a retry the tgw-attachments need to be tried.
		if a.Created.Before(time.Now().AddDate(0, 0, -3)) {
			return RejectAttachment, "pending without allocation for >3 days", nil
		}
	case "available":
		if allocation == nil {
			return DeleteAttachment, "missing allocation for account", nil
		}
		routes, err := m.ListAttachmentRoutes(a)
		if err != nil {
			return "", "", fmt.Errorf("list attachment route: %s: %s", string(a.ID), err)
		}
		for _, r := range routes {
			if r.Type == "static" || strings.Contains(r.CIDR.String(), ":") {
				// Ignore static routes and IPv6 routes
				continue
			}
			region, found := getRegionForRoute(r.CIDR, allocation)
			if !found {
				cause := fmt.Sprintf("missing allocation for propagated route: %s", r.CIDR.String())
				return DeleteAttachment, cause, nil
			}
			if region != currentRegion {
				cause := fmt.Sprintf("route propagation from wrong region (subnet belongs to %s): %s", region, r.CIDR.String())
				return DeleteAttachment, cause, nil
			}
		}
	}
	if nameTag, ok := a.Tags["Name"]; ok {
		if nameTag != allocation.Name {
			return TagAttachment, "name tag does not match allocation", nil
		}
	}

	return NoOp, "", nil
}

func getAllocationByOwner(ownerID string, allocations []*Allocation) (*Allocation, bool) {
	for _, allocation := range allocations {
		if allocation.Owner == ownerID {
			return allocation, true
		}
	}
	return nil, false
}

func getRegionForRoute(route *CIDR, allocation *Allocation) (string, bool) {
	for region, cidrs := range allocation.CIDRs {
		for _, c := range cidrs {
			if c.String() == route.String() {
				return region, true
			}
			if c.Includes(route) {
				return region, true
			}
		}
	}
	return "", false
}
