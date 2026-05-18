# Cross-references

You just followed a link. Press `←` (or `Backspace`) to return to the
[entry point](index.md).

## Linking between documents

Three ways to reference other content:

- **Relative path** — `[label](other.md)` for sibling files
- **Wikilink** — `[[ABC123]]` resolves by id through the tiki index, so it
  survives file moves
- **External** — regular `[label](https://example.com)` URLs open in your
  default browser

Unknown wikilink targets render as `[[ZZZZZZ]] *(not found)*` so broken
references stay visible.

## Where to go next

- Edit this file directly — it lives at `.doc/linked.md`
- Open `.doc/workflow.yaml` to add fields, statuses, or views
- Run `tiki exec 'select'` to list every document the store sees
