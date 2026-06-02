# Contributing

## Architecture as Source of Truth

[`ARCHITECTURE.md`](./ARCHITECTURE.md) is the **single source of truth** for the Event-Driven Sleep Audio Pipeline design.

Before opening an issue or submitting a pull request that changes infrastructure or data flow, read `ARCHITECTURE.md` to understand the intended design. Any change that adds, removes, or modifies AWS resources, data flow steps, security controls, or observability components **must** include a corresponding update to `ARCHITECTURE.md` and its Mermaid diagram in the same commit or pull request. Reviewers will check for consistency between code and documentation.

If you are unsure whether your change affects the architecture, err on the side of updating the document.

---

## Strict TDD Rules

Always write failing test(s) first, then write the minimal code to make them pass. No production code without a corresponding failing test.

## Conventional Commits

All commit messages must follow the Conventional Commits format:

- `feat:` - new feature
- `fix:` - bug fix
- `chore:` - maintenance tasks
- `docs:` - documentation changes
- `refactor:` - code restructuring without behavior change

## Keep Architecture in Sync

Keep `ARCHITECTURE.md` and its Mermaid diagram in sync with all changes. Any change to infrastructure or data flow must be reflected in the architecture documentation.

## Before Pushing

Run the following commands and ensure they pass before pushing:

```bash
go test ./...
cdk synth
```
