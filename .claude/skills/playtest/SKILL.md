---
name: playtest
description: Play the real Landing Party UI headlessly from a script and look at the frames. Use to see a change on screen, check a flow end to end (title, Star Map, site, modals, tutorial), or compare before/after frames of a fix.
---

# Play-test the game in a VM

`go tool vmtest` (mlge's vmtest, a go.mod tool) runs the game as an
Ebitengine VM guest (no window, no desktop input), plays
`vmtest/scripts/<name>.jsonl`, and writes frames and state to
`vmtest/out/<name>/<label>/`. Game reference: `docs/developer/vmtest.md`;
engine reference: mlge `docs/vmtest.md`.

## Steps

1. Run a starter script, or write one under `vmtest/scripts/`:

   ```bash
   make vmtest SCRIPT=new_expedition LABEL=before
   ```

   Starters: `new_expedition`, `tutorial`, `incident`, `save_load`,
   `storage_beam`, `esc_menu`. `make vmtest-all` runs them all.

2. Read `vmtest/out/<script>/<label>/log.txt`, `result.json`, then the
   `NNNN_<name>.png` frames with the image tool. Frames are 1152x800, the same
   coordinates clicks use. Each frame has a `state_NNNN.json`.

3. On a failure, read `NNNN_FAIL.png` and the target list at the end of
   `log.txt`. To list resolvable targets at the end of a script:
   `make vmtest-targets SCRIPT=<script>`.

4. After a fix, rerun with another label and compare:

   ```bash
   make vmtest SCRIPT=new_expedition LABEL=after
   ```

   ```bash
   make vmtest-diff SCRIPT=new_expedition A=before B=after
   ```

   Up to ~0.2% pixel noise is normal (wall-clock animation); state should match.

## Rules

- Scripts click named targets (`title:new`, `dash:travelBtn`, `hud:build_bed`,
  `entity:storage_locker`, `settle:0,-2`), not coordinates, when a name exists.
- `wait` is ticks, not seconds. Use `wait_until` with a `max_ticks` budget for
  anything the colonists have to do.
- Never run the guest from the repo or with `data/config.local.json`; the tool
  sandboxes it already.
- Do not use `git stash` to get a "before" run; run from a `git worktree`.
- Release builds must stay clean: `make vmtest-check`.
