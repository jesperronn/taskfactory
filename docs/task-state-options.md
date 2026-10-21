# Task state options

This document explains where TaskFactory task state can live, which option the
owner chose, and what a cutover risks. TF-102 to TF-104 build on the decision in
the section "Owner decisions". It supersedes the exploration started in TF-099.

## Problem

Task state is the set of task files under `tasks/` (inbox, ready, active,
failed, archive) plus operational files under `.taskfactory/`. Today all of it
lives inside the project, and several commands write project history:

- `promote` moves a file from `tasks/inbox` to `tasks/ready` and commits only
  those paths (`PromoteWith` and `commitPaths` in `internal/promote/promote.go`).
- `fail` moves an active task to `tasks/failed` and commits it with the message
  `docs: mark <ID> failed` (`FailWith` and `commitPaths` in
  `internal/fail/fail.go`).
- `requeue` moves a failed task to inbox or ready and commits it with the
  message `docs: requeue <ID> to <target>` (`RequeueWith` and `commitPaths` in
  `internal/requeue/requeue.go`).
- `integrate` fast-forwards main, then moves the task to `tasks/archive` and
  commits it with `chore(tasks): archive integrated <ID>` (`RunWith` in
  `internal/integrate/integrate.go`).
- `claim` and `integrate` also need `.taskfactory/config.toml` tracked in the
  project. `cleanConfig` in `internal/integrate/integrate.go` refuses when it is
  untracked, staged or different from HEAD, and claim creates the branch from
  `refs/heads/main` (`ClaimWith` in `internal/claim/claim.go`).

Some projects want task bookkeeping outside project history: a shared product
repository should not carry a stream of `docs: mark ... failed` commits, and
some projects cannot add a `tasks/` tree at all.

Claim does not commit. `ClaimWith` renames the ready file to `tasks/active` in
the project root and creates a branch and worktree, with no git commit. The
claimed active file is uncommitted operational state, so the committed
transition for a claimed task is the tracked ready deletion plus the new failed
or archive addition (`lifecyclePaths` in `internal/fail/fail.go`, and the
`ls-files --error-unmatch` branch in `RunWith`). `verify` appends evidence to
`.taskfactory/evidence/<ID>.jsonl` (`internal/verify/verify.go`) and does not
stage or commit it. Integration evidence is likewise untracked.

The task folders do not have to be named `tasks/`: the optional top-level
`tasks_dir` key in `.taskfactory/config.toml` moves them to another directory
under the state root, for example `.taskfactory/tasks` (see `docs/config-v1.md`).
Everything below applies to that directory in the same way.

## Options

Each option is described by its mechanics, its effect on every lifecycle
command, and its cost to the audit trail.

### Option A: an external plain directory

Mechanics: `stateRoot` points at a plain folder outside the project, for
example `~/taskstate/<project>/`, holding `inbox`, `ready`, `active`, `failed`
and `archive`, and the evidence directories. It is not a git repository.

Effect on commands:

- promote: a plain rename from inbox to ready. There is nothing to commit, so
  the `commitPaths` step is skipped.
- claim: renames ready to active in `stateRoot`. The branch and worktree are
  still created in the project from `refs/heads/main`. The worker's checkout no
  longer contains the contract, so the prompt must carry it or point at it.
- verify: unchanged in behaviour. It reads the active file and appends evidence,
  now under `stateRoot`.
- fail: a plain rename from active to failed.
- requeue: a plain rename from failed to inbox or ready. Its worktree and branch
  cleanup still operates on the project.
- integrate: fast-forwards project main as today, then renames active to
  archive in `stateRoot`. No archive commit lands in the project.

Audit trail cost: the highest. There is no history of moves, no authorship and
no diff of contract edits. Only the evidence files and the project's feature
commits remain. A deleted or edited file cannot be recovered.

### Option B: a separate git repository

Mechanics: `stateRoot` is its own git repository, kept beside or outside the
project. Task files and evidence directories live there. The project repository
holds code only.

Effect on commands:

- promote, fail, requeue: moved and committed in the state repository using the
  owner's normal git config. The commit messages are the existing ones.
- claim: renames ready to active in the state repository, uncommitted as today.
  Branch and worktree creation stays in the project.
- verify: appends evidence under `stateRoot`, untracked as today.
- integrate: fast-forwards project main, then commits the archive move in the
  state repository.

Audit trail cost: low. Every move is a commit with an author and a diff. The
price is two repositories to keep in step. The two histories are not linked by
one commit, so a crash between the project fast-forward and the state commit
leaves them out of step (see Cutover risks).

### Option C: a git-excluded `tasks/` inside the project

Mechanics: `tasks/` stays at its current path but is listed in
`.git/info/exclude` or `.gitignore`, so the project repository never tracks it.

Effect on commands:

- promote, fail, requeue: the `git add --all -- <paths>` plus `commit --only`
  steps stage nothing for ignored paths and fail or commit nothing useful. These
  commands need a code change to rename without committing.
- claim: works, since the active move is uncommitted already. But the new
  worktree checks out the branch from main, which has no contract.
- verify: unchanged.
- integrate: the archive commit has nothing tracked to stage, so it must skip
  that step.

Audit trail cost: as high as Option A, because the files are plain and
untracked, with the hazard that a `git clean -x` or a fresh clone loses them.
Its advantage is that the paths do not change, so globbing and lint need no
update.

### Option D: an orphan branch checked out as a worktree

Mechanics: an orphan branch, for example `taskfactory/state`, holds only task
files and is checked out as a worktree at a fixed path such as
`.taskfactory/tasks`. It lives in the same repository, so it is cloned and
pushed with the project but shares no history with main.

Effect on commands:

- promote, fail, requeue: commit on the orphan branch, inside the worktree, with
  the owner's git config. The commit code must run git in the worktree rather
  than the project root.
- claim: renames ready to active in the worktree, uncommitted as today.
- verify: appends evidence as today.
- integrate: fast-forwards main from the project, then commits the archive on
  the orphan branch.

Audit trail cost: low, the same history as Option B in one repository. The cost
is worktree management: a missing or detached state worktree, branch checkout
conflicts, and `integrate`'s clean-tree checks (`cleanTree`, `cleanMainRuntime`
in `internal/integrate/integrate.go`) needing to ignore the state worktree.

## Comparison

Options A and C trade history for simplicity. They need no second repository
and no commit code, but a lost directory is a lost audit trail. Option C keeps
today's paths, which avoids glob and lint changes, but it needs the most
command changes to stop committing ignored paths and its state is easy to
destroy by accident. Option A is the same behaviour at a path that is clearly
not part of the project.

Options B and D keep a full history. Option B isolates the state completely and
is easy to explain: a project with a state repository is a project with two
repositories. Option D keeps everything in one repository and travels with
clone and push, but it is the most fragile operationally because it depends on
worktree health and branch rules.

A design that supports a plain `stateRoot` which may or may not be a git
repository covers A and B with one setting, and fits the owner's requirement to
keep history when it is available. C and D need extra layout rules and are not
chosen.

## Owner decisions

These are final.

1. Task state may live outside `<project>/tasks/` under a separate `stateRoot`.
   The default layout, state inside the project at `tasks/` and `.taskfactory/`,
   stays bit-for-bit as it is for every other project.
2. History is kept. If the state directory is a git repository, moves are
   committed there with the owner's normal git config, and signing is never
   touched. If it is a plain folder, moves are plain renames.
3. This repository dogfoods the feature by moving its own `tasks/` to
   `.taskfactory/tasks` as the last step, in one atomic commit while no task is
   active. Product behaviour for other projects stays separate from this
   repository's own data migration.

## Cutover risks

- Dot-directory blind spots: `.taskfactory/tasks` is a dot directory. Globs
  such as `tasks/*` and lint and tooling that skip dot paths will miss it, and
  `.gitignore` lines for `.taskfactory/` runtime paths must not hide the moved
  tasks. Check every glob and the `bin/lint` and `bin/test` scopes.
- A quiet moment: the cutover must happen while no task is active, with no
  claimed worktree reading the old paths and no integration in flight.
- Hardcoded main branch: claim, integrate and `check-main` use
  `refs/heads/main` (`internal/claim/claim.go`, `internal/integrate/`). TF-098
  handles that, and the cutover should not depend on a branch other than main.
- External projects need their own worker rules: a project whose state is
  outside its tree has no `tasks/` in the worker's checkout, so
  `AGENTS.md` and `docs/worker-instructions.md` in that project must say where
  the contract lives and how to validate it.
- The crash window: integrate fast-forwards project main first and commits the
  archive afterwards (`RunWith` in `internal/integrate/integrate.go`). If the
  process dies between the two, main has advanced and the task is still active
  in `stateRoot`. The integration evidence in `stateRoot` is the source of
  truth for whether the merge happened, and a rerun or manual archive must
  consult it before acting.

## Follow-up tasks

In this order:

1. TF-102: the `stateRoot` setting and path resolution.
2. TF-103: the commit policy, committing moves when `stateRoot` is a git
   repository and renaming when it is a plain folder.
3. TF-104: this repository's own migration of `tasks/` to `.taskfactory/tasks`.
