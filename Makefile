name = "landing_party"
GIT_TAG := $(shell git describe --tags --always --dirty)
LDFLAGS := -ldflags "-X github.com/mechanical-lich/landing_party/internal/buildinfo.Version=$(GIT_TAG)"

bin:
	mkdir bin
	cp -r ./assets ./bin/assets
	cp -r ./data ./bin/data

.PHONY: clean history-dump
clean:
	rm -rf ./bin

run:
	go run $(LDFLAGS) ./cmd/game/*.go

run-planet-generator:
	go run $(LDFLAGS) ./cmd/planet-generator

# Print a campaign's generated prehistory (docs/brainstorm/lore_plan.md).
# make history-dump HSEED=42 HSITES=8 HMAP=moon HFREQ=500
# make history-dump HSEED=42 HCHARTED=1      (a real charted campaign)
# make history-dump HCAMPAIGNS=300 HFREQ=0   (scenario share through charting)
# make history-dump HFIND=hazard,dormant_nest (seeds for vmtest scripts)
# make history-dump HSTATS=500               (density: events, origins, stamps per site)
# make history-dump HFACTIONS=0,1,2,3 HFSEEDS=1-5 HFDAYS=100 (rival factions over time; HFDETAIL=1 lists them, HFCOLS=0,40,100,200,300 picks the days)
HSEED ?= 1
HSITES ?= 8
HMAP ?=
HFREQ ?= 500
HCHARTED ?=
HCAMPAIGNS ?= 0
HFIND ?=
HSTATS ?=
HFACTIONS ?=
HFSEEDS ?= 1-5
HFDAYS ?= 100
HFDETAIL ?=
HFCOLS ?=
history-dump:
	go run ./cmd/history-dump -seed $(HSEED) -sites $(HSITES) -freq $(HFREQ) $(if $(HMAP),-map $(HMAP)) $(if $(HCHARTED),-charted) -campaigns $(HCAMPAIGNS) $(if $(HFIND),-find $(HFIND)) $(if $(HSTATS),-stats $(HSTATS)) $(if $(HFACTIONS),-factions $(HFACTIONS) -fseeds $(HFSEEDS) -fdays $(HFDAYS)) $(if $(HFDETAIL),-fdetail) $(if $(HFCOLS),-fcols $(HFCOLS))

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

# ---- Observer soak (docs/developer/factions.md, "Observer mode"): ten seeds,
# the director running a landed party for 20,000 ticks, the expedition soak
# (ten seeds, the expedition director flying the ship from adrift), then the
# autonz soak (phase E3: ten node seeds and five hub seeds, AUTONZTICKS,
# 30,000) and the hive soak (HIVETICKS, 30,000: forty days); prints every
# table. make soak SOAKSEEDS=3 SOAKTICKS=6000 for a shorter run.
#
# The greyish soak (phase E4: ten scout seeds and five warship seeds,
# GREYTICKS, 22,500: one 30-day stay; a row ends at lift-off).
#
# make soak-autonz, make soak-hive and make soak-greyish run one soak alone, its seeds as
# separate processes side by side (SOAKJOBS at once, scripts/soak_par.sh),
# joined into the same table and thresholds. make soak-autonz SOAKSEEDS=3
# AUTONZTICKS=9000 for a quick tuning round (HUBSEEDS: the hub seeds,
# default half the node seeds).
SOAKSEEDS ?= 10
SOAKTICKS ?= 20000
HIVETICKS ?= 30000
AUTONZTICKS ?= 30000
GREYTICKS ?= 22500
HUBSEEDS ?= $(shell echo $$(( ($(SOAKSEEDS) + 1) / 2 )))
SOAKJOBS ?= 10
# upto lists 1..N, none for 0 (BSD seq 1 0 counts down: "1 0").
upto = $(shell i=1; while [ $$i -le $(1) ]; do echo $$i; i=$$((i+1)); done)
.PHONY: soak soak-autonz soak-hive soak-greyish
soak:
	LP_SOAK=1 LP_SOAK_SEEDS=$(SOAKSEEDS) LP_SOAK_TICKS=$(SOAKTICKS) go test ./internal/game -run "TestObserverSoak|TestExpeditionSoak" -v -count=1 -timeout 30m
	$(MAKE) soak-autonz
	$(MAKE) soak-hive
	$(MAKE) soak-greyish

soak-autonz:
	LP_SOAK_AUTONZ_TICKS=$(AUTONZTICKS) sh scripts/soak_par.sh TestAutonzSoak TestAutonzSoakJoin $(SOAKJOBS) \
		$(addprefix node:,$(call upto,$(SOAKSEEDS))) $(addprefix hub:,$(call upto,$(HUBSEEDS)))

soak-hive:
	LP_SOAK_HIVE_TICKS=$(HIVETICKS) sh scripts/soak_par.sh TestHiveSoak TestHiveSoakJoin $(SOAKJOBS) \
		$(addprefix hive:,$(call upto,$(SOAKSEEDS)))

soak-greyish:
	LP_SOAK_GREY_TICKS=$(GREYTICKS) sh scripts/soak_par.sh TestGreyishSoak TestGreyishSoakJoin $(SOAKJOBS) \
		$(addprefix scout:,$(call upto,$(SOAKSEEDS))) $(addprefix warship:,$(call upto,$(HUBSEEDS)))

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
