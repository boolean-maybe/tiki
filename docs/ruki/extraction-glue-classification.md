# Ruki extraction — host-glue classification (Phase D)

This document records the Phase D analysis of the host glue under `internal/ruki/runtime/`. The goal of the ruki
extraction milestone is a **language-only** standalone library: parser, semantic validator, executor, and the moved
leaf utilities (recurrence, duration, idfmt, collections). Everything that wires ruki to tiki's runtime services —
the store, the mutation gate, persistence, and the workflow field catalog — stays in tiki.

Each non-test `.go` file under `internal/ruki/runtime/` is classified as:

- **host-bound** — imports any of `store`, `service`, `workflow`, `config`, `store/tikistore`. It depends on tiki's
  runtime services and stays in tiki permanently.
- **portable** — imports only `ruki`, `tiki` (for `Doc` wrap/unwrap), and stdlib. It could theoretically move into
  the ruki library later, but for this milestone the library is language-only, so it also stays in tiki.

## Classification table

| File        | tiki packages imported             | classification |
| ----------- | ---------------------------------- | -------------- |
| `format.go` | `ruki`, `workflow`                 | host-bound     |
| `runner.go` | `ruki`, `service`, `store`, `tiki` | host-bound     |
| `schema.go` | `ruki`, `workflow`                 | host-bound     |

Per-file rationale:

- `format.go` — resolves the field catalog via `workflow.Fields()`/`workflow.Field` and renders cells by
  `workflow.FieldDef`/`workflow.ValueType`.
- `runner.go` — CLI entry points wired to `service.TikiMutationGate` and `store.ReadStore`; performs
  persistence (create/update/delete), pipe, and clipboard.
- `schema.go` — adapts a snapshot of `workflow.Fields()` (system + workflow) into the `ruki.Schema` interface
  used by the parser/executor.

## Conclusion

The entire `internal/ruki/runtime` layer stays in tiki for this milestone; the library is language-only.
