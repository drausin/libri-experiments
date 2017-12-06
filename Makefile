SHELL=/bin/bash -eou pipefail
GOTOOLS= github.com/alecthomas/gometalinter \
	 github.com/wadey/gocovmerge
LIBRI_PKGS=$(shell go list ./... | grep -v /vendor/)
LIBRI_PKG_SUBDIRS=$(shell go list ./... | grep -v /vendor/ | sed -r 's|github.com/drausin/libri-experiments/||g' | sort)
GIT_STATUS_SUBDIRS=$(shell git status --porcelain | grep -e '\.go$$' | sed -r 's|^...(.+)/[^/]+\.go$$|\1|' | sort | uniq)
GIT_DIFF_SUBDIRS=$(shell git diff develop..HEAD --name-only | grep -e '\.go$$' | sed -r 's|^(.+)/[^/]+\.go$$|\1|' | sort | uniq)
GIT_STATUS_PKG_SUBDIRS=$(shell echo $(LIBRI_PKG_SUBDIRS) $(GIT_STATUS_SUBDIRS) | tr " " "\n" | sort | uniq -d)
GIT_DIFF_PKG_SUBDIRS=$(shell echo $(LIBRI_PKG_SUBDIRS) $(GIT_DIFF_SUBDIRS) | tr " " "\n" | sort | uniq -d)


.PHONY: bench build

build:
	@echo "--> Running go build"
	@go build $(LIBRI_PKGS)

build-static:
	@echo "--> Running go build for static binary"
	@./scripts/build-static.sh deploy/bin/libri-sim

docker-image:
	@echo "--> Building docker image"
	@docker build --rm=false -t daedalus2718/libri-sim:snapshot deploy

fix:
	@echo "--> Running goimports"
	@find . -name *.go | grep -v /vendor/ | xargs goimports -l -w

get-deps:
	@echo "--> Getting dependencies"
	@go get -u github.com/golang/dep/cmd/dep
	@dep ensure
	@go get -u -v $(GOTOOLS)
	@gometalinter --install

install-git-hooks:
	@echo "--> Installing git-hooks"
	@./scripts/install-git-hooks.sh

lint:
	@echo "--> Running gometalinter"
	@gometalinter $(LIBRI_PKG_SUBDIRS) --config=.gometalinter.json --deadline=5m

lint-diff:
	@echo "--> Running gometalinter on packages with uncommitted changes"
	@echo $(GIT_STATUS_PKG_SUBDIRS) | tr " " "\n"
	@echo $(GIT_STATUS_PKG_SUBDIRS) | xargs gometalinter --config=.gometalinter.json --deadline=5m

test:
	@echo "--> Running go test"
	@go test -race $(LIBRI_PKGS)


