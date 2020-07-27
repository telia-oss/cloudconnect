package cloudconnect

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/endpoints"
)

// Config represents the configuration for a cloud connect environment. Including
// which network ranges are approved, and CIDR allocations for teams and their accounts.
type Config struct {
	Gateways  map[string]*Gateway `yaml:"gateways"`
	Supernets []*Supernet         `yaml:"supernets"`
	Teams     []*Team             `yaml:"teams"`
}

// Gateway ...
type Gateway struct {
	ID           string `yaml:"transit_gateway_id"`
	RouteTableID string `yaml:"route_table_id"`
}

// Supernet ...
type Supernet struct {
	Description string             `yaml:"description,omitempty"`
	CIDRs       map[string][]*CIDR `yaml:"cidrs"`
}

// Team ...
type Team struct {
	Team     string        `yaml:"team"`
	Accounts []*Allocation `yaml:"accounts"`
}

// Allocation represents a cloud connect CIDR allocation.
type Allocation struct {
	Name  string             `yaml:"name"`
	Owner string             `yaml:"id"`
	CIDRs map[string][]*CIDR `yaml:"cidrs"`
}

// Validate the configuration for cloud connect to ensure that there are no overlapping
// CIDRs, and that all CIDRs fall within the approved supernets.
func (c *Config) Validate() error {
	if len(c.Gateways) == 0 {
		return errors.New("no gateways specified")
	}
	for region, g := range c.Gateways {
		if err := validateRegion(region); err != nil {
			return err
		}
		if err := g.validate(); err != nil {
			return err
		}
	}
	if len(c.Supernets) == 0 {
		return errors.New("no supernets specified")
	}
	for i, n := range c.Supernets {
		if err := n.validate(); err != nil {
			return fmt.Errorf("supernet %d: %s", i, err)
		}
	}

	if len(c.Teams) == 0 {
		return errors.New("no teams specified")
	}

	accounts := make(map[string]struct{})
	for i, t := range c.Teams {
		if err := t.validate(); err != nil {
			return fmt.Errorf("team %d: %s", i, err)
		}
		for j, a := range t.Accounts {
			if _, seen := accounts[a.Owner]; seen {
				return fmt.Errorf("account has duplicate entries: %s", a.Owner)
			}
			accounts[a.Owner] = struct{}{}
			err := a.validate()
			if err != nil {
				return fmt.Errorf("team: %s: allocation %d: %s", t.Team, j, err)
			}
		}
	}

	if err := VerifyNoOverlap(c.ListSupernets()); err != nil {
		return fmt.Errorf("verify supernets: %s", err)
	}

	for region, subnets := range c.ListSubnetsByRegion() {
		for _, n := range subnets {
			supernetRegion, ok := getSupernetRegion(n, c.ListSupernetsByRegion())
			if !ok {
				return fmt.Errorf("subnet is not in a known supernet: %s", n.String())
			}
			if region != supernetRegion {
				return fmt.Errorf("subnet must be in the same region as the containing supernet (%s): %s", supernetRegion, n.String())
			}
		}
	}

	if err := VerifyNoOverlap(c.ListSubnets()); err != nil {
		return fmt.Errorf("verifying subnets: %s", err)
	}

	return nil
}

// Allocations returns the allocations as a list.
func (c *Config) Allocations() (allocations []*Allocation) {
	for _, t := range c.Teams {
		allocations = append(allocations, t.Accounts...)
	}
	return allocations
}

// ListSupernets returns a list of the configured supernets.
func (c *Config) ListSupernets() (supernets []*CIDR) {
	for _, cidrs := range c.ListSupernetsByRegion() {
		supernets = append(supernets, cidrs...)
	}
	return supernets
}

// ListSupernetsByRegion returns a map of the configured supernets per region.
func (c *Config) ListSupernetsByRegion() map[string][]*CIDR {
	supernets := make(map[string][]*CIDR)
	for _, n := range c.Supernets {
		for region, cidrs := range n.CIDRs {
			supernets[region] = append(supernets[region], cidrs...)
		}
	}
	return supernets
}

// ListSubnets returns a list of the reserved subnets.
func (c *Config) ListSubnets() (subnets []*CIDR) {
	for _, cidrs := range c.ListSubnetsByRegion() {
		subnets = append(subnets, cidrs...)
	}
	return subnets
}

// ListSubnetsByRegion returns a map of the reserved subnets per region.
func (c *Config) ListSubnetsByRegion() map[string][]*CIDR {
	subnets := make(map[string][]*CIDR)
	for _, t := range c.Teams {
		for _, a := range t.Accounts {
			for region, cidrs := range a.CIDRs {
				subnets[region] = append(subnets[region], cidrs...)
			}
		}
	}
	return subnets
}

func (g *Gateway) validate() error {
	if g.ID == "" {
		return errors.New("missing gateway id")
	}
	if g.RouteTableID == "" {
		return errors.New("missing route table id")
	}
	return nil
}

func (n *Supernet) validate() error {
	if len(n.CIDRs) == 0 {
		return errors.New("missing CIDR ranges")
	}
	for region, cidrs := range n.CIDRs {
		if err := validateRegion(region); err != nil {
			return err
		}
		if len(cidrs) == 0 {
			return fmt.Errorf("missing CIDR ranges for region: %s", region)
		}
	}
	return nil
}

func (t *Team) validate() error {
	if t.Team == "" {
		return errors.New("missing name")
	}
	if len(t.Accounts) == 0 {
		return errors.New("missing allocations")
	}
	return nil
}

func (a *Allocation) validate() error {
	if a.Name == "" {
		return errors.New("missing name")
	}
	if a.Owner == "" {
		return errors.New("missing owner")
	}
	if len(a.CIDRs) == 0 {
		return errors.New("missing cidrs")
	}
	for region, cidrs := range a.CIDRs {
		if err := validateRegion(region); err != nil {
			return err
		}
		if len(cidrs) == 0 {
			return fmt.Errorf("missing cidrs for region: %s", region)
		}
	}
	return nil
}

func validateRegion(region string) error {
	partition := endpoints.AwsPartition()
	for _, r := range partition.Regions() {
		if r.ID() == region {
			return nil
		}
	}
	return fmt.Errorf("unknown region: %s", region)
}

func getSupernetRegion(c *CIDR, supernets map[string][]*CIDR) (string, bool) {
	for region, cidrs := range supernets {
		for _, cc := range cidrs {
			if cc.Includes(c) {
				return region, true
			}
		}
	}
	return "", false
}
