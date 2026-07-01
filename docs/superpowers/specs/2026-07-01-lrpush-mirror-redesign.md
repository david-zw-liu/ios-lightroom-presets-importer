# lrpush redesign: continuous mirror + live watcher

Date: 2026-07-01

## Summary

Replace lrpush's discrete subcommands (`inspect` / `push` / `rm` / `devices`)
with a single, flagless flow. Running bare `./lrpush`:

1. Picks a connected device.
2. Detects every installed Lightroom app on it.
3. For each app, mirrors the device's `userStyles` down into a local folder
   `./sync/{bundle-id}/userStyles/` (pull-and-replace).
4. Warns the user to close Lightroom, then watches each local folder and
   auto-pushes any change back up to the device in real time (including
   deletions), until the user hits Ctrl-C.

The local folder is an ephemeral working copy: edit presets there and they land
on the device. Nothing carries over between runs — `./sync/` is wiped at the
start of every run, so each session begins from a fresh device pull. (It is not
deleted on exit; it lingers on disk for inspection until the next startup wipe.)
Direction is never ambiguous — **device → local at startup, local → device
during the session** — so there is no conflict resolution.

## Non-goals

- No two-way merge, no persistent sync state, no digesting on startup (startup
  is a wholesale pull-and-replace).
- No backups (the startup pull is non-destructive to the device; local is
  disposable and re-pullable). The user is trusted after a single warning.
- No Creative Cloud sync. Presets pushed this way live on the device.

## Invocation

Bare `./lrpush`. **Zero flags.** `--udid`, `--bundle-id`, `--path-prefix`,
`--catalog` are all removed. Everything resolves by auto-detection or an
interactive picker.

## Flow

### 1. Device selection
`device.List()` returns connected USB devices (deduped by udid).

- 0 devices → error and exit.
- 1 device → auto-select.
- >1 → arrow-key picker (huh) listing name / model / udid.

### 2. Lightroom app detection (no bundle picker)
For the chosen device, detect the installed Lightroom apps by **attempting a
house_arrest `VendDocuments` vend** on each known bundle id, in this precedence
order:

1. `com.adobe.lrmobilephone` (iPhone)
2. `com.adobe.lrmobile` (iPad / universal)

A bundle id that vends successfully is installed and file-sharing-enabled — it
is a mirror target. This reuses the exact code path `device.Connect` already
uses (validated on-device), so detection cannot disagree with what a later
connect would do, and no separate `installationproxy` call is introduced. A
successful probe's open AFC session is kept and reused for that app (no second
vend).

- 0 installed → error and exit ("Lightroom not found on this device").
- ≥1 installed → **mirror every one of them.** Each installed app becomes an
  independent mirror session with its own local root and its own watcher, all
  running concurrently. Because each app's local tree is namespaced by bundle
  id (`./sync/{bundle-id}/`), there is no collision and therefore no bundle
  picker.

In the common case exactly one Lightroom app is installed, so this degenerates
to a single session.

### 3. Per-app setup
For each installed Lightroom app (reusing the AFC session opened during
detection in step 2):

1. Use the app's already-open house_arrest AFC session.
2. Locate the userStyles target: `locate.DocumentsRoot` → `locate.FindCatalogs`
   → `locate.SelectCatalog`.
   - 1 catalog → auto-select.
   - >1 → arrow-key picker.
3. Compute the local root: `./sync/{bundle-id}/userStyles/`.

Catalog pickers (if any) are resolved up front, before any watcher starts, so
the interactive phase is over before mirroring begins.

### 4. Pull-and-replace (device → local)
First, remove `./sync/` entirely (clearing any leftovers from a prior crashed
run). Then, for each app's local root:

1. Create `./sync/{bundle-id}/userStyles/`.
2. Recursively pull the entire device `userStyles` tree into it.
3. Log per file.

The device is never written during this phase. A pull failure aborts that
session with an error (re-run to retry — local is just a mirror).

### 5. Warn once
Before starting the watchers, print the existing "fully close Lightroom now,
reopen when you're done so it rebuilds its preset index" banner a single time.
No per-change backups.

Then, for each app, print the **absolute** path of its watched folder so the
user knows exactly where to edit, e.g.:

```
editing → /Users/.../sync/com.adobe.lrmobile/userStyles  (watching for changes; Ctrl-C to stop)
```

### 6. Watch (local → device)
For each app, a recursive fsnotify watcher over its local `userStyles` tree:

- Watches are added on the root and every subfolder, and dynamically
  added/removed as folders appear/disappear.
- Raw events are coalesced with a debounce (~400 ms) into a deduped set of
  changed **relative paths**.
- Each debounced batch is reconciled to the device via a pure function
  `Reconcile(fs, localDir, deviceUserStyles, changedRelPaths, out)`:
  - local path exists as file → `MkDir -p` parent on device + `PushFile`.
  - local path exists as dir → ensure the dir and push its contents (walk).
  - local path missing → `RemoveAll` on device (mirror the deletion).
  - rename → surfaces as delete(old) + create(new); handled by the above.
- Every device path is passed through the existing path-containment guard so a
  local path can never escape `userStyles` on the device.

fsnotify stays at the edge (collect paths + debounce only). The device-mutating
logic lives entirely in `Reconcile`, which is unit-testable without fsnotify.

When more than one app is mirrored concurrently, every log line is prefixed with
the app's bundle id (e.g. `[com.adobe.lrmobile] pushed A/foo.xmp`) so the
interleaved output of the concurrent watchers stays readable.

### 7. Shutdown
Run until Ctrl-C (SIGINT). On signal, stop all watchers, close all AFC sessions
cleanly, and print a closing reminder to reopen Lightroom so it rebuilds its
preset index. `./sync/` is **not** deleted on exit — it is left on disk and
wiped at the start of the next run (step 4), so no exit-time cleanup or
signal-handler teardown of the local tree is needed.

## Error handling

- No device / no Lightroom app → error and exit before any mirroring.
- Initial pull failure → that session aborts with an error.
- During watching, a push/delete error is **logged and the session keeps
  running**. A device disconnect surfaces as repeated op errors rather than a
  crash. (Whether to auto-exit on a hard disconnect is left to implementation
  judgement; logging-and-continuing is the baseline.)
- Concurrent sessions are independent: one app's failure does not stop another.

## Packages

**Keep**
- `internal/afcfs` (+ `MemFS`) — AFC filesystem boundary and in-memory fake.
- `internal/device` — `List`, `Connect`, `DescribeDevice`; **add** installed-app
  detection by probing `VendDocuments` on each Lightroom bundle id and returning
  the sessions that vend successfully (no `installationproxy`).
- `internal/locate` — DocumentsRoot / FindCatalogs / SelectCatalog.

**New**
- `internal/mirror` —
  - `PullReplace(fs, deviceUserStyles, localDir, out) error` — wipe local, pull
    the whole device tree.
  - `Reconcile(fs, localDir, deviceUserStyles, changedRelPaths, out) error` —
    the pure device-mutating core (push/delete for a set of changed paths).
  - `Watcher` — wraps fsnotify: recursive watches, debounce, calls `Reconcile`.

**Remove**
- Packages: `internal/inspect`, `internal/pushsync`, `internal/rmsync`.
- cmd files: `inspect.go`, `push.go`, `rm.go`, `devices.go`, `interactive.go`,
  and the rm-select `tui.go`. The device picker and catalog picker move into a
  small shared file under `cmd/lrpush`.

**Dependencies**
- Add `github.com/fsnotify/fsnotify`.
- Keep `cobra` (single command + `--help`), `huh` (device/catalog pickers),
  `golang.org/x/term`.

## Local folder layout

```
./sync/                         (wiped at the start of every run)
  {bundle-id}/
    userStyles/
      <preset groups and loose files, mirrored from the device>
```

`.gitignore` adds `/sync/` (defensive — the folder should not survive a normal
run, but a crash could leave it behind).

## Testing

- `PullReplace`: `MemFS` device side + a temp dir local side; asserts local is
  wiped and repopulated to exactly match the device tree.
- `Reconcile`: `MemFS` device + temp local; table-driven cases for create,
  modify, delete, new nested dir, rename (delete+create), and a
  path-containment-escape attempt (must be refused).
- Device app detection: unit-test the precedence/collection logic (which bundle
  ids become mirror targets) against a fake vend-probe that reports success or
  failure per bundle id.
- fsnotify itself is not unit-tested; the reconcile logic is exercised directly.

## Rationale for the removed subcommands

`inspect` (listing) is subsumed by the local folder itself — after startup you
just look at `./sync/{bundle-id}/userStyles/`. `push`/`rm` are
subsumed by editing that folder while the watcher runs. `devices` is subsumed by
the startup device picker. Git history preserves the removed code.
