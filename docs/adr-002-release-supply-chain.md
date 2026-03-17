# ADR-002: Release Supply-Chain Hardening

**Status:** Accepted  
**Date:** 2026-03-17  
**Context:** OSS launch hardening

## Decision

Use a phased supply-chain hardening approach:

1. **Immediate (implemented now):**
   - Pin all GitHub Actions in CI/release workflows to immutable commit SHAs.
2. **Next phase (tracked work):**
   - Add SBOM generation for release artifacts.
   - Add provenance attestation for release artifacts.

## Context

Unpinned action tags can move over time and introduce unexpected changes into
CI/release pipelines. For public open-source releases, reproducibility and
traceability are important for maintainer confidence and downstream trust.

At the same time, introducing SBOM and provenance requires tool and process
choices (format, attachment location, verification workflow). We apply the
minimum high-value control now (SHA pinning), then complete SBOM/provenance as
follow-up work.

## Consequences

### Positive

- Reduced supply-chain risk from mutable action tags.
- More deterministic CI/release behavior.
- Clear path toward stronger artifact transparency.

### Trade-offs

- Action updates require manual SHA refreshes.
- SBOM/provenance are not yet published with releases.

## Follow-Up

- Add SBOM generation and publication to release pipeline.
- Add provenance attestation generation and publication to release pipeline.
