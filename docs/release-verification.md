# Release Verification

`tq` releases publish:

- per-artifact SBOM files (SPDX JSON) alongside release archives
- GitHub artifact provenance attestations for release archives

## Verify provenance

Use GitHub CLI (v2.50+ recommended):

```bash
gh attestation verify tq_1.2.3_linux_amd64.tar.gz \
  --repo tq-lang/tq
```

Expected result:

- verification succeeds
- predicate type is build provenance
- source repository is `tq-lang/tq`

## Inspect SBOM

Download the archive SBOM file from the same GitHub Release:

- archive: `tq_1.2.3_linux_amd64.tar.gz`
- SBOM: `tq_1.2.3_linux_amd64.tar.gz.spdx.json`

Validate or inspect:

```bash
jq '.spdxVersion, .name, .creationInfo.created' tq_1.2.3_linux_amd64.tar.gz.spdx.json
```
