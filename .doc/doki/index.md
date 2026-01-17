# Hello! こんにちは

This is a wiki-style documentation called `doki` saved as Markdown files alongside the project
Since they are stored in git they are versioned and all edits can be seen in the git history along with the timestamp
and the user. They can also be perfectly synced to the current or past state of the repo or its git branch

This is just a samply entry point. You can modify it and add content or add linked documents 
to create your own wiki style documentation

Press `Tab/Enter` to select and follow this [link](linked.md) to see how. 
You can refer to external documentation by linking an [external link](https://raw.githubusercontent.com/boolean-maybe/navidown/main/README.md)

 You can also create multiple entry points such as:
- Brainstorm
- Architecture
- Prompts

by configuring multiple plugins. Just author a file like `brainstorm.yaml`:

```text
        name: Brainstorm
        type: doki
        foreground: "##ffff99"
        background: "#996600"
        key: "F6"
        url: new-doc-root.md
```

and place it where the `tiki` executable is. Then add it as a plugin to the tiki `config.yaml` located in the same directory:

```text
        plugins:
            - file: brainstorm.yaml
```