# Terraform Provider Mimirtool

Terraform provider [mimirtool](https://grafana.com/docs/mimir/latest/operators-guide/tools/mimirtool/). It can use to execute a number of common tasks that involve Grafana Mimir or Grafana Cloud Metrics.

## Requirements

-	[Terraform](https://www.terraform.io/downloads.html) >= 1.1.6
-	[Go](https://golang.org/doc/install) >= 1.19

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:
```sh
$ go install
```

To learn more about how to overrides the provider built locally have a look at [the developper documentation](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```sh
$ make testacc
```

### Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Documentation

Documentation is generated with
[tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs). Generated
files are in `docs/` and should not be updated manually. They are derived from:

- Schema `Description` fields in the provider Go code.
- [examples/](./examples)
- [templates/](./templates)

Use `go generate` to update generated docs.

## Releasing

Builds and releases are automated with GitHub Actions and
[GoReleaser](https://github.com/goreleaser/goreleaser/).

Currently there are a few manual steps to this:

1. Kick off the release:

   ```sh
   RELEASE_VERSION=v... \
   make release
   ```

2. Publish release:

   The Action creates the release, but leaves it in "draft" state. Open it up in
   a [browser](https://github.com/grafana/terraform-provider-grafana/releases)
   and if all looks well, click the `Auto-generate release notes` button and mash the publish button.
