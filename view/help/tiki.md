# tiki format

First of all, you just navigated to a linked file. To go back press `Left` arrow or `Alt-Left`
To go forward press `Right` arrow or `Alt-Right`

Tiki stores tickets (aka tikis) and documents (aka dokis) in the git repo along with code
They are stored under `.doc` directory and are supposed to be checked-in/versioned along with all other files

The `.doc/` directory contains two main subdirectories:
- **doki/**: Documentation files (wiki-style markdown pages)
- **tiki/**: Task files (kanban style tasks with YAML frontmatter)

## Directory Structure

```
.doc/
├── doki/
│   ├── index.md
│   ├── page2.md
│   ├── page3.md
│   └── sub/
│       └── page4.md
└── tiki/
    ├── tiki-k3x9m2.md
    ├── tiki-7wq4na.md
    ├── tiki-p8j1fz.md
    └── ...
```


## Tiki files

Tiki files are saved in `.doc/tiki` directory and can be managed via:

- `tiki` cli 
- AI tools such as `claude`, `codex` or `opencode`
- manually

A tiki is made of its frontmatter that includes all fields related to a tiki status and types and its description
in Markdown format

```text
        ---
        title: Sample title
        type: story
        status: backlog
        assignee: booleanmaybe
        priority: 3
        points: 10
        tags:
            - UX
            - test
        ---
        
        This is the description of a tiki in Markdown:
        
        # Tests
        Make sure all tests pass
        
        ## Integration tests
        Integration test cases
```

### Derived fields

Fields such as:
- `created by`
- `created at`
- `updated at`

are not stored and are calculated from git - the time and git user who created a tiki or the time it was last modified


## Doki files

Documents are any file in a Markdown format saved under `.doc/doki` directory. They can be organized in subdirectory
tree and include links between them or to external Markdown files