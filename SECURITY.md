# Security Policy

`tonys-agent-telemetry` collects and persists telemetry from local AI-agent
sessions. We take supply-chain integrity and vulnerability reporting
seriously. This document explains how to report issues and how to verify
the binaries you download.

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | ✅ |
| < 0.1   | ❌ (pre-release) |

We support the latest minor release. Security fixes are not backported.

## Reporting a vulnerability

**Do not** open a public GitHub issue for security problems.

Instead, use **one** of:

1. **GitHub private vulnerability reporting** (preferred) —
   <https://github.com/vanillacake369/tonys-agent-telemetry/security/advisories/new>
2. **Email** the maintainer at
   [lonelynight1026@gmail.com](mailto:lonelynight1026@gmail.com).

Please include:

- A description of the issue and its impact (data exposure / RCE / DoS /
  supply chain / etc.).
- Reproduction steps or a proof-of-concept.
- Affected version(s) and OS.
- Whether you'd like public credit on disclosure.

### Response timeline

- **Within 7 days** — acknowledgement of the report.
- **Within 30 days** — initial severity assessment and rough fix ETA.
- **Coordinated disclosure** — we'll work with you on a public timeline; the
  default is 90 days from initial report or release of a fix, whichever is
  sooner.

## Verifying release artefacts

Every tagged release ships with **cosign keyless signatures** (Fulcio
certificate + OIDC) and **SLSA L3 provenance** (in-toto attestation).
Use both to confirm a binary came from this repository's release workflow
on the tagged commit.

### 1. Cosign signature

```sh
# Replace ARCH and OS to match the artefact you downloaded.
ARTIFACT=tonys-agent-telemetry_linux_amd64.tar.gz

gh release download v0.1.0 -p "${ARTIFACT}" -p "${ARTIFACT}.sig" -p "${ARTIFACT}.pem"

cosign verify-blob \
  --certificate "${ARTIFACT}.pem" \
  --signature   "${ARTIFACT}.sig" \
  --certificate-identity-regexp \
    '^https://github\.com/vanillacake369/tonys-agent-telemetry/\.github/workflows/release\.yml@refs/tags/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  "${ARTIFACT}"
```

Expected output: `Verified OK`.

The regex pins the signature to **this repo's release.yml workflow on a
tag ref** — a compromised CI workflow on a different ref or in a forked
repo would not match.

### 2. SLSA L3 provenance

```sh
gh release download v0.1.0 -p "${ARTIFACT}" -p tonys-agent-telemetry.intoto.jsonl

slsa-verifier verify-artifact \
  --provenance-path tonys-agent-telemetry.intoto.jsonl \
  --source-uri github.com/vanillacake369/tonys-agent-telemetry \
  "${ARTIFACT}"
```

Expected output: `PASSED: SLSA verification passed` with the source commit
and builder identity printed.

### 3. Checksums

`checksums.txt` itself is signed (`checksums.txt.sig` + `checksums.txt.pem`).
Verify it first, then verify any individual archive against it.

## Threat model

In scope:

- Supply-chain compromise of release artefacts (cosign + SLSA address this).
- Code execution paths in the ingest pipeline (claudecode JSONL parsing,
  OTLP receiver, vLLM scrape, ollama poll).
- Data exposure through the OTLP receiver (default bind is `127.0.0.1` —
  set `TONYS_OTLP_BIND=0.0.0.0:4318` only when you intend LAN exposure).
- Local denial-of-service via malformed input.

Out of scope (project boundary):

- Vulnerabilities in upstream tools you point us at (Claude Code, vLLM,
  Ollama, LangGraph, etc.). Report those to their projects.
- Issues only reproducible with `TONYS_RUN_LIVE=1` or other opt-in test
  flags — those are by design.
- Issues only reproducible when running unsigned binaries (e.g., a local
  source build) — reproduce against a signed release.

## Hardening defaults

The release shipped with these conservative defaults to reduce blast
radius even before any active enforcement:

- OTLP receiver binds `127.0.0.1:4318` (not `0.0.0.0`).
- Span buffer capped at `TONYS_MAX_SPANS=50000` to bound memory.
- Catalog cache pinned to a specific upstream Git commit SHA — auto-track
  of upstream branches is disabled.
- Signal store path lives under `$XDG_CACHE_HOME` (not `/tmp`) and uses
  `flock` for concurrent writers.
- CGO disabled in all release builds (pure-Go static binaries).

## Acknowledgements

We will credit researchers in the affected release's
[CHANGELOG](./CHANGELOG.md) under "Security" unless they request
anonymity.
