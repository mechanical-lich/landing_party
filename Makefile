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

# ---- VM play-testing (docs/developer/vmtest.md) ----
# make vmtest SCRIPT=new_expedition LABEL=before
# make vmtest-diff SCRIPT=new_expedition A=before B=after
SCRIPT ?= new_expedition
LABEL ?=
VMTEST_BIN_DIR := vmtest/out/.bin
VMTEST_BIN := $(VMTEST_BIN_DIR)/vmtest
VMTEST_FLAGS := $(if $(LABEL),-label $(LABEL))

.PHONY: vmtest-host vmtest vmtest-all vmtest-diff vmtest-targets vmtest-check
vmtest-host:
	@mkdir -p $(VMTEST_BIN_DIR)
	@go build -o $(VMTEST_BIN) ./cmd/vmtest

vmtest: vmtest-host
	$(VMTEST_BIN) run $(VMTEST_FLAGS) $(SCRIPT)

vmtest-targets: vmtest-host
	$(VMTEST_BIN) targets $(SCRIPT)

vmtest-all: vmtest-host
	@fail=0; for s in vmtest/scripts/*.jsonl; do \
		$(VMTEST_BIN) run $(VMTEST_FLAGS) $$s || fail=1; \
	done; exit $$fail

vmtest-diff: vmtest-host
	$(VMTEST_BIN) diff $(SCRIPT) $(A) $(B)

# Proves release builds carry no guest code: cmd/game must not depend on the
# VM host or the vmtest packages, and an untagged binary must not contain the
# sidecar entry point. The tagged build must contain it, so the check can fail.
# Both binaries are left in vmtest/out/.bin (gitignored).
vmtest-check:
	@mkdir -p $(VMTEST_BIN_DIR)
	@n=$$(go list -deps ./cmd/game | grep -c 'exp/vmhost\|landing_party/internal/vmtest\|landing_party/internal/vmproto'); \
	if [ "$$n" != 0 ]; then echo "vmtest-check: cmd/game depends on vmhost/vmtest/vmproto ($$n packages)"; exit 1; fi
	@go build -o $(VMTEST_BIN_DIR)/lp-release ./cmd/game
	@go build -tags ebitenginevmguest -o $(VMTEST_BIN_DIR)/lp-guest ./cmd/game
	@rel=$$(go tool nm $(VMTEST_BIN_DIR)/lp-release | grep -c vmStartServer); \
	guest=$$(go tool nm $(VMTEST_BIN_DIR)/lp-guest | grep -c vmStartServer); \
	if [ "$$rel" != 0 ]; then echo "vmtest-check: release binary contains vmStartServer"; exit 1; fi; \
	if [ "$$guest" = 0 ]; then echo "vmtest-check: guest binary lacks vmStartServer, so this check proves nothing"; exit 1; fi; \
	echo "vmtest-check: ok (release binary has no guest hook; guest binary has it)"
