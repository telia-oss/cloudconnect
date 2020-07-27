package main

import (
	"os"

	"github.com/telia-oss/cloudconnect"
	"github.com/telia-oss/cloudconnect/internal/cli"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	app := kingpin.New("cloud-connect", "CLI for managing cloud connect").DefaultEnvars()
	cli.Setup(app, ec2Factory, cloudconnect.NewManager, os.Stdout)
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func ec2Factory(region string) cloudconnect.EC2API {
	// AWS_REGION is checked by kingpin
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		kingpin.Fatalf("create new aws session: %s", err.Error())
	}
	return ec2.New(sess)
}
