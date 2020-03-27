
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
APP_NAME=galera-backup

# Binary output dir
OUTPUT_DIR=bin

# Building LDFLAGS
LDFLAGS=-ldflags="-w -s -X galera-backup/pkg/version.Version=$(TAG) -X galera-backup/pkg/version.Date=$(DATE) -X galera-backup/pkg/version.GitSHA=$(GITSHA)"

.PHONY: validate gen build container push clean test

# The `validate` target checks for errors and inconsistencies in
# our specification of an API. This target can check if we're
# referencing inexistent definitions and gives us hints to where
# to fix problems with our API in a static manner.
validate:
	swagger validate ./swagger/swagger.yaml

# The `gen` target depends on the `validate` target as
# it will only succesfully generate the code if the specification
# is valid.
#
# Here we're specifying some flags:
# --target              the base directory for generating the files;
# --spec                path to the swagger specification;
# --exclude-main        generates only the library code and not a
#                       sample CLI application;
# --name                the name of the application.
gen: validate
	swagger generate server \
		--target=./pkg \
		--spec=./swagger/swagger.yaml \
#		--exclude-main \
		--name=galera-backup
	rm -r ./cmd
	mv ./pkg/cmd ./

build:
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o $(OUTPUT_DIR)/$(APP_NAME) cmd/galera-backup-tool-server/main.go

container: build
	docker build -t $(PREFIX)/$(APP_NAME):$(TAG) .

push: container
	docker push $(PREFIX)/$(APP_NAME):$(TAG)

clean:
	rm -rf $(OUTPUT_DIR)
	rm -rf test/restore/

test:
	go build -ldflags="-w -s" -o test/e2e/$(APP_NAME) cmd/galera-backup-tool-server/main.go
	mkdir test/restore
	test/e2e/$(APP_NAME) --port=8080 & echo $$! > test/restore/pid
	go test -v test/e2e/galera-backup-client_test.go
	kill `cat test/restore/pid`
	rm -rf test/restore/
	rm test/e2e/$(APP_NAME)
