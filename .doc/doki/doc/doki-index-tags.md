# doki index tags

doki file fetcher supports three HTML comment tags that are processed in memory at load time. The file on disk is never modified.

Tags are only active in plugins with `fetcher: file`.

## INDEX_NAV

```
<!-- INDEX_NAV -->
```

Replaced with a flat markdown list of every `index.md` found recursively under the directory of the current file. Each entry links to the sub-index using a path relative to the current file. The H1 heading of each sub-index is used as the link label; the directory name is used as fallback when no H1 exists.

Depth is shown visually through leading spaces in the link label — no nested markdown list structure is created.

Example output for a docs root with `infrastructure/` and `infrastructure/network/` subdirectories:

```markdown
- [Infrastructure](infrastructure/index.md)
- [  Network](infrastructure/network/index.md)
```

Subdirectories without an `index.md` are silently skipped.

## INCLUDE

```
<!-- INCLUDE:path/to/file.md -->
```

Replaced with the full content of the referenced file. The path is relative to the current file's directory. Relative links inside the included content are rewritten to remain valid from the including file's location. Tags inside the included file are **not** processed.

## INCLUDE_RECURSIVE

```
<!-- INCLUDE_RECURSIVE:path/to/file.md -->
```

Same as `INCLUDE` but also processes `INDEX_NAV`, `INCLUDE`, and `INCLUDE_RECURSIVE` tags found inside the included file. Cycle detection prevents infinite loops — if a file has already been visited in the current include chain it is silently skipped.
