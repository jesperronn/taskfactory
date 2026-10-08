# TF-036: Keep dependencies updated with Dependabot

## Goal

Get automatic update pull requests for Go modules, npm packages and GitHub
Actions so CI warnings such as deprecated action runtimes are caught early.

## Dependencies

- TF-018

## Scope

Add `.github/dependabot.yml` (version 2) with three update entries rooted at
`/`: `gomod`, `npm` and `github-actions`. Use a weekly schedule, group minor and
patch updates per ecosystem, and cap open pull requests at 5 each. Label the
pull requests `dependencies`. Document the policy in `docs/ci.md`: Dependabot
pull requests must pass the same `bin/test` and `bin/lint` workflow, and they
are reviewed and merged by a human.

## Constraints

Do not add auto-merge, extra workflows, tokens or secrets. Do not change the
existing CI workflow beyond what the docs reference. Do not enable ecosystems
the repository does not use.

## Success criteria

### C1: Dependabot config covers gomod, npm and github-actions

Check: grep -c "package-ecosystem" .github/dependabot.yml

### C2: Config is valid YAML with version 2

Check: npx prettier --check .github/dependabot.yml

### C3: CI documentation describes the Dependabot policy

Check: grep -n -i dependabot docs/ci.md

### C4: Repository checks and task validation pass

Check: bin/test

## Verification

Run the four checks above, then `bin/lint` and
`go run ./cmd/taskfactory validate`. Report each exit code. The first real
Dependabot run on GitHub cannot be verified locally; report it as unverified.
