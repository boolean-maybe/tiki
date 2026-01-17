# Contributing Guidelines

Contributions welcome!

**Before spending lots of time on something, ask for feedback on your idea first!** For general ideas or "what if" scenarios, 
please start a thread in **GitHub Discussions**. For specific, actionable bug reports or concrete feature proposals, use **GitHub Issues**.

Please search issues and pull requests before adding something new to avoid duplicating efforts and conversations.

In addition to improving the project by refactoring code and implementing relevant features, this project welcomes the following types of contributions:

* **Ideas**: Start a conversation in **GitHub Discussions** to brainstorm or participate in an existing **GitHub Issue** thread to help refine a technical goal.
* **Writing**: contribute your expertise in an area by helping expand the included content.
* **Copy editing**: fix typos, clarify language, and generally improve the quality of the content.
* **Formatting**: help keep content easy to read with consistent formatting.

## 💬 Communication Channels

To keep the project organized, please use these channels:

* **GitHub Discussions**: Use this for general questions, brainstorming new features, or sharing how you are using the project.
* **GitHub Issues**: Use this strictly for bug reports or finalized feature requests that are ready for implementation.

## Installing

Fork and clone the repo, then `go mod download` to install all dependencies.

## Testing

Tests are run with `go test ./...`. Unless you're creating a failing test to increase test coverage or show a problem, please make sure all tests are passing before submitting a pull request.

---

# Collaborating Guidelines

**This is an Open Source Project.**

## Rules

There are a few basic ground rules for collaborators:

1. **No `--force` pushes** or modifying the Git history in any way.
2. **Non-master branches** ought to be used for ongoing work.
3. **External API changes and significant modifications** ought to be subject to an **internal pull request** to solicit feedback from other collaborators.
4. Internal pull requests to solicit feedback are *encouraged* for any other non-trivial contribution but left to the discretion of the contributor.
5. Contributors should attempt to adhere to the prevailing code style (run `go fmt` before committing).

## Releases

Declaring formal releases remains the prerogative of the project maintainer.