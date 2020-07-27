## cloud-connect

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/telia-oss/cloudconnect)
[![latest release](https://img.shields.io/github/v/release/telia-oss/cloudconnect?style=flat-square)](https://github.com/telia-oss/cloudconnect/releases/latest)
[![build status](https://img.shields.io/github/workflow/status/telia-oss/cloudconnect/test?label=build&logo=github&style=flat-square)](https://github.com/telia-oss/cloudconnect/actions?query=workflow%3Atest)
[![code quality](https://goreportcard.com/badge/github.com/telia-oss/cloudconnect?style=flat-square)](https://goreportcard.com/report/github.com/telia-oss/cloudconnect)

Cloud connect provides a CLI and Lambda function for managing CIDR allocations and attachments for a multi-tenant (and
multi-region) setup of AWS Transit Gateway.

## CLI

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

#### Usage

The autoapprover needs to be built with `task build` and then deployed for the respective environment under `terraform/<env>/autoapprover`. For
development purposes it can be run locally like so:

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

You can use `--local` and `--dry-run` to test the code locally without side effects.

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
