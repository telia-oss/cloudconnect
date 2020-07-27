package main

import (
	"os"
	"time"

	"github.com/telia-oss/cloudconnect"
	"github.com/telia-oss/cloudconnect/internal/autoapprover"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	app := kingpin.New("autoapprover", "A lambda handler for managing cloud connect").DefaultEnvars()
	autoapprover.Setup(app, lambda.Start, awsClientFactory, cloudconnect.NewManager, loggerFactory)
	kingpin.MustParse(app.Parse(os.Args[1:]))
}

func awsClientFactory(region string) (autoapprover.S3API, cloudconnect.EC2API, error) {
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return nil, nil, err
	}
	return s3.New(sess), ec2.New(sess), nil

}

func loggerFactory(debug bool) (*zap.Logger, error) {
	config := zap.NewProductionConfig()

	// Disable entries like: "caller":"autoapprover/autoapprover.go:97"
	config.DisableCaller = true

	// Disable logging the stack trace
	config.DisableStacktrace = true

	// Format timestamps as RFC3339 strings
	// Adapted from: https://github.com/uber-go/zap/issues/661#issuecomment-520686037
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoder(
		func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.UTC().Format(time.RFC3339))
		},
	)

	if debug {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}
	return config.Build()
}
