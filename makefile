
default: push

.EXPORT_ALL_VARIABLES:

# Container
# Specify the tag with git : git tag <value> <hash>
TAG=`git describe --tags`
DATE=`date +%FT%T%z`
GITSHA=`git rev-parse HEAD`
PREFIX=sebs42

# Testing
TESTING_NAMESPACE=default
TESTING_DB_IMAGE_START=sebs42/mariadb:10.4.2-bionic
TESTING_DB_IMAGE_UPGRADE=sebs42/mariadb:10.4.12-bionic

# API
API_VERSION=v1beta2

# Name of app/binary
APP_NAME=galera-operator

# Binary output dir
OUTPUT_DIR=bin

# Building LDFLAGS
LDFLAGS=-ldflags="-w -s -X galera-operator/pkg/version.Version=$(TAG) -X galera-operator/pkg/version.Date=$(DATE) -X galera-operator/pkg/version.GitSHA=$(GITSHA)"


.PHONY: build container push clean codegen test

build:
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o $(OUTPUT_DIR)/$(APP_NAME) cmd/galera-operator/main.go

container: build
	docker build -t $(PREFIX)/$(APP_NAME):$(TAG) .

push: container
	docker push $(PREFIX)/$(APP_NAME):$(TAG)

clean:
	rm -rf $(OUTPUT_DIR)
	docker rmi -f "$(PREFIX)/$(APP_NAME):$(TAG)" || true

codegen: clean
	./hack/update-codegen.sh
	rm ./pkg/client/listers/apigalera/$(API_VERSION)/expansion_generated.go

unittest:
	go test -v ./pkg/galera
	go test -v ./pkg/controllers/cluster

inite2etest:
	./test/init_e2etest.sh

e2etest: inite2etest
	go test -v ./test/e2e --kubeconfig=/Users/seb/.kube/config --operator-image=$(PREFIX)/$(APP_NAME):$(TAG) --namespace=$(TESTING_NAMESPACE)

