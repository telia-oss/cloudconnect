## cloudconnect

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/telia-oss/cloudconnect)
[![latest release](https://img.shields.io/github/v/release/telia-oss/cloudconnect?style=flat-square)](https://github.com/telia-oss/cloudconnect/releases/latest)
[![build status](https://img.shields.io/github/workflow/status/telia-oss/cloudconnect/test?label=build&logo=github&style=flat-square)](https://github.com/telia-oss/cloudconnect/actions?query=workflow%3Atest)
[![code quality](https://goreportcard.com/badge/github.com/telia-oss/cloudconnect?style=flat-square)](https://goreportcard.com/report/github.com/telia-oss/cloudconnect)

Cloud connect provides a CLI (`cloud-connect`) and Lambda function (`autoapprover`) for managing CIDR allocations and 
attachments for a multi-tenant (and multi-region) setup of AWS Transit Gateway. This is done using a YAML configuration
that contains CIDR allocations (see [example/allocations.yml](./example/allocations.yml)) and using the `autoapprover` 
to manage transit gateway attachments and routes.

## CLI

#### Installation

Use [homebrew](https://brew.sh/) to install the latest version on OS X and Linux: `brew install telia-oss/tap/cloud-connect`.
Otherwise you can install `cloud-connect` by downloading it from the [releases](https://github.com/telia-oss/cloudconnect/releases).

#### Usage

```
$ cloud-connect --help
usage: cloud-connect [<flags>] <command> [<args> ...]

CLI for managing cloud connect

Flags:
  --help  Show context-sensitive help (also try --help-long and --help-man).

Commands:
  help [<command>...]
    Show help.

  format <file>...
    Format config file

  validate <file>...
    Validate config file

  next-cidr --supernet=SUPERNET [<flags>] <file>
    Get the next available CIDR

  list attachments --region=REGION <file>
    List transit gateway attachments

  list routes --region=REGION <file>
    List transit gateway routes

  list supernets <file>
    List available supernets

  plan --region=REGION <config>
    Plan changes to transit gateway based on the specified config

  apply --region=REGION [<flags>] <config>
    Apply changes to transit gateway
```

## Autoapprover

#### Installation

You can download the latest version of `autoapprover` from the [releases](https://github.com/telia-oss/cloudconnect/releases),
or you can use the pre-packaged zip files available from our public S3 bucket and reference it directly in your terraform:

```hcl
data "aws_region" "current" {}

module "lambda" {
  source  = "telia-oss/lambda/aws"
  version = "3.0.0"

  name_prefix = "autoapprover"
  s3_bucket   = "telia-oss-${data.aws_region.current.name}"
  s3_key      = "autoapprover/v0.2.0.zip"
  handler     = "autoapprover"

  ...
}
```

#### Usage

The `autoapprover` is a Lambda function, but can also be run locally for development purposes:

```bash
$ autoapprover --help
usage: autoapprover --config-bucket=CONFIG-BUCKET --config-path=CONFIG-PATH --region=REGION [<flags>]

A lambda handler for managing cloud connect

Flags:
  --help                         Show context-sensitive help (also try --help-long and --help-man).
  --config-bucket=CONFIG-BUCKET  S3 bucket where the config is stored
  --config-path=CONFIG-PATH      Path to the config file (in the S3 bucket)
  --region=REGION                AWS Region to target
  --local                        Run the handler in local mode (i.e. not inside a Lambda)
  --dry-run                      Use the dry-run option for AWS API requests (no side-effects).
  --debug                        Enable debug logging.
```

I.e. you can use `--local` and `--dry-run` to test the code locally without side effects.

#### Environment variables

As with the CLI, flags can be set via the environment:

```
# For staging
export AUTOAPPROVER_CONFIG_BUCKET=dc-stage-autoapprover
export AUTOAPPROVER_CONFIG_PATH=allocations.yml 

# For production
export AUTOAPPROVER_CONFIG_BUCKET=dc-prod-autoapprover
export AUTOAPPROVER_CONFIG_PATH=allocations.yml 
```

After setting the above, you can run the `autoapprover` locally like this:

```bash
./build/autoapprover --dry-run --local --debug
```
