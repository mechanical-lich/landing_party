name = "landing_party"
GIT_TAG := $(shell git describe --tags --always --dirty)
LDFLAGS := -ldflags "-X github.com/mechanical-lich/landing_party/internal/buildinfo.Version=$(GIT_TAG)"

bin:
	mkdir bin
	cp -r ./assets ./bin/assets
	cp -r ./data ./bin/data

.PHONY: clean
clean:
	rm -rf ./bin

run:
	go run $(LDFLAGS) ./cmd/game/*.go

run-planet-generator:
	go run $(LDFLAGS) ./cmd/planet-generator

run-debug:
	GODEBUG=gctrace=1 go run $(LDFLAGS) ./cmd/game/*.go

build: clean bin
	go build $(LDFLAGS) -o ./bin/$(name)

build-all: clean build-mac build-windows build-linux
	@echo "Builds for all platforms are complete."

build-mac: bin
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o ./bin/$(name)-mac

build-linux: bin
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o ./bin/$(name)-linux

build-windows: bin
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o ./bin/$(name)-windows.exe

.PHONY: update-go-deps
update-go-deps:
	@echo ">> updating Go dependencies"
	@for m in $$(go list -mod=readonly -m -f '{{ if and (not .Indirect) (not .Main)}}{{.Path}}{{end}}' all); do \
		go get $$m; \
	done
	go mod tidy
ifneq (,$(wildcard vendor))
	go mod vendor
endif

# ---- VM play-testing (docs/developer/vmtest.md; the host is mlge's cmd/vmtest,
# a go.mod tool, so it is built against this module's Ebiten) ----
# make vmtest SCRIPT=new_expedition LABEL=before
# make vmtest-diff SCRIPT=new_expedition A=before B=after
SCRIPT ?= new_expedition
LABEL ?=
VMTEST := go tool vmtest
VMTEST_FLAGS := $(if $(LABEL),-label $(LABEL))

.PHONY: vmtest vmtest-all vmtest-diff vmtest-targets vmtest-check
vmtest:
	$(VMTEST) run $(VMTEST_FLAGS) $(SCRIPT)

vmtest-targets:
	$(VMTEST) targets $(SCRIPT)

vmtest-all:
	@fail=0; for s in vmtest/scripts/*.jsonl; do \
		$(VMTEST) run $(VMTEST_FLAGS) $$s || fail=1; \
	done; exit $$fail

vmtest-diff:
	$(VMTEST) diff $(SCRIPT) $(A) $(B)

# Proves release builds carry no guest code (see vmtest.json's check_symbols).
# Both binaries are left in vmtest/out/.bin (gitignored).
vmtest-check:
	@$(VMTEST) check
