## What this PR does

<!-- One-paragraph summary of the change and why it's needed. Link any issue
     this fixes with "Fixes #123". -->

## How it was tested

<!-- e.g. "go build ./... · ran webtermin locally and walked through services
     start/stop/restart" — be concrete. -->

## Checklist

- [ ] `go build ./...` passes locally
- [ ] `cd web && npm run build` passes (only if you touched frontend)
- [ ] No raw shell strings constructed from user input — all system calls use argv slices through validated allowlists
- [ ] New mutating endpoints write to the audit log
- [ ] User-visible behaviour added → README updated; release-noteworthy → CHANGELOG `[Unreleased]` entry added
