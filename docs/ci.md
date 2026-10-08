# Continuous integration

Reproduce the GitHub Actions checks locally from the repository root:

```sh
bin/test
bin/lint
```

## Dependabot policy

`.github/dependabot.yml` opens weekly update pull requests for Go modules
(`gomod`), npm packages (`npm`) and GitHub Actions (`github-actions`), rooted at
`/`. Minor and patch updates are grouped per ecosystem, and each ecosystem keeps
at most 5 open pull requests. Dependabot pull requests are labelled
`dependencies`.

Dependabot pull requests must pass the same `bin/test` and `bin/lint` workflow
as any other change. They are reviewed and merged by a human; auto-merge is not
enabled.
