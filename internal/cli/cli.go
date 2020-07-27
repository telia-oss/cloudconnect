package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/telia-oss/cloudconnect"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v3"
)

type (
	ec2Factory     func(string) cloudconnect.EC2API
	managerFactory func(cloudconnect.EC2API, string, string, bool) cloudconnect.Manager
)

// Setup the CLI.
func Setup(app *kingpin.Application, ec2Factory ec2Factory, managerFactory managerFactory, w io.Writer) {
	addFormatCommand(app)
	addValidateCommand(app)
	addNextCIDRCommand(app, w)
	addListCommand(app, ec2Factory, managerFactory, w)
	addPlanCommand(app, ec2Factory, managerFactory, w)
	addApplyCommand(app, ec2Factory, managerFactory, w)
}

func addFormatCommand(app *kingpin.Application) {
	var (
		cmd   = app.Command("format", "Format config file")
		files = cmd.Arg("file", "Path to one or more config files").Required().ExistingFiles()
	)

	cmd.Action(func(_ *kingpin.ParseContext) error {
		for _, f := range *files {
			c := readConfig(f)

			var b bytes.Buffer
			encoder := yaml.NewEncoder(&b)
			encoder.SetIndent(2)

			if err := encoder.Encode(c); err != nil {
				kingpin.Fatalf("marshal config: %s: %s", f, err)
			}
			encoder.Close()

			fs, err := os.Stat(f)
			if err != nil {
				kingpin.Fatalf("file stat: %s: %s", f, err)
			}
			if err := ioutil.WriteFile(f, b.Bytes(), fs.Mode()); err != nil {
				kingpin.Fatalf("write file: %s: %s", f, err)
			}
		}
		return nil
	})
}

func addValidateCommand(app *kingpin.Application) {
	var (
		cmd   = app.Command("validate", "Validate config file")
		files = cmd.Arg("file", "Path to one or more config files").Required().ExistingFiles()
	)

	cmd.Action(func(_ *kingpin.ParseContext) error {
		for _, f := range *files {
			c := readConfig(f)
			if err := c.Validate(); err != nil {
				kingpin.Fatalf("validate config: %s: %s", f, err)
			}
		}
		return nil
	})
}

func addNextCIDRCommand(app *kingpin.Application, w io.Writer) {
	var (
		cmd      = app.Command("next-cidr", "Get the next available CIDR")
		file     = cmd.Arg("file", "Path to a config file").Required().ExistingFile()
		supernet = cmd.Flag("supernet", "Supernet for the new CIDR").Short('s').Required().String()
		prefixes = cmd.Flag("prefix", "Desired prefix length").Short('p').Default("25").Ints()
	)

	cmd.Action(func(_ *kingpin.ParseContext) error {
		c := readConfig(*file)
		if err := nextCIDR(w, c, *supernet, *prefixes); err != nil {
			kingpin.Fatalf("next cidr: %s", err)
		}
		return nil
	})
}

func nextCIDR(w io.Writer, c *cloudconnect.Config, super string, prefixes []int) error {
	var supernet *cloudconnect.CIDR
	for _, s := range c.ListSupernets() {
		if s.String() == super {
			supernet = s
		}
	}
	if supernet == nil {
		return fmt.Errorf("invalid supernet: %s", super)
	}

	subnets := c.ListSubnets()
	for _, prefix := range prefixes {
		cidr, err := supernet.Subnet(prefix, subnets)
		if err != nil {
			return err
		}
		subnets = append(subnets, cidr)
		_, err = fmt.Fprintln(w, cidr.String())
		if err != nil {
			return err
		}
	}
	return nil
}

func addListCommand(app *kingpin.Application, ec2Factory ec2Factory, managerFactory managerFactory, w io.Writer) {
	var (
		list              = app.Command("list", "List attachments and routes")
		attachments       = list.Command("attachments", "List transit gateway attachments")
		attachmentsFile   = attachments.Arg("file", "Path to a config file").Required().ExistingFile()
		attachmentsRegion = attachments.Flag("region", "AWS Region to target").Envar("AWS_REGION").Required().String()
		routes            = list.Command("routes", "List transit gateway routes")
		routesFile        = routes.Arg("file", "Path to a config file").Required().ExistingFile()
		routesRegion      = routes.Flag("region", "AWS Region to target").Envar("AWS_REGION").Required().String()
		supernets         = list.Command("supernets", "List available supernets")
		supernetsFile     = supernets.Arg("file", "Path to a config file").Required().ExistingFile()
	)

	attachments.Action(func(_ *kingpin.ParseContext) error {
		c := readConfig(*attachmentsFile)
		g, ok := c.Gateways[*attachmentsRegion]
		if !ok {
			kingpin.Fatalf("list attachments: no gateway config for region: %s", *attachmentsRegion)
		}
		m := managerFactory(ec2Factory(*attachmentsRegion), g.ID, "", true)
		if err := listAttachments(w, m); err != nil {
			kingpin.Fatalf("list attachments: %s", err)
		}
		return nil
	})

	routes.Action(func(_ *kingpin.ParseContext) error {
		c := readConfig(*routesFile)
		g, ok := c.Gateways[*routesRegion]
		if !ok {
			kingpin.Fatalf("list routes: no gateway config for region: %s", *attachmentsRegion)
		}
		m := managerFactory(ec2Factory(*routesRegion), g.ID, g.RouteTableID, true)
		if err := listRoutes(w, m); err != nil {
			kingpin.Fatalf("list routes: %s", err)
		}
		return nil
	})

	supernets.Action(func(_ *kingpin.ParseContext) error {
		c := readConfig(*supernetsFile)
		if err := listSupernets(w, c); err != nil {
			kingpin.Fatalf("list supernets: %s", err)
		}
		return nil
	})
}

func listAttachments(w io.Writer, m cloudconnect.Manager) error {
	attachments, err := m.ListAttachments()
	if err != nil {
		return err
	}
	if err := printAttachments(w, attachments); err != nil {
		return err
	}
	return nil
}

func printAttachments(w io.Writer, attachments []*cloudconnect.Attachment) error {
	header := []string{"Attachment ID", "Name", "Owner", "Type", "State", "Created"}
	divide := []string{"-------------", "----", "-----", "----", "-----", "-------"}

	var rows [][]string
	rows = append(rows, header, divide)

	for _, a := range attachments {
		row := []string{string(a.ID), a.Tags["Name"], a.Owner, a.Type, a.State, a.Created.Format(time.RFC3339)}
		rows = append(rows, row)
	}
	return writeTable(w, rows)
}

func listRoutes(w io.Writer, m cloudconnect.Manager) error {
	attachments, err := m.ListAttachments()
	if err != nil {
		return err
	}

	var routes []*cloudconnect.Route
	for _, a := range attachments {
		r, err := m.ListAttachmentRoutes(a)
		if err != nil {
			return fmt.Errorf("list route for attachment: %s: %s", string(a.ID), err)
		}
		routes = append(routes, r...)
	}

	if err := printRoutes(w, routes); err != nil {
		return err
	}
	return nil
}

func printRoutes(w io.Writer, routes []*cloudconnect.Route) error {
	header := []string{"Attachment ID", "Name", "Owner", "Type", "State", "Created", "Routes", "   ", "   "}
	divide := []string{"-------------", "----", "-----", "----", "-----", "-------", "------", "---", "---"}

	var rows [][]string
	rows = append(rows, header, divide)

	for _, r := range routes {
		a := r.Attachment
		row := []string{string(a.ID), a.Tags["Name"], a.Owner, a.Type, a.State, a.Created.Format(time.RFC3339), r.CIDR.String(), r.Type, r.State}
		rows = append(rows, row)
	}
	return writeTable(w, rows)
}

func listSupernets(w io.Writer, c *cloudconnect.Config) error {
	var (
		supernets        = c.ListSupernets()
		allocationsCount = make(map[string]int, len(supernets))
		usedIPs          = make(map[string]int, len(supernets))
	)

	for _, subnet := range c.ListSubnets() {
		for _, s := range supernets {
			if s.Includes(subnet) {
				usedIPs[s.String()] += subnet.AddressCount()
				allocationsCount[s.String()]++
			}
		}
	}

	if err := printSupernets(w, supernets, allocationsCount, usedIPs); err != nil {
		return err
	}
	return nil
}

func printSupernets(w io.Writer, supernets []*cloudconnect.CIDR, allocationsCount map[string]int, usedIPs map[string]int) error {
	header := []string{"Supernet", "# of allocations", "IPs (used/total)"}
	divide := []string{"--------", "----------------", "----------------"}

	var rows [][]string
	rows = append(rows, header, divide)

	for _, s := range supernets {
		var (
			total = s.AddressCount()
			used  = usedIPs[s.String()]
			count = allocationsCount[s.String()]
		)
		row := []string{s.String(), strconv.Itoa(count), fmt.Sprintf("%d/%d", used, total)}
		rows = append(rows, row)
	}
	return writeTable(w, rows)

}

func addPlanCommand(app *kingpin.Application, ec2Factory ec2Factory, managerFactory managerFactory, w io.Writer) {
	var (
		cmd    = app.Command("plan", "Plan changes to transit gateway based on the specified config")
		file   = cmd.Arg("config", "Path to a config file").Required().ExistingFile()
		region = cmd.Flag("region", "AWS Region to target").Envar("AWS_REGION").Required().String()
	)

	cmd.Action(func(_ *kingpin.ParseContext) error {
		c := readConfig(*file)
		g, ok := c.Gateways[*region]
		if !ok {
			kingpin.Fatalf("plan: no gateway config for region: %s", *region)
		}
		m := managerFactory(ec2Factory(*region), g.ID, g.RouteTableID, true)
		if _, err := planChanges(w, m, c, *region); err != nil {
			kingpin.Fatalf("plan changes: %s", err)
		}
		return nil
	})
}

func planChanges(w io.Writer, m cloudconnect.Manager, c *cloudconnect.Config, region string) ([]*cloudconnect.AttachmentChange, error) {
	attachments, err := m.ListAttachments()
	if err != nil {
		return nil, err
	}
	changes, err := cloudconnect.PlanAll(m, attachments, c.Allocations(), region)
	if err != nil {
		return nil, err
	}
	if err := printPlan(w, changes); err != nil {
		return nil, err
	}
	return changes, nil
}

func printPlan(w io.Writer, changes []*cloudconnect.AttachmentChange) error {
	header := []string{"Attachment ID", "Name", "Owner", "Type", "State", "Created", "Action", "Reason"}
	divide := []string{"-------------", "----", "-----", "----", "-----", "-------", "------", "------"}

	var rows [][]string
	rows = append(rows, header, divide)

	for _, c := range changes {
		a := c.Attachment
		row := []string{string(a.ID), a.Tags["Name"], a.Owner, a.Type, a.State, a.Created.Format(time.RFC3339), string(c.Action), c.Reason}
		rows = append(rows, row)
	}
	return writeTable(w, rows)
}

func addApplyCommand(app *kingpin.Application, ec2Factory ec2Factory, managerFactory managerFactory, w io.Writer) {
	var (
		cmd         = app.Command("apply", "Apply changes to transit gateway")
		file        = cmd.Arg("config", "Path to a config file").Required().ExistingFile()
		region      = cmd.Flag("region", "AWS Region to target").Envar("AWS_REGION").Required().String()
		autoApprove = cmd.Flag("auto-approve", "Auto approve the apply").Bool()
		dryRun      = cmd.Flag("dry-run", "Use the dry-run option for AWS API requests. Useful for debugging and checking whether the user has permission to perform the apply.").Bool()
	)

	cmd.Action(func(_ *kingpin.ParseContext) error {
		c := readConfig(*file)
		g, ok := c.Gateways[*region]
		if !ok {
			kingpin.Fatalf("apply: no gateway config for region: %s", *region)
		}
		m := managerFactory(ec2Factory(*region), g.ID, g.RouteTableID, *dryRun)
		if err := applyChanges(w, m, c, *region, *autoApprove); err != nil {
			kingpin.Fatalf("appl changes: %s", err)
		}
		return nil
	})
}

func applyChanges(w io.Writer, m cloudconnect.Manager, c *cloudconnect.Config, region string, autoApprove bool) error {
	changes, err := planChanges(w, m, c, region)
	if err != nil {
		return err
	}

	if !autoApprove {
		reader := bufio.NewReader(os.Stdin)
		fmt.Fprintf(w, "\nApply changes? (Y/n): ")

		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read input: %s", err)
		}

		if strings.TrimSpace(input) != "Y" {
			fmt.Fprintln(w, "Aborting!")
			return nil
		}
		fmt.Fprintln(w, "Proceeding!")
	}
	fmt.Fprintln(w, "")

	var applyErrs int
	for _, c := range changes {
		fmt.Fprintf(w, "Applying change: %-8s on attachment: %s", c.Action, string(c.Attachment.ID))

		var result string
		if err := cloudconnect.Apply(m, c); err != nil {
			result = fmt.Sprintf(" - ERROR: %s", err.Error())
			applyErrs++
		} else {
			result = " - SUCCESS"
		}
		fmt.Fprintln(w, result)
	}

	if applyErrs > 0 {
		return errors.New("one or more changes failed")
	}
	return nil
}

func readConfig(f string) *cloudconnect.Config {
	b, err := ioutil.ReadFile(f)
	if err != nil {
		kingpin.Fatalf("read config: %s", err.Error())
	}
	var c cloudconnect.Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		kingpin.Fatalf("unmarshal config: %s", err.Error())
	}
	return &c
}

func writeTable(w io.Writer, rows [][]string) error {
	table := tabwriter.NewWriter(w, 0, 8, 1, ' ', 0)
	for _, row := range rows {
		if _, err := fmt.Fprintln(table, "|\t"+strings.Join(row, "\t|\t")+"\t|"); err != nil {
			return err
		}
	}
	return table.Flush()
}
