<div align="center">
  <h1>Customer Connect Terraform Provider</h1>
</div>

-------------------------------------

The Customer Connect Terraform Provider allows you to manage and configure NetFoundry Customer Connect resources as code using
[Terraform](https://www.terraform.io/).


## Getting started

Configuring [required providers](https://www.terraform.io/docs/language/providers/requirements.html#requiring-providers):

```terraform
terraform {
  required_providers {
    customer-connect = {
      source  = "netfoundry/customer-connect"
    }
  }
}
```


### Authentication

The Customer Connect provider authenticates against the NetFoundry API using **Congnito client credentials**. Credentials can be supplied as static values in the provider block or via environment variables.

#### Static credentials

```terraform
provider "customer-connect" {
  client_id     = "your-client-id"
  client_secret = "your-client-secret"
  environment   = "production"
}
```

#### Environment variables

Set `NF_CLIENT_ID`, `NF_CLIENT_SECRET`, and optionally `NF_ENVIRONMENT` before running Terraform:

```terraform
provider "customer-connect" {}
```

#### Provider arguments

| Argument | Description | Default |
|---|---|---|
| `client_id` | Cognito OAuth2 client ID. Env: `NF_CLIENT_ID`. | — |
| `client_secret` | Cognito OAuth2 client secret (sensitive). Env: `NF_CLIENT_SECRET`. | — |
| `environment` | Deploy environment: `sandbox`, `staging`, or `production`. Env: `NF_ENVIRONMENT`. | `production` |
| `auth_url` | Override the Cognito OAuth2 token endpoint. Inferred from `environment` when omitted. | — |


## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements)).

For local plugin development see [Local Plugin Development](#local-plugin-development) before building the provider.

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.


### Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.8+
- [Go](https://golang.org/doc/install) >= >= 1.21+


### Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the `go install` command:

   ```sh
   go install
   ```


### Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```
go get github.com/author/dependency
go mod tidy
```


## Local Plugin Development

Add the below snippet at `~/.terraformrc` on your machine.

```
provider_installation {

  dev_overrides {
    # point to local go path for compiled binaries
    "netfoundry/customer-connect" = "/path/to/go/bin"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```

> **Note:** Do not run `terraform init` during local development.