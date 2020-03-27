
default: push

.EXPORT_ALL_VARIABLES:

# Container
# Specify the tag with git : git tag <value> <hash>
TAG=`git describe --tags`
DATE=`date +%FT%T%z`
GITSHA=`git rev-parse HEAD`
PREFIX=

# Testing
GB_S3Login=
GB_S3Password=
GB_S3Endpoint=
GB_S3Bucket=
GB_S3BackupName=
GB_Backup=
GB_Restore=

# Name of app/binary
APP_NAME=galera-bootstrap

# Binary output dir
OUTPUT_DIR=bin

# Building LDFLAGS
LDFLAGS=-ldflags="-w -s -X galera-bootstrap/pkg/version.Version=$(TAG) -X galera-bootstrap/pkg/version.Date=$(DATE) -X galera-bootstrap/pkg/version.GitSHA=$(GITSHA)"

.PHONY: build container push clean test dev

build:
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o $(OUTPUT_DIR)/$(APP_NAME) main.go

container: build
	docker build -t $(PREFIX)/$(APP_NAME):$(TAG) .

push: container
	docker push $(PREFIX)/$(APP_NAME):$(TAG)

clean:
	rm -rf $(OUTPUT_DIR)

test:
	go test

dev:
	GOARCH=amd64 go build ${LDFLAGS} -o $(OUTPUT_DIR)/$(APP_NAME) main.go
