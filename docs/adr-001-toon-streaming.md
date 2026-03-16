# ADR-001: Native TOON Streaming Tokenizer

**Status:** Accepted
**Date:** 2026-03-17
**Context:** Issue #4 — Streaming mode for large files

## Decision

Implement a native TOON `TOONTokenReader` that emits `[path, value]` pairs with O(depth) memory, replacing the previous `tostream | (filter)` fallback that loaded the entire TOON document into memory before streaming.

## Context

tq's `--stream` flag already worked for JSON via `TokenReader` (a port of gojq's `stream.go`), achieving O(depth) memory for arbitrarily large JSON. However, TOON streaming was fake: it loaded the full document via `toon.Decode`, then piped it through a `tostream | (filter)` jq expression. For GB+ TOON files, `--stream` provided no memory benefit.

### Requirements from Issue #4

1. GB+ files (JSON **and** TOON) processed with bounded memory
2. Streaming opt-in (`--stream`) **and** auto-detected (file > threshold)
3. Clear warnings when a filter is incompatible with streaming
4. Robust testing: memory tests, benchmarks, unit tests

## Alternatives Considered

### 1. Modify toon-go to expose a streaming API

**Rejected.** toon-go's decoder is a batch parser that builds the full document tree. Adding streaming would require a major rewrite of an upstream library we don't control, and the streaming output format (`[path, value]` pairs) is specific to tq/jq, not a general TOON concern.

### 2. Line-by-line io.Reader adapter wrapping toon.Decode on chunks

**Rejected.** TOON documents can't be reliably chunked — indentation determines structure, so a chunk boundary mid-object would produce parse errors. The only safe chunk boundary is a blank line (document separator), but a single TOON document could be gigabytes.

### 3. Native line-oriented tokenizer in tq (chosen)

Build a streaming tokenizer that processes TOON line-by-line using a `bufio.Scanner`, tracking path state with a stack. This matches JSON `TokenReader`'s output format exactly, so both formats share the same filter pipeline.

**Advantages:**
- O(depth) memory for both JSON and TOON
- No dependency on toon-go internals
- Shared filter code path (no more `toonCode` / `tostream` wrapper)
- Preserves document key order (the old `tostream` approach sorted keys)

**Trade-offs:**
- Duplicates some parsing logic from toon-go (`decodePrimitiveToken`, `splitKeyValue`, etc.)
- Must stay in sync with any TOON format changes

## Architecture

### TOONTokenReader

```
internal/input/toon_tokenizer.go  — state machine (~320 lines)
internal/input/toon_parse.go      — reimplemented parsing helpers (~280 lines)
```

The tokenizer is a line-oriented state machine:
- Uses `bufio.Scanner` for line-by-line reading
- Maintains a `path []any` (strings for keys, ints for indices) and a `stack []containerInfo`
- Each `Next()` call drains a pending queue or processes the next non-blank line
- Supports all TOON constructs: key-value, nested objects, list arrays (`- item`), tabular arrays (`key[N]{fields}:`), inline arrays (`key[N]: a,b,c`)

Output format matches JSON `TokenReader` exactly:
- Leaf values: `[]any{path, value}`
- Container close: `[]any{path}` (truncate marker)
- Top-level object close: final `[]any{lastKeyPath}` at EOF

### streamCfg Simplification

The old `streamCfg` carried two compiled filters:
- `code` for JSON (applied to TokenReader output)
- `toonCode` for TOON (`tostream | (filter)`, applied to fully-decoded document)

The new `streamCfg` carries only `code` — both formats now produce identical `[path, value]` pairs, so the same filter works for both.

### Auto-detection

Files exceeding a configurable threshold are automatically streamed:
- Default: 256MB
- Override: `--stream-threshold 1GB` flag or `TQ_STREAM_THRESHOLD` env var
- Disable: `--no-stream` flag
- stdin: auto-detection not possible (can't stat), `--stream` must be explicit

### Filter Warnings

When streaming is active, a word-boundary scan of the filter expression warns about builtins that expect full documents (`sort`, `group_by`, `unique`, `reverse`, `transpose`, `flatten`, `combinations`, `walk`, `min_by`, `max_by`).

## Consequences

### Positive

- **True O(depth) streaming for TOON** — the primary goal
- **Simpler code path** — one filter, one code path for both formats
- **Document-order preservation** — TOON keys now stream in document order (the old tostream sorted them)
- **Auto-detection** — users don't need to remember `--stream` for large files
- **Proactive warnings** — incompatible filters are flagged before producing confusing output

### Negative

- **~600 lines of new parsing code** that partially duplicates toon-go internals
- **TOON format coupling** — if TOON adds new syntax, both toon-go and this tokenizer need updates
- **Behavioral change** — TOON streaming output order changed from alphabetical to document order (this is actually more correct and matches JSON behavior)

### Risks

- Edge cases in TOON parsing (deeply nested arrays-in-arrays, exotic delimiters) may diverge from toon-go's behavior. Mitigated by comprehensive unit tests covering all TOON constructs.
- Auto-detection could surprise users who don't expect streaming behavior. Mitigated by the info message on stderr and the `--no-stream` escape hatch.
