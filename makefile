
default: push

.EXPORT_ALL_VARIABLES:

# Container
# Specify the tag with git : git tag <value> <hash>
TAG=`git describe --tags`
DATE=`date +%FT%T%z`
GITSHA=`git rev-parse HEAD`
#PREFIX=
#CODE_GEN=v0.15.9

# Testing
#TESTING_NAMESPACE=
#TESTING_DB_IMAGE_START=
#TESTING_DB_IMAGE_UPGRADE=
#TESTING_S3_ACCESS_KEY_ID=
#TESTING_S3_SECRET_ACCESS_KEY=
#TESTING_S3_ENDPOINT=
#TESTING_S3_BUCKET=

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

clonegen:
	git clone -b $(CODE_GEN) https://github.com/kubernetes/code-generator ./vendor/k8s.io/code-generator

codegen:
	./hack/update-codegen.sh
	rm ./pkg/client/listers/apigalera/$(API_VERSION)/expansion_generated.go

unittest:
	go test -v ./pkg/galera
	go test -v ./pkg/controllers/cluster

inite2etest:
	./test/init_e2etest.sh

e2etest: inite2etest
	go test -d -v ./test/e2e --kubeconfig=/Users/seb/.kube/config --operator-image=$(PREFIX)/$(APP_NAME):$(TAG) --namespace=$(TESTING_NAMESPACE)

