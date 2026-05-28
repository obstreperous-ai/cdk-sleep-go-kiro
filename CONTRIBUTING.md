# Contributing

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

Keep ARCHITECTURE.md and its Mermaid diagram in sync with all changes. Any change to infrastructure or data flow must be reflected in the architecture documentation.

## Before Pushing

Run the following commands and ensure they pass before pushing:

```bash
go test ./...
cdk synth
```
