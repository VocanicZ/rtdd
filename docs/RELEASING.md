# Releasing rtdd

This document is the human release sequence for `VocanicZ/rtdd`. It exists because the
three steps below have to happen **in this order**, and because the third one is easy to
skip — skipping it looks exactly like a bug in `install.sh`.

The sequence, in order:

1. flip the repository's visibility to public (#10);
2. push the `v*` tag, which triggers the release workflow and GoReleaser;
3. publish the GoReleaser draft, which is what makes `releases/latest` resolve.

**No agent performs any of these three steps** — no agent may flip visibility, push a
release tag, or publish a release. Each one is irreversible or close to it, and each is a
human decision. What an agent may run is
[`scripts/release-preflight.sh`](../scripts/release-preflight.sh), described in
[Before you start](#before-you-start) below: it runs every automated check and then stops
at a `DECISION REQUIRED` block instead of acting. That stop is the script working as
designed, not a defect.

## Before you start

Run the pre-flight from a clean checkout:

```bash
scripts/release-preflight.sh
```

It runs `rtdd-gen check`, `rtdd-gen verify`, `go test ./...`, the `bench/swebench` pytest
suite and `preflight.py`, the release-artifact linkage check, and a placeholder grep. It
then prints:

```
DECISION REQUIRED — repository visibility and release
```

with every field filled in from what it just ran — the branch taken at Task 14, the kill
criterion, the front-end checks, the test suite, the release binaries, placeholder hits,
the pre-registration tag, the M4 launch gate, and the current visibility — and exits
without touching anything. It never runs `gh repo edit`, `git tag`, `git push` or `gh release`; a test
(`TestReleasePreflightNeverMutatesRepositoryState`) enforces that. Read the block, then
work through the steps below yourself.

Two decisions are open and neither belongs to an agent:

- **#10 — repository visibility.** Flip `VocanicZ/rtdd` to public. This is step 1 below.
- **#8 — the pre-registration signature and the `prereg-m4` tag.** `bench/PREREGISTRATION.md`
  still reads `status: UNSIGNED`, and the pre-flight reports `Pre-registration tag:
  prereg-m4 -> missing` until a human signs the file and tags the commit it is reachable
  from. An agent may not sign a pre-registration or create that tag on a human's behalf —
  the whole point of a pre-registration is that a person committed to it before seeing the
  results.

## Step 1 — flip the repository's visibility to public (#10)

**Why first:** `install.sh` is fetched from `raw.githubusercontent.com`, and that URL
returns 404 while the repository is private. Every later step produces artifacts nobody
outside the repo can reach until this one is done. Doing it last would mean shipping a
release that no stranger can install.

**What flipping visibility publishes**, all at once and irreversibly in practice:

- the benchmark results (`bench/results/`);
- the raw per-instance records behind them;
- the pre-registration (`bench/PREREGISTRATION.md`), whatever state it is in;
- the prior-art claims in the README, which name other people's work — TDAD,
  pytest-testmon, Wallaby, Infinitest, Ekstazi and SonarQube.

Read the README's prior-art section once more before saying yes.

## Step 2 — push the `v*` tag

**Why second:** the tag is what triggers the release. `.github/workflows/release.yml` is
active and runs on `push: tags: ['v*']`; it reads `docs/outcomes/SELECTED` first and skips
the release entirely if the negative branch was taken, then runs GoReleaser for real
against `.goreleaser.yaml`.

```bash
git tag v0.1.0
git push origin v0.1.0
```

A tag that has been pushed and built against is not something to reuse: if the build is
wrong, cut the next patch version rather than moving the tag.

## Step 3 — publish the GoReleaser draft

**Why this step exists at all, and why a first release walks straight into it:**

`.goreleaser.yaml` sets

```yaml
release:
  draft: true
```

so the release GoReleaser creates in step 2 is a **draft**. A draft release is invisible to
the release API. Until a human opens the release on GitHub and clicks **Publish release**:

- `https://api.github.com/repos/VocanicZ/rtdd/releases/latest` returns **404**;
- `install.sh`, which resolves the version from exactly that URL, dies with
  `could not resolve the latest release version from
  https://api.github.com/repos/VocanicZ/rtdd/releases/latest`;
- the README's `curl | sh` line therefore cannot work, even though the workflow was green
  and the archives are all sitting there attached to the draft.

That combination — a green release run, real artifacts, and a `curl | sh` that 404s — is
the failure mode a first release walks into, and it reads like a broken installer rather
than an unfinished release.

**Do not "fix" it by removing `draft: true`.** The draft is a deliberate safety catch: it
gives a human the chance to look at the built artifacts and the generated changelog before
anything is public and permanent. The fix is to know about the step and take it.

Publish it either from the release page or with:

```bash
gh release edit v0.1.0 -R VocanicZ/rtdd --draft=false
```

Then confirm the trap is actually closed:

```bash
curl -fsSL https://api.github.com/repos/VocanicZ/rtdd/releases/latest | grep tag_name
```

## Summary

| # | Step | Human-only | What it unblocks |
|---|------|-----------|------------------|
| 1 | Flip visibility to public (#10) | yes | anonymous fetches of `install.sh` and the repo |
| 2 | Push the `v*` tag | yes | the `release` workflow and the GoReleaser build |
| 3 | Publish the GoReleaser draft | yes | `releases/latest` stops returning 404; `install.sh` works |

Steps 1 and 3 are not reversible in any meaningful sense — what is published stays
mirrored somewhere. Take them deliberately, in this order.
