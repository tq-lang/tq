# ADR-002: Release Supply-Chain Hardening

**Status:** Accepted  
**Date:** 2026-03-17  
**Context:** OSS launch hardening

## Decision

Use a phased supply-chain hardening approach:

1. **Immediate (implemented now):**
   - Pin all GitHub Actions in CI/release workflows to immutable commit SHAs.
2. **Next phase (implemented):**
   - Generate and publish SBOMs for release archives.
   - Generate provenance attestations for release archives.

## Context

Unpinned action tags can move over time and introduce unexpected changes into
CI/release pipelines. For public open-source releases, reproducibility and
traceability are important for maintainer confidence and downstream trust.

SBOM and provenance required additional tool/process choices (format, upload
location, verification workflow). We implemented SHA pinning first, then
completed SBOM/provenance in the next phase.

## Consequences

### Positive

- Reduced supply-chain risk from mutable action tags.
- More deterministic CI/release behavior.
- Clear path toward stronger artifact transparency.

### Trade-offs

- Action updates require manual SHA refreshes.
- Release archives now include published SPDX SBOMs.
- Release archives now have GitHub provenance attestations.

## Follow-Up

- Keep SBOM/provenance verification docs current with release workflow changes.
