# TaskFactory — Bootstrap Specification

Status: Initial design / implementation handoff  
Protocol: v1 draft  
Implementation language: Go

---

# 1. Objective

TaskFactory is a small, reusable CLI and protocol for coordinating software-development work across multiple coding agents.

The basic idea is:

> A capable planner turns work into explicit contracts. Workers independently implement those contracts and continue testing and repairing their own work until the predefined success criteria pass.

TaskFactory should allow relatively small or inexpensive coding models to execute well-defined tasks safely and autonomously.

The system must support parallel workers without requiring a large orchestration platform.

TaskFactory itself should be independent of:

- ChatGPT;
- OpenAI;
- a specific coding agent;
- a specific model;
- a specific programming language used by the target project;
- cloud services.

Agents such as ChatGPT, Codex, OpenCode, Pi, or local models should eventually be able to participate in the same TaskFactory protocol.

---

# 2. Core philosophy

TaskFactory separates:

```text
planning
    ↓
task contracts
    ↓
parallel implementation
    ↓
worker self-verification
    ↓
serialized integration
```

The important distinction is between:

```text
LLM reasoning
```

and:

```text
deterministic enforcement
```

Use agents for:

- understanding requirements;
- decomposition;
- implementation;
- diagnosis;
- repair.

Use deterministic tooling for:

- task state;
- task claiming;
- concurrency limits;
- Git operations;
- validation;
- verification execution;
- integration locking;
- lifecycle transitions.

The guiding rule is:

> Models decide what work should be done. Deterministic tooling enforces what operations are allowed.

---

# 3. Task lifecycle

Use:

```text
tasks/
├── inbox/
├── ready/
├── active/
├── failed/
└── archive/
```

These directories are the authoritative lifecycle state.

## `inbox/`

Contains work that exists but is not yet safe for autonomous execution.

Examples:

- loose idea;
- incomplete requirement;
- task requiring research;
- task without verification;
- work that is too large;
- work requiring decomposition;
- work with unresolved design decisions.

The planner primarily works here.

---

## `ready/`

Contains tasks that are sufficiently specified for immediate execution.

The semantic guarantee of `ready/` is important:

> A worker may claim any eligible task in `ready/` and begin implementation without performing additional project-level planning.

A ready task must have:

- unique task ID;
- clear goal;
- bounded scope;
- explicit success criteria;
- verification instructions;
- known dependencies;
- no unresolved dependencies;
- enough context for autonomous execution.

A task must not be moved to `ready/` simply because somebody wants it implemented.

`ready` means executable.

---

## `active/`

Contains claimed work.

Exactly one worker owns an active task.

The runtime state should make it possible to determine at least:

- task ID;
- worker identity;
- branch;
- worktree;
- base commit;
- start time.

Claiming must prevent two workers from successfully claiming the same task.

---

## `failed/`

Contains work where the worker could not satisfy the contract within its configured effort/retry budget.

Failure must preserve evidence.

A failed task should retain enough information that a stronger worker can continue rather than starting over.

Relevant information includes:

- original specification;
- attempted implementation;
- branch;
- last commit;
- modified files;
- successful checks;
- failed checks;
- command output;
- worker diagnosis;
- number of attempts.

The orchestrator decides whether to:

- retry;
- use a stronger worker;
- re-plan;
- split the task;
- create prerequisite work;
- abandon the task.

---

## `archive/`

Contains successfully implemented, verified, integrated, and accepted tasks.

Worker success alone does not move a task into `archive/`.

Integration must succeed first.

`archive/` therefore also acts as useful project history.

---

# 4. Blocked work

Do not initially create:

```text
tasks/blocked/
```

Instead, `BLOCKED` is an execution result.

Examples:

- prerequisite unexpectedly missing;
- contradictory requirements;
- required service unavailable;
- missing human decision;
- task based on invalid assumptions;
- required dependency unavailable.

The orchestrator decides what to do with blocked work.

Possible actions:

```text
retry
re-plan
create prerequisite
escalate
abandon
```

A dedicated blocked lifecycle state can be introduced later if real usage demonstrates a need.

---

# 5. Roles

TaskFactory defines four logical roles:

```text
PLANNER
ORCHESTRATOR
WORKER
INTEGRATOR
```

They are roles, not necessarily separate programs or models.

One agent may perform multiple roles.

---

# 6. Planner

The planner performs semantic work.

Responsibilities:

- inspect inbox;
- understand requirements;
- refine ideas;
- split large work;
- determine dependencies;
- identify parallelizable work;
- define constraints;
- define success criteria;
- define verification;
- move sufficiently specified work toward `ready`.

The planner answers:

> What exactly must be true for this work to be considered complete?

Success criteria must be defined before implementation.

The worker must not be allowed to redefine "done."

---

# 7. Orchestrator

The orchestrator manages operational state.

Responsibilities:

- schedule tasks;
- enforce worker concurrency;
- claim work;
- manage worker slots;
- handle worker results;
- retry or escalate failures;
- prioritize remediation;
- invoke planning when necessary;
- manage integration state.

Default maximum implementation workers:

```text
4
```

The orchestrator should eventually be mostly deterministic.

It should not require a highly capable model for routine state transitions.

---

# 8. Worker

A worker owns exactly one task.

Workers operate in isolated Git worktrees.

The worker is responsible for implementation **and its own verification loop**.

The intended lifecycle is:

```text
inspect task
    ↓
inspect relevant code
    ↓
implement
    ↓
verify
    ↓
PASS ─────────────────────────┐
                              │
FAIL                          │
 ↓                            │
diagnose                      │
 ↓                            │
repair                        │
 ↓                            │
verify again ─────────────────┘
```

The worker continues until:

```text
all required success criteria pass
```

or its configured execution budget is exhausted.

The factory should not have to tell the worker after every failed test what to fix.

The worker owns that feedback loop.

---

# 9. Worker rules

Workers must not:

- remove success criteria;
- weaken success criteria;
- skip required checks;
- delete legitimate tests to obtain green results;
- reinterpret requirements solely to make implementation easier;
- modify unrelated code without justification.

If a criterion cannot be satisfied because the specification itself appears invalid, the worker should report that rather than silently changing the contract.

---

# 10. Worker output

Workers return evidence rather than merely:

```text
done
```

Example conceptual result:

```text
status: PASS

task: AUTH-017
base_commit: ...
result_commit: ...

modified_files:
  ...

verification:
  SC1:
    command: ...
    exit_code: 0

  SC2:
    command: ...
    exit_code: 0

attempts: 2
```

The principle is:

> Workers return a proof package, not a claim.

---

# 11. Verification

Prefer deterministic verification.

Preferred hierarchy:

```text
tests
compiler
type checker
lint
build
schema validation
filesystem assertions
Git assertions
structured output comparison
LLM judgement
worker self-assessment
```

LLM judgment may occasionally be necessary, but it should not replace deterministic verification when deterministic verification is practical.

---

# 12. Verification levels

Conceptually distinguish three verification levels.

## Worker verification

Answers:

> Does my implementation satisfy my task?

Typically:

- targeted tests;
- compile;
- lint;
- task-specific assertions;
- relevant regressions.

---

## Integration verification

Answers:

> Does this task still work against the latest main?

Typically broader than worker verification.

---

## Main verification

Answers:

> Is the repository healthy after integration?

These levels do not necessarily require running three completely separate suites.

The distinction exists so verification policy can evolve and be optimized.

---

# 13. Worker isolation

Every parallel implementation worker gets a separate Git worktree.

Conceptually:

```text
main

task/TF-101 ─── worktree 1
task/TF-102 ─── worktree 2
task/TF-103 ─── worktree 3
task/TF-104 ─── worktree 4
```

Workers must not share mutable working directories.

Default:

```text
max workers = 4
```

This remains true even if hundreds of tasks are ready.

---

# 14. Integration

Implementation is parallel.

Integration is serialized.

Use a specialized integration worker / integration role.

The integrator should normally not modify product code.

Its job is verification and integration.

---

# 15. Integration procedure

Conceptually:

```text
worker candidate passes
        ↓
integration queue
        ↓
acquire integration lock
        ↓
update latest main
        ↓
rebase candidate onto main
        ↓
run integration verification
        ↓
git merge --ff-only
        ↓
optional main verification
        ↓
archive task
        ↓
release integration lock
```

The lock should cover the period where the exact candidate is rebased, verified, and merged.

This ensures the commit that was verified is the commit that gets integrated.

---

# 16. Fast-forward-only integration

Use:

```bash
git merge --ff-only
```

This deliberately rejects stale integration assumptions.

If main moves before the candidate is integrated:

```text
ff-only fails
    ↓
candidate must rebase again
    ↓
verification reruns
```

With only four implementation workers, serialized integration is acceptable.

Optimize this only if actual usage demonstrates integration contention.

---

# 17. Integration conflicts between valid work

Two parallel tasks may independently pass but fail when combined.

For example:

```text
TF-101 passes independently
TF-102 passes independently

TF-101 + TF-102 fails
```

Do not automatically declare one original task invalid.

Create a remediation/integration task containing information such as:

```text
related tasks:
  TF-101
  TF-102

known good:
  TF-101 individually
  TF-102 individually

failing verification:
  ...

goal:
  Resolve the interaction while preserving the success
  criteria of both original tasks.
```

This prevents a repair worker from simply undoing one otherwise valid implementation.

---

# 18. Stop the line

If main becomes known to be broken:

```text
STOP INTEGRATION
```

Do not necessarily stop implementation workers.

Workers remain isolated and may continue.

Conceptually:

```text
W1 ─┐
W2 ─┤
W3 ─┤ continue working
W4 ─┘

       integration
            X
            │
       main is RED
            │
     remediation task
            │
        highest priority
            │
       main becomes GREEN
            │
     integration resumes
```

No ordinary work integrates while main is known to be broken.

---

# 19. Source of truth

Do not depend on an LLM remembering TaskFactory state.

Use:

```text
filesystem  → task lifecycle
Git         → source-code state
TOML        → repository configuration
task files  → work contracts
CLI         → deterministic operations
agents      → planning, implementation, diagnosis
```

No database is required.

No TaskFactory server is required.

---

# 20. Implementation language

TaskFactory is implemented in:

```text
Go
```

We considered:

- Bash;
- JavaScript/TypeScript;
- Ruby;
- Python;
- Go;
- Rust;
- other compiled alternatives.

Go was selected as the best fit for the intended finished tool.

---

# 21. Technical selection criteria

The TaskFactory implementation must prioritize:

1. small codebase;
2. simple code;
3. few files;
4. easy maintenance;
5. easy understanding by coding agents;
6. straightforward testing;
7. straightforward packaging;
8. straightforward public distribution;
9. easy installation on `$PATH`;
10. easy upgrades to newer language versions;
11. extensive use of the standard library;
12. little or no third-party dependencies;
13. cross-platform behavior where practical;
14. deterministic behavior;
15. no unnecessary runtime infrastructure.

These requirements are part of the technical specification and should be considered when evaluating future implementation decisions.

---

# 22. Why Go

TaskFactory primarily consists of:

```text
filesystem operations
+
Git subprocesses
+
structured state
+
process execution
+
CLI behavior
```

Go provides:

- strong filesystem APIs;
- excellent process execution;
- excellent standard library;
- built-in testing;
- straightforward concurrency;
- simple compilation;
- single executable distribution;
- no required runtime on the target system;
- easy cross-platform builds;
- stable language/tooling;
- simple long-term upgrades.

For TaskFactory, Go provides a better finished-product distribution story than Python while retaining a small and understandable implementation.

---

# 23. Implementation philosophy

Prefer boring code.

Avoid architecture for hypothetical future requirements.

Prefer:

```text
explicit functions
simple structs
filesystem operations
os/exec
small packages
standard Go testing
```

Avoid unless demonstrated necessary:

```text
dependency injection frameworks
plugin frameworks
ORMs
databases
servers
RPC
event buses
large CLI frameworks
logging frameworks
workflow engines
generated code
```

---

# 24. Standard-library-first policy

Use Go's standard library wherever practical.

Likely packages include:

```text
os
io
io/fs
path/filepath
os/exec
encoding/json
flag
testing
errors
fmt
strings
time
sync
context
```

Do not add a dependency simply because it makes ten lines of code become five.

A dependency should solve a meaningful maintenance or correctness problem.

---

# 25. Dependency policy

Target:

```text
zero or near-zero runtime third-party dependencies
```

Every dependency must have a concrete justification.

Avoid dependencies for:

- CLI parsing;
-