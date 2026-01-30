###service-notification

![project tests](https://github.com/antinvestor/service-notification/actions/workflows/run_tests.yml/badge.svg) ![image release](https://github.com/antinvestor/service-notification/actions/workflows/release.yml/badge.svg)


A repository for the  notification service being developed
for ant investor

## Development Setup

### Git Hooks

This repository includes a pre-commit hook that automatically runs `make format` before each commit to ensure consistent code formatting.

**Enable the hook:**
```bash
git config core.hooksPath .githooks
```

**What it does:**
- Detects staged `.go` files
- Runs `make format` to apply gofmt/goimports
- If formatting changes any files, the commit is blocked
- You must review and stage the formatted files before committing again

**To disable temporarily:**
```bash
git commit --no-verify
```

