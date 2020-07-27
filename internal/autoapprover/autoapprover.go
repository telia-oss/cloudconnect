package autoapprover

import (
	"bytes"
	"fmt"
	"io"

	"github.com/telia-oss/cloudconnect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"go.uber.org/zap"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v3"
)

type (
	lambdaStartFunc  func(interface{})
	awsClientFactory func(string) (S3API, cloudconnect.EC2API, error)
	managerFactory   func(cloudconnect.EC2API, string, string, bool) cloudconnect.Manager
	loggerFactory    func(bool) (*zap.Logger, error)
)

// Setup a kingpin.Application to run the autoapprover.
func Setup(app *kingpin.Application, lambdaStartFunc lambdaStartFunc, awsClientFactory awsClientFactory, managerFactory managerFactory, loggerFactory loggerFactory) {
	var (
		bucket = app.Flag("config-bucket", "S3 bucket where the config is stored").Required().String()
		path   = app.Flag("config-path", "Path to the config file (in the S3 bucket)").Required().String()
		region = app.Flag("region", "AWS Region to target").Envar("AWS_REGION").Required().String()
		local  = app.Flag("local", "Run the handler in local mode (i.e. not inside a Lambda)").Bool()
		dryRun = app.Flag("dry-run", "Use the dry-run option for AWS API requests (no side-effects).").Bool()
		debug  = app.Flag("debug", "Enable debug logging.").Bool()
	)

	app.Action(func(_ *kingpin.ParseContext) error {
		logger, err := loggerFactory(*debug)
		if err != nil {
			panic(fmt.Errorf("initialize zap logger: %s", err))
		}
		defer logger.Sync()

		s3Client, ec2Client, err := awsClientFactory(*region)
		if err != nil {
			logger.Fatal("initialize aws clients", zap.Error(err))
		}

		c, err := readConfig(s3Client, *bucket, *path)
		if err != nil {
			logger.Fatal("read config", zap.Error(err))
		}
		if err := c.Validate(); err != nil {
			logger.Fatal("validate config", zap.Error(err))
		}
		g, ok := c.Gateways[*region]
		if !ok {
			kingpin.Fatalf("list attachments: no gateway config for region: %s", *region)
		}

		m := managerFactory(ec2Client, g.ID, g.RouteTableID, *dryRun)

		handler := newLambdaHandler(m, c, *region, logger)
		if lambdaStartFunc == nil || *local {
			lambdaStartFunc = newLocalStartFunc(logger)
		}

		lambdaStartFunc(handler)
		return nil
	})
}

// newLocalStartFunc returns a function with the same signature as lambda.Start
// and can be used to invoke the handler function locally, without needing to
// invoke it over RPC (as is done in a Lambda environment).
func newLocalStartFunc(logger *zap.Logger) lambdaStartFunc {
	return func(handler interface{}) {
		h := handler.(func() error)
		if err := h(); err != nil {
			logger.Fatal("handler execution", zap.Error(err))
		}
	}
}

func newLambdaHandler(m cloudconnect.Manager, c *cloudconnect.Config, region string, logger *zap.Logger) func() error {
	return func() error {
		attachments, err := m.ListAttachments()
		if err != nil {
			logger.Fatal("failed to list attachments", zap.Error(err))
		}
		allocations := c.Allocations()

		logger.Debug(
			"processing attachments",
			zap.Int("attachments", len(attachments)),
			zap.Int("allocations", len(allocations)),
		)

	Loop:
		for _, a := range attachments {
			log := logger.With(
				zap.String("id", string(a.ID)),
				zap.String("owner", a.Owner),
				zap.String("state", a.State),
			)

			log.Info("planning change")
			change, err := cloudconnect.Plan(m, a, allocations, region)
			if err != nil {
				log.Error("failed to plan change", zap.Error(err))
				continue Loop
			}
			log = log.With(zap.String("action", string(change.Action)))

			switch change.Action {
			case cloudconnect.NoOp:
				log.Debug("skipping no-op", zap.String("reason", change.Reason))
				continue Loop
			case cloudconnect.RejectAttachment, cloudconnect.DeleteAttachment:
				log.Warn("applying destructive change", zap.String("reason", change.Reason))
			default:
				log.Info("applying change", zap.String("reason", change.Reason))
			}
			if err := cloudconnect.Apply(m, change); err != nil {
				log.Error("failed to apply change", zap.Error(err))
				continue Loop
			}
			log.Info("apply done")
		}
		return nil
	}
}

// S3API wraps the interface for the API and provides a mocked implementation.
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . S3API
type S3API interface {
	GetObject(*s3.GetObjectInput) (*s3.GetObjectOutput, error)
}

func readConfig(client S3API, bucket, key string) (*cloudconnect.Config, error) {
	output, err := client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get object: %s", err)
	}
	defer output.Body.Close()

	b := bytes.NewBuffer(nil)
	if _, err := io.Copy(b, output.Body); err != nil {
		return nil, fmt.Errorf("copy config body: %s", err)
	}

	var c cloudconnect.Config
	if err := yaml.Unmarshal(b.Bytes(), &c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %s", err)
	}
	return &c, nil
}
