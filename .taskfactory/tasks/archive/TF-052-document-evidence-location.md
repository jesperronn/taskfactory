# TF-052: Document where verification evidence lives

## Goal

The docs state that worker and integration evidence are untracked files under
.taskfactory/, not Git commits, what git status shows after integration, and
that init leaves config untracked so it must be committed before the first
claim.

## Dependencies

None

## Scope

docs/worker-instructions.md, docs/protocol-v1.md and
skills/taskfactory/SKILL.md. Documentation only; no Go code changes.

## Constraints

Describe only what the code and docs do. Read internal/claim before stating
what claim checks: claim reads config from the project root working tree and
bases the worktree on refs/heads/main. If claim does not refuse an untracked
config, say so as "not verified" rather than claiming it refuses. Integrate
requires .taskfactory/config.toml tracked in HEAD and unchanged, and that is
the documented enforcement. Do not edit .mdsmith.yml, bin/lint or the bin/lint
tests.

## Success criteria

### C1: Worker instructions name the worker evidence path

Check: grep -q '.taskfactory/evidence/' docs/worker-instructions.md

### C2: Worker instructions say evidence is not Git commits

Check: grep -q 'not Git commits' docs/worker-instructions.md

### C3: Worker instructions say config is committed before the first claim

Check: grep -q 'before the first claim' docs/worker-instructions.md

### C4: Protocol says evidence files are untracked

Check: grep -q 'Evidence files are untracked' docs/protocol-v1.md

### C5: Skill says evidence is not Git commits

Check: grep -q 'not Git commits' skills/taskfactory/SKILL.md

### C6: Skill says init leaves config untracked

Check: grep -q 'init leaves' skills/taskfactory/SKILL.md

### C7: Skill test still passes

Check: bin/skill.test.sh

## Verification

Run each check from the repository root and report its exit code, then run
bin/test and bin/lint and report their exit codes. Quote the internal/claim
lines relied on for the claim statement.
