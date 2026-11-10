module github.com/mechanical-lich/landing_party

go 1.26.0

toolchain go1.26.4

// rand.Seed seeds the global math/rand source again (a no-op since Go 1.24):
// the simulation (needs, scripts, the GameMaster, combat dice here and in
// mlge/ml-rogue-lib) draws from it, and seedSimRand seeds it from the
// campaign so the same seed plays the same way (soaks, saves).
godebug randseednop=0

//replace github.com/mechanical-lich/mechanical-basic => ../mechanical-basic
//replace github.com/mechanical-lich/mlge => ../mlge

//replace github.com/mechanical-lich/ml-rogue-lib => ../ml-rogue-lib

require (
	github.com/aquilax/go-perlin v1.1.0
	github.com/hajimehoshi/ebiten/v2 v2.10.5
	github.com/mechanical-lich/mechanical-basic v1.2.0
	github.com/mechanical-lich/ml-rogue-lib v1.15.0
	github.com/mechanical-lich/mlge v1.15.0
	github.com/stretchr/testify v1.12.1
	golang.org/x/image v0.47.0
)

require (
	github.com/ebitengine/gomobile v0.0.0-20260820040257-d11f821a26a6 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/oto/v3 v3.5.1 // indirect
	github.com/ebitengine/purego v0.11.2 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/hajimehoshi/go-mp3 v0.3.4 // indirect
	github.com/jfreymuth/oggvorbis v1.0.5 // indirect
	github.com/jfreymuth/pulse v0.1.3 // indirect
	github.com/jfreymuth/vorbis v1.0.2 // indirect
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

tool github.com/mechanical-lich/mlge/cmd/vmtest
