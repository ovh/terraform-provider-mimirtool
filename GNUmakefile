MIMIR_VERSION ?= 2.6.0

default: testacc

# Run acceptance tests
.PHONY: testacc
testacc: compose-up
	TF_ACC=1 MIMIRTOOL_ADDRESS=http://localhost:8080 go test ./... -v $(TESTARGS) -timeout 120m

build:
	go build -o dist/

compose-up: compose-down
	MIMIR_VERSION=$(MIMIR_VERSION) docker-compose -f ./docker-compose.yml up -d
	curl -s --retry 12 -f --retry-all-errors --retry-delay 10 http://localhost:8080/ready

compose-down:
	docker-compose -f ./docker-compose.yml stop

docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

release:
	@test $${RELEASE_VERSION?Please set environment variable RELEASE_VERSION}
	@git tag $$RELEASE_VERSION
	@git push origin $$RELEASE_VERSION

clean: compose-down
	rm -rf dist/*
