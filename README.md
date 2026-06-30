# lrpush

Push local Lightroom presets onto an iPhone's Adobe Lightroom mobile app
(`com.adobe.lrmobile`) over USB, using Apple house_arrest + AFC via
[go-ios](https://github.com/danielpaulus/go-ios). No jailbreak, no tunnel.

## How it works

Lightroom mobile stores user presets (styles) inside its app container at
`Documents/{catalog}/settings-acr/userStyles/`. lrpush connects to that
container over USB and copies presets in.

## Requirements

- macOS with the iPhone connected via USB and **trusted** (tap "Trust This
  Computer" on the phone; unlock once so the pairing is valid).
- Go 1.26+ to build.
- Dependencies (Go modules): `github.com/danielpaulus/go-ios`,
  `github.com/spf13/cobra`.

## Build

    make build        # produces ./lrpush
    # or: go build -o lrpush ./cmd/lrpush

Other targets: `make test`, `make vet`, `make fmt`, `make clean`.

## Safety first

- Commands are **dry-run by default**. Add `--commit` to actually write.
- Before any `--commit`, fully **close Lightroom** on the iPhone (swipe it
  away). Re-open it afterwards. Otherwise the app may overwrite the changes.
- `push` backs up the whole `userStyles` to `./_userStyles_backup/<timestamp>/`
  before writing; `rm` backs up each target before deleting.
- Presets pushed this way may appear only on the device and may not sync to
  Creative Cloud.

## Workflow

### 1. Inspect (run this first)

    ./lrpush inspect

Dumps the container tree, lists catalogs containing `settings-acr`, selects the
userStyles target, and pulls one existing preset into `./_inspect_sample/` so
you can confirm the real file extension/format.

### 2. Push (dry-run, then commit)

A folder keeps its own name as a subfolder under userStyles
(`./my-presets/` -> `userStyles/my-presets/...`, structure preserved). If that
target subfolder already exists it is replaced wholesale (old folder removed,
backed up first). Other existing userStyles content is left untouched.

    # preview
    ./lrpush push --source ./my-presets
    # apply
    ./lrpush push --source ./my-presets --commit

Single file:

    ./lrpush push --source ./foo.xmp --commit

### 3. Remove

Paths are relative to userStyles; multiple allowed; files or folders.

    ./lrpush rm my-presets foo.xmp            # dry-run
    ./lrpush rm my-presets foo.xmp --commit   # apply (backs up first)

## Troubleshooting

**Multiple devices connected:** `lrpush` uses the first USB device by default,
which is non-deterministic when several are attached. Pass `--udid <udid>` to
target a specific one. You can find udids by running `inspect` per device, or
via Apple's tooling.

**`InstallationLookupFailed` / lockdown errors:** make sure the device is
unlocked and trusted (accept the "Trust This Computer" prompt). `lrpush` opens
the app's documents container via house_arrest `VendDocuments` (falling back to
`VendContainer`), so the target app must be installed and expose file sharing.

**Pushed presets don't appear in Lightroom:** fully close Lightroom (swipe it
away) before pushing and reopen it after, so it re-reads its preset index.
Presets pushed this way may not sync to Creative Cloud.

## Flags

- `--udid` — target device (default: first USB device)
- `--bundle-id` — default `com.adobe.lrmobile`
- `--path-prefix` — override AFC root prefix if auto-detection is wrong
- `--catalog` — pick catalog by name (non-interactive; otherwise a menu appears
  when multiple catalogs exist)
- `--backup-dir`, `--commit` — see Safety
