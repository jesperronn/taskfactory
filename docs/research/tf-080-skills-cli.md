# TF-080 research: skills CLI discovery

## Ran (verified)

Scratch npm cache, no global install, repo not modified by the run:

```sh
npx -y skills --help                                  # exit 0
npx -y skills add <worktree> --list                   # exit 0
npx -y skills add <worktree> --list --full-depth      # exit 0
```

Both `--list` runs found one skill, `taskfactory`, with the description from
`skills/taskfactory/SKILL.md`. So the CLI discovers that file in this layout.

## Read only (unverified)

The mintlify page (redirected to mintlify.wiki) describes the frontmatter
(`name`, `description`) and `--list`. It did not describe the discovery rules
(which directories are searched, root `SKILL.md`, `--full-depth` semantics).
Those rules are unverified here. The upstream repository
github.com/vercel-labs/skills was not read.

## Not done

`npx skills add jesperronn/taskfactory` against the public GitHub repository
was not run. A human confirms it after the repository is public.
