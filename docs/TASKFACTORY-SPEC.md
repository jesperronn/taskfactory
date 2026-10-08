# TaskFactory — Bootstrap Specification

Status: Initial design / implementation handoff  
Protocol: v1 draft  
Implementation language: Go  
Configuration: TOML

---

# 1. Objective

TaskFactory is a small, reusable CLI and protocol for coordinating
software-development work across multiple coding agents.

The basic idea is:

> A capable planner turns work into explicit contracts. Workers independently
> implement those contracts and continue testing and repairing their own work
> until the predefined success criteria pass.

TaskFactory should allow relatively small or inexpensive coding models to
execute well-defined tasks safely and autonomously.

The system must support parallel workers without requiring a large orchestration
platform.

TaskFactory itself must remain independent of:

- ChatGPT;
- OpenAI;
- a specific coding agent;
- a specific model;
- a specific model provider;
- a specific programming language used by the target project;
- cloud services.

Agents such as ChatGPT, Codex, OpenCode, Pi, or local models should eventually
be able to participate in the same TaskFactory protocol.

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

> Models decide what work should be done. Deterministic tooling enforces what
> operations are allowed.

---

# 3. Source of truth

TaskFactory must not depend on an LLM remembering system state.

Use:

```text
Git          → source-code state and history
filesystem   → task lifecycle state
TOML         → repository configuration
task files   → work contracts and requirements
CLI          → deterministic state transitions
agents       → planning, implementation and diagnosis
```

There is no hidden TaskFactory server state.

No database is required.

---

# 4. Core protocol invariants

The following rules are fundamental to TaskFactory.

Implementations must preserve them unless a future protocol version explicitly
changes them.

1. Only sufficiently specified tasks may enter `ready`.
2. A task's definition of success is established before worker execution.
3. The planner/specification author owns the test oracle.
4. Workers must never redefine or weaken the test oracle.
5. A task may have at most one active owner.
6. Every implementation worker operates in an isolated Git worktree.
7. At most four implementation workers execute concurrently by default.
8. Workers own their complete implement → verify → diagnose → repair loop.
9. Worker success requires evidence for every required success criterion.
10. Worker success does not imply integration success.
11. Integration is serialized.
12. The integration lock covers rebase → integration verification → fast-forward
    merge.
13. The exact candidate commit verified for integration must be the commit
    merged.
14. Known-red `main` stops ordinary integration, not isolated implementation.
15. Individually successful tasks that conflict create remediation work; neither
    task is retroactively considered an implementation failure.
16. Failed work preserves enough state and evidence for another worker to resume
    rather than restart.
17. Agents must not bypass deterministic TaskFactory lifecycle operations once
    equivalent CLI commands exist.
18. Git is source-code truth; filesystem is lifecycle truth; TOML is
    configuration truth; task contracts are requirements truth.
19. TaskFactory protocol must not depend on a particular agent, model, model
    provider, or cloud service.
20. Task lifecycle state and execution outcome are separate concepts.

These invariants are more important than individual implementation details.

---

# 5. Task lifecycle

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

---

# 6. `inbox/` — needs planning

`inbox` contains work that exists but is not yet safe for autonomous execution.

Examples:

- loose idea;
- incomplete requirement;
- bug report requiring investigation;
- task requiring research;
- task without verification;
- work that is too large;
- work requiring decomposition;
- work with unresolved design decisions.

The planner primarily works here.

The semantic meaning is:

> We know we may want this work, but an autonomous worker should not start it
> yet.

---

# 7. `ready/` — safe to execute

A task enters `ready` only when it is sufficiently specified for immediate
autonomous execution.

The semantic guarantee is:

> A worker may claim an eligible task in `ready` and begin implementation
> without additional project-level planning.

A ready task must have:

- unique task ID;
- clear goal;
- bounded scope;
- relevant constraints;
- predefined success criteria;
- predefined verification;
- known dependencies;
- no unresolved dependencies;
- enough context for autonomous execution.

The distinction is fundamental:

```text
inbox = we know we want this

ready = an autonomous worker can safely execute this now
```

Do not use `ready/` as a generic backlog.

---

# 8. `active/` — claimed work

When a worker claims a ready task, TaskFactory moves it to `active`.

Exactly one worker owns an active task.

Active task state should make it possible to determine at least:

```text
task ID
worker identity
branch
worktree
base commit
start timestamp
```

Claiming must be atomic from TaskFactory's perspective.

Two workers must never successfully claim the same task.

---

# 9. `failed/` — unsuccessful execution

`failed` contains work where the worker could not satisfy the contract within
its configured execution budget.

Failure must preserve evidence.

A failed task should retain enough information that another or stronger worker
can continue rather than starting from scratch.

Relevant information includes:

```text
original task specification
branch
base commit
last/resulting commit
modified files
attempt count
successful checks
failed checks
command output
worker diagnosis
```

A failed branch should not automatically be destroyed.

The orchestrator may decide to:

```text
retry
resume with another worker
use a stronger worker/model
re-plan
split the task
create prerequisite work
abandon the task
```

---

# 10. `archive/` — accepted work

A task enters `archive` only after successful integration.

Worker success alone does not archive a task.

The archive therefore represents work that was:

```text
specified
implemented
verified
integrated
accepted
```

The archive also provides useful historical evidence about why changes were made
and how they were verified.

---

# 11. Lifecycle versus execution outcome

Task lifecycle and worker outcome are different concepts.

Do not initially create:

```text
tasks/blocked/
```

Instead, workers should return sufficiently precise outcomes.

Conceptually:

```text
PASS
FAILED_AFTER_RETRIES
BLOCKED_SPEC
BLOCKED_ENVIRONMENT
```

Additional result types may be introduced if real usage demonstrates a need.

## `PASS`

All required success criteria passed.

## `FAILED_AFTER_RETRIES`

The task appears valid and executable, but the worker exhausted its allowed
effort without satisfying the contract.

## `BLOCKED_SPEC`

The worker cannot proceed because the task contract itself is incomplete,
contradictory, impossible, or requires an unresolved decision.

## `BLOCKED_ENVIRONMENT`

The task appears valid, but something external prevents execution.

Examples:

- required service unavailable;
- missing tool;
- missing credentials;
- inaccessible dependency;
- broken development environment.

The orchestrator determines what lifecycle transition follows an execution
outcome.

---

# 12. Roles

TaskFactory defines four logical roles:

```text
PLANNER
ORCHESTRATOR
WORKER
INTEGRATOR
```

These are roles rather than necessarily separate programs or models.

One agent may perform multiple roles.

Different roles may deliberately use models with different capabilities and
costs.

Model selection itself is outside TaskFactory protocol v1.

---

# 13. Planner

The planner performs semantic work.

Responsibilities:

- inspect inbox;
- understand requirements;
- refine ideas;
- split large work;
- determine dependencies;
- identify parallelizable work;
- define scope;
- define constraints;
- define success criteria;
- define verification;
- move sufficiently specified work toward `ready`.

The planner's central question is:

> What exactly must be true for this work to be considered complete?

---

# 14. Planner owns the test oracle

This is a core design principle.

The planner/specification author defines the test oracle **before implementation
begins**.

The oracle consists of:

```text
requirements
success criteria
verification mechanisms
```

Workers execute against that oracle.

They must not redefine it.

This prevents a worker from deciding after implementation that its own
implementation constitutes success.

The worker may discover that the oracle itself is invalid.

In that case it should return an appropriate blocked result rather than
rewriting the contract.

---

# 15. Orchestrator

The orchestrator manages operational state.

Responsibilities:

- enforce worker count;
- identify eligible ready tasks;
- assign/claim work;
- observe worker completion;
- handle failures;
- handle blocked outcomes;
- retry or escalate work;
- request replanning;
- prioritize remediation;
- control integration state.

Default maximum implementation workers:

```text
4
```

Planner and integrator activity are conceptually separate from these four
implementation slots.

Over time, orchestration should become mostly deterministic code rather than LLM
reasoning.

---

# 16. Worker

A worker owns exactly one claimed task.

Workers operate in isolated Git worktrees.

The worker is responsible for implementation **and its own verification/repair
loop**.

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

or:

```text
the configured execution/retry budget is exhausted
```

or execution is legitimately blocked.

---

# 17. Worker rules

Workers must not:

- remove success criteria;
- weaken success criteria;
- skip required checks;
- delete legitimate tests to obtain green results;
- reinterpret requirements solely to make implementation easier;
- modify unrelated code without justification;
- silently change their own contract.

If a criterion cannot be satisfied because the specification itself appears
invalid, the worker should report `BLOCKED_SPEC`.

---

# 18. Worker scope

Tasks may provide expected or likely files.

For example:

```text
expected_files:
  - internal/task/task.go
  - internal/task/task_test.go
```

Such scope information should initially be treated as **advisory rather than an
absolute filesystem sandbox**.

Workers should remain focused on the task but may modify adjacent files when
necessary.

Unexpected scope expansion should be justified in the worker evidence.

A stricter scope-enforcement mechanism may be added later if real usage
demonstrates the need.

---

# 19. Worker evidence

Workers return evidence rather than merely reporting:

```text
done
```

A successful result should conceptually contain:

```text
status
task ID
worker
base commit
resulting commit
modified files
verification commands
verification results
exit codes
attempt count
relevant retry/failure information
```

The principle is:

> Workers return a proof package, not a claim.

---

# 20. Failed work is resumable

Failure evidence has an important purpose.

A stronger worker should normally be able to continue from:

```text
existing branch
existing commit
existing modifications
existing verification evidence
existing diagnosis
```

rather than restarting the task.

TaskFactory should preserve work products until an explicit policy decides they
are no longer useful.

---

# 21. Verification hierarchy

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

LLM judgment may occasionally be necessary, but it should not replace
deterministic verification when deterministic verification is practical.

---

# 22. Verification levels

Conceptually distinguish three verification levels.

## Worker verification

Answers:

> Does my implementation satisfy my task?

Typically:

- targeted tests;
- compile;
- lint;
- type checks;
- task-specific assertions;
- relevant regressions.

## Integration verification

Answers:

> Does this task still work against the latest main?

This is performed after rebasing the candidate onto current `main`.

It may use a broader test suite.

## Main verification

Answers:

> Is the repository healthy after integration?

This may include:

- smoke tests;
- repository invariants;
- full CI where appropriate.

These levels may overlap.

TaskFactory should not blindly execute the same expensive suite three times
merely because three conceptual verification levels exist.

---

# 23. Post-merge verification

Full post-merge verification is intentionally optional.

If TaskFactory guarantees that:

1. the integration lock remains held;
2. the candidate is rebased onto the current `main`;
3. that exact candidate commit passes integration verification;
4. `git merge --ff-only` moves `main` to that exact commit;

then repeating the complete integration suite after the fast-forward may be
redundant.

Main verification may therefore focus on smoke checks or repository invariants.

Correctness comes from ensuring that:

> The exact commit verified is the exact commit merged.

---

# 24. Worker isolation

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
max implementation workers = 4
```

This remains true even if hundreds of tasks are ready.

---

# 25. Claiming

Claiming must be atomic from TaskFactory's point of view.

Conceptually:

```text
tasks/ready/TF-101
        ↓
tasks/active/TF-101
```

Only one claimant may succeed.

The implementation should use simple filesystem semantics and locking where
necessary.

Do not introduce a database solely for task claiming.

The exact locking implementation remains an open design decision.

---

# 26. Integration

Implementation is parallel.

Integration is serialized.

Use a specialized integration worker/integration role.

The integrator should normally not modify product code.

Its responsibility is to protect `main`.

---

# 27. Integration lock invariant

The integration lock must protect the entire critical sequence:

```text
acquire integration lock
        ↓
establish latest main
        ↓
rebase candidate onto main
        ↓
integration verification
        ↓
git merge --ff-only
        ↓
release integration lock
```

Do not release the lock between verification and merge.

Otherwise another candidate could advance `main`, meaning the commit
relationship that was verified is no longer the relationship being integrated.

The central invariant is:

> The exact candidate commit tested against the exact main state is the
> candidate commit fast-forwarded into main.

---

# 28. Integration procedure

Conceptually:

```text
worker candidate passes
        ↓
integration queue
        ↓
acquire integration lock
        ↓
establish/update latest main
        ↓
rebase candidate onto current main
        ↓
run integration verification
        ↓
checkout/update main
        ↓
git merge --ff-only
        ↓
optional main verification
        ↓
archive task
        ↓
release integration lock
```

The precise Git commands may evolve as implementation is tested.

---

# 29. Fast-forward-only integration

Use:

```bash
git merge --ff-only
```

This prevents unexpected merge commits and detects stale integration
assumptions.

If `main` changes before the candidate can be integrated:

```text
ff-only fails
    ↓
candidate rebases again
    ↓
verification reruns
```

Correctness is preferred over maximum integration throughput.

With only four implementation workers, serialized integration is expected to be
sufficient.

---

# 30. Integration worker

The integration worker is intentionally conservative.

It should normally not write application code.

Responsibilities:

- serialize integration;
- rebase candidates;
- run verification;
- merge using `--ff-only`;
- record integration evidence;
- archive successful tasks.

If integration verification fails, the integrator records the failure and
creates or requests remediation work rather than casually patching product code
itself.

---

# 31. Integration conflict between valid tasks

Two parallel tasks may independently pass but fail when combined.

For example:

```text
TF-101 passes independently
TF-102 passes independently

TF-101 + TF-102 fails
```

Do not automatically declare TF-102 or TF-101 to have been an implementation
failure.

Instead create a remediation/integration task containing information such as:

```text
related tasks:
  TF-101
  TF-102

known good:
  TF-101 individually
  TF-102 individually

base/main commits:
  ...

failing verification:
  ...

goal:
  Resolve the interaction while preserving the
  success criteria of both original tasks.
```

This prevents a repair worker from "fixing" the problem by simply undoing one
otherwise valid implementation.

---

# 32. Stop the line

TaskFactory assumes `main` should remain green.

If `main` becomes known to be broken:

```text
integration pipeline = STOPPED
```

Do not necessarily stop implementation workers.

Their worktrees are isolated and they may continue.

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

No ordinary task may integrate while `main` is known to be broken.

---

# 33. Task dependencies

Tasks may depend on other tasks.

For example:

```text
TF-101 ──► TF-103
TF-102 ──► TF-103
```

TF-101 and TF-102 may execute in parallel.

TF-103 must not be considered executable until its dependencies are satisfied.

The planner identifies semantic dependencies.

TaskFactory enforces them mechanically.

---

# 34. Implementation language

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

# 35. Technical selection criteria

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

These requirements are part of the technical specification.

Future implementation decisions should be evaluated against them.

---

# 36. Why Go

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
- no required runtime on the target machine;
- easy cross-platform builds;
- stable language/tooling;
- straightforward upgrades.

For TaskFactory, Go provides a strong finished-product distribution story while
retaining a small and understandable implementation.

---

# 37. Implementation philosophy

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

# 38. Standard-library-first policy

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

Do not add a dependency simply because it saves a few lines of code.

A dependency should solve a meaningful correctness or maintenance problem.

---

# 39. Dependency policy

Target:

```text
zero or near-zero runtime third-party dependencies
```

Every dependency must have a concrete justification.

Avoid dependencies for:

- CLI parsing;
- logging;
- filesystem operations;
- Git;
- process execution;
- testing;
- assertions.

---

# 40. Configuration format

Use:

```text
TOML
```

Repository configuration:

```text
.taskfactory/config.toml
```

TOML was selected because configuration should remain:

- readable;
- easy for humans;
- easy for agents;
- simple;
- stable.

Go does not contain TOML parsing in the standard library.

Therefore one small, mature TOML parser dependency is acceptable.

Do **not** implement a complicated custom TOML parser merely to claim zero
dependencies.

The goal is minimal dependencies, not zero dependencies at any cost.

The exact TOML library remains an open implementation decision.

---

# 41. Example configuration

```toml
protocol_version = 1

[workers]
max_parallel = 4

[git]
use_worktrees = true
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = [
    "go test ./..."
]

integration = [
    "go test ./...",
    "go vet ./..."
]
```

Configuration should remain intentionally narrow.

TaskFactory must not evolve into a generic build configuration language.

---

# 42. Protocol versioning

Repository configuration includes:

```toml
protocol_version = 1
```

The CLI must not silently reinterpret repositories using unsupported protocol
versions.

Unsupported versions should produce an explicit error or clearly defined
compatibility behavior.

---

# 43. Task representation

Task files must be:

- human-readable;
- agent-readable;
- machine-validatable;
- easy to edit;
- concise.

The final serialization format remains intentionally unresolved.

The primary candidates are:

```text
pure TOML
```

or:

```text
Markdown with structured metadata
```

The decision should be based on:

- readability;
- deterministic parsing;
- dependency count;
- validation simplicity;
- agent ergonomics;
- ability to represent verification evidence cleanly.

Do not finalize the representation solely for aesthetic reasons.

---

# 44. Task contract

Regardless of serialization, a task conceptually contains:

```text
id
title
goal
dependencies
scope
constraints
success criteria
verification
```

Runtime/evidence information may either be appended or stored separately if that
keeps the task contract cleaner.

---

# 45. Task contract example

Conceptually:

```text
id: AUTH-017

title:
Reject expired SAML assertions

goal:
Reject assertions whose NotOnOrAfter timestamp
is earlier than the current time.

dependencies:
none

expected files:
  SamlValidator.java
  SamlValidatorTest.java

constraints:
  Do not change public APIs.
  Do not upgrade dependencies.

SC1:
  requirement:
    Expired assertions are rejected.

  verification:
    ./mvnw -Dtest=SamlValidatorTest test

SC2:
  requirement:
    Existing authentication behavior remains valid.

  verification:
    ./mvnw test
```

`expected files` are advisory scope information rather than necessarily an
absolute restriction.

---

# 46. Project-specific verification

TaskFactory must work across arbitrary projects.

Examples:

Go:

```text
go test ./...
go vet ./...
```

Java:

```text
./mvnw test
./mvnw verify
```

Node:

```text
npm test
npm run lint
npm run build
```

Rust:

```text
cargo test
cargo clippy
```

TaskFactory executes project-defined verification rather than understanding
every build ecosystem itself.

---

# 47. CLI

Executable:

```text
taskfactory
```

It should eventually be possible to install the binary somewhere on `$PATH`.

Initial command direction:

```text
taskfactory --help
taskfactory --version

taskfactory init
taskfactory status
taskfactory validate
taskfactory claim
taskfactory verify
taskfactory integrate
```

Do not implement every conceivable command before real usage demonstrates the
need.

---

# 48. CLI implementation

Prefer Go's standard:

```text
flag
```

package initially.

Do not introduce Cobra or another CLI framework unless actual CLI complexity
justifies it.

The CLI should remain predictable for both humans and agents.

Commands must use meaningful non-zero exit codes on failure.

Errors must be actionable.

Prefer:

```text
error: task TF-102 cannot be claimed:
dependency TF-099 is not archived
```

over:

```text
claim failed
```

---

# 49. Agents must use deterministic operations

As TaskFactory commands become available, agents should use those commands
instead of directly manipulating TaskFactory state.

For example, once claiming is implemented, an agent should use something
conceptually equivalent to:

```text
taskfactory claim TF-101
```

rather than manually moving:

```text
tasks/ready/TF-101
```

to:

```text
tasks/active/TF-101
```

The filesystem remains the lifecycle source of truth, but the CLI becomes the
safe mechanism for modifying that state.

This allows TaskFactory to enforce invariants consistently.

---

# 50. Testing philosophy

TaskFactory itself should be highly testable.

Use Go's standard:

```text
testing
```

package.

Prefer:

- table-driven tests;
- `t.TempDir()`;
- temporary Git repositories;
- real filesystem behavior;
- real local Git commands;
- deterministic tests.

Avoid excessive mocking.

Where behavior concerns Git itself, invoking real local Git against a temporary
repository is preferred over mocking Git behavior.

---

# 51. Test layers

TaskFactory should eventually have:

## Unit tests

For:

- task validation;
- configuration;
- lifecycle rules;
- path handling;
- state transitions.

## Integration tests

For:

- repository initialization;
- task claiming;
- worktree creation;
- worker lifecycle;
- rebasing;
- ff-only integration;
- stop-the-line state.

## Example-project tests

A separate `taskfactory-examples` repository can provide minimal external
projects for end-to-end TaskFactory behavior.

---

# 52. Portability

Initial practical targets:

```text
macOS
Linux
```

Do not deliberately prevent Windows support.

Prefer Go filesystem/process APIs over shell-specific constructs.

Call the `git` executable directly through process execution rather than
constructing large shell scripts.

---

# 53. Distribution

Public release/distribution is intentionally deferred.

Possible future distribution includes:

```text
GitHub Releases
Homebrew
go install
platform-specific standalone binaries
other package managers
```

The architecture must not prevent these.

Important early rule:

> The `taskfactory` executable must not assume that it is running from the
> TaskFactory source checkout.

Do not build release infrastructure yet.

---

# 54. TaskFactory Skill

TaskFactory should eventually have a reusable Skill.

However:

> The Skill is not the protocol, state machine, or source of truth.

The Skill is a thin intelligent frontend.

Conceptually:

```text
TaskFactory Skill
       ↓
TaskFactory CLI
       ↓
TaskFactory protocol
       ↓
repository configuration
       ↓
task contracts
```

The Skill may teach an agent:

- how TaskFactory works;
- when planning is necessary;
- how to refine tasks;
- how to use the CLI;
- how to interpret failures;
- how to orchestrate work.

The Skill must not implement a parallel competing version of TaskFactory
lifecycle logic.

Once the CLI exposes an operation, the Skill should invoke that operation rather
than bypassing it.

This also allows agents without ChatGPT Skills to participate in the same
protocol.

---

# 55. Agent and model independence

A likely TaskFactory deployment may use:

```text
strong planner
      ↓
precise task contracts
      ↓
smaller / cheaper local workers
```

But this is a deployment choice, not a protocol requirement.

TaskFactory must not encode assumptions about:

- OpenAI;
- Anthropic;
- Google;
- Ollama;
- LM Studio;
- specific models;
- model pricing;
- cloud inference.

Local and air-gapped execution must remain possible.

---

# 56. Examples repository

Examples should eventually live separately:

```text
~/src/taskfactory-examples
```

rather than inside TaskFactory.

This better simulates arbitrary external repositories.

Examples must remain extremely small so agents learn TaskFactory rather than
application architecture.

---

# 57. When to build examples

Do not overbuild example projects before the basic CLI exists.

A useful point to create the first external example is after core commands such
as:

```text
taskfactory init
taskfactory status
taskfactory validate
taskfactory claim
```

exist.

The example project can then be created and operated on using actual TaskFactory
behavior rather than hypothetical conventions.

---

# 58. First example

A first external project could be:

```text
taskfactory-examples/
└── go-simple/
    ├── calculator.go
    └── calculator_test.go
```

Possible tasks:

```text
add subtraction
add multiplication
add division
```

The purpose is to demonstrate:

```text
inbox
  ↓
ready
  ↓
active
  ↓
worker worktree
  ↓
self verification
  ↓
integration
  ↓
archive
```

---

# 59. Later examples

Add examples only when needed to exercise specific behavior.

Useful later examples:

```text
parallel-safe
integration-conflict
broken-main
worker-failure
dependency-chain
```

Especially valuable:

## Integration conflict

Two tasks independently pass but fail together.

## Broken main

Demonstrates stop-the-line and remediation.

---

# 60. TaskFactory should dogfood TaskFactory

TaskFactory itself should use its own task lifecycle.

The repository should therefore contain:

```text
tasks/
├── inbox/
├── ready/
├── active/
├── failed/
└── archive/
```

Implementation work should use the same contracts that TaskFactory eventually
expects external projects to use.

This allows weaknesses in the protocol to be discovered while building the tool.

---

# 61. Repository structure

Start small.

Suggested initial repository:

```text
taskfactory/
├── README.md
├── TASKFACTORY-SPEC.md
├── go.mod
├── cmd/
│   └── taskfactory/
│       └── main.go
├── internal/
├── docs/
├── tasks/
│   ├── inbox/
│   ├── ready/
│   ├── active/
│   ├── failed/
│   └── archive/
└── skill/
```

Do not create empty package hierarchies merely because this specification lists
possible future areas.

Create packages when implementation actually needs them.

Likely future packages may include:

```text
internal/config
internal/task
internal/git
internal/workspace
internal/integration
```

but these are suggestions, not required scaffolding.

---

# 62. Initial implementation backlog

The following IDs establish the initial direction.

Do not blindly consider every item `ready`.

Only tasks with sufficiently explicit contracts and verification belong in
`ready/`.

---

## TF-001 — CLI skeleton

Goal:

Create the minimal Go `taskfactory` executable.

Requirements:

- establish `go.mod`;
- create CLI entry point;
- support `taskfactory --help`;
- support `taskfactory --version`;
- establish automated tests;
- do not implement lifecycle behavior yet;
- do not implement public distribution.

Success criteria:

```text
go test ./...
```

passes.

```text
taskfactory --help
```

exits successfully.

```text
taskfactory --version
```

exits successfully and reports a version.

The CLI can be built using:

```text
go build
```

No unnecessary CLI dependency is introduced.

---

## TF-002 — `taskfactory init`

Goal:

Initialize TaskFactory in an arbitrary Git repository.

Expected result:

```text
.taskfactory/
└── config.toml

tasks/
├── inbox/
├── ready/
├── active/
├── failed/
└── archive/
```

Requirements:

- preserve existing work;
- fail clearly when initialization is unsafe;
- initialization should be idempotent where reasonable;
- generate a minimal default configuration;
- do not require the TaskFactory source repository.

Success criteria should include tests against temporary Git repositories.

---

## TF-003 — Task format and validation contract

Goal:

Define the first machine-readable TaskFactory task contract.

The implementation must explicitly evaluate:

```text
TOML task
```

versus:

```text
Markdown + structured metadata
```

against the technical requirements in this document.

The resulting format must represent:

```text
id
title
goal
dependencies
scope
constraints
success criteria
verification
```

The format must be:

- easy for agents;
- easy for humans;
- machine-validatable;
- simple to parse;
- maintainable.

Provide valid and invalid test fixtures.

---

## TF-004 — `taskfactory validate`

Goal:

Mechanically validate TaskFactory tasks.

Requirements:

- valid task → exit 0;
- invalid task → non-zero;
- actionable errors;
- identify failed rule/field;
- support validating one task;
- later may support repository-wide validation.

Depends on:

```text
TF-001
TF-003
```

---

## TF-005 — `taskfactory status`

Goal:

Show concise repository TaskFactory state.

At minimum report counts for:

```text
inbox
ready
active
failed
archive
```

Also eventually report integration state:

```text
GREEN
STOPPED
```

Output should be readable by humans and predictable enough for agents.

---

## TF-006 — Atomic claim and worktree creation

Goal:

Allow a worker to claim a ready task and receive isolated Git state.

Requirements:

- exactly one worker can successfully claim a task;
- move lifecycle from `ready` to `active`;
- create task branch;
- create/assign worktree;
- record worker;
- record branch;
- record worktree;
- record base commit.

Concurrent claim attempts for the same task must not both succeed.

Depends on:

```text
TF-002
TF-004
```

---

# 63. Initial inbox

The following work is known but should remain in `inbox/` until sufficiently
specified:

```text
TF-007  minimal external example project
TF-008  TaskFactory Skill
TF-009  public installation/distribution
TF-010  worker verification command
TF-011  integration queue and ff-only integration
TF-012  stop-the-line implementation
TF-013  worker evidence/failure recording
TF-014  remediation-task creation
TF-015  orchestrator max-concurrency enforcement
```

Additional tasks should be created as implementation reveals missing protocol
behavior.

---

# 64. Initial dependency direction

A rough early dependency graph:

```text
TF-001 CLI
  │
  ├──────────────┐
  │              │
  ▼              ▼
TF-002 init    TF-003 task contract
  │              │
  │              ▼
  │           TF-004 validate
  │              │
  └──────┬───────┘
         │
         ▼
      TF-006 claim/worktree

TF-002
  │
  ▼
TF-005 status
```

TF-001 and TF-003 may be able to progress largely independently once the initial
repository skeleton exists.

---

# 65. Initial implementation sequence

The first useful milestone is:

```text
taskfactory --help
taskfactory --version
taskfactory init
taskfactory status
taskfactory validate
taskfactory claim
```

At that point TaskFactory can begin exercising its own basic protocol.

Next milestone:

```text
worker verification
worker evidence
integration queue
rebase
ff-only integration
archive
```

Next:

```text
parallel workers
integration failure
stop-the-line
remediation
```

Only after the basic lifecycle works should substantial effort shift toward:

```text
Skill
external examples
public packaging
release automation
```

---

# 66. Open design decisions

The following questions are intentionally unresolved.

Do not allow an implementation agent to accidentally turn an incidental
implementation choice into permanent protocol without considering these
tradeoffs.

## 66.1 Task serialization

Choose between:

```text
pure TOML
```

and:

```text
Markdown + structured metadata
```

Evaluate based on:

- human readability;
- agent readability;
- parsing;
- validation;
- dependency count;
- evidence representation.

---

## 66.2 TOML library

Go has no TOML parser in its standard library.

Select one mature, narrowly scoped TOML dependency if necessary.

The decision should consider:

- maintenance;
- correctness;
- API simplicity;
- transitive dependencies;
- long-term stability.

Do not write a substantial custom TOML parser merely to avoid one dependency.

---

## 66.3 Lock implementation

The exact mechanism for:

```text
task claiming
integration locking
```

is not yet specified.

Possible mechanisms include filesystem operations, lock files, atomic renames,
or operating-system locking.

The selected implementation must preserve the protocol invariants and work
reliably on the initial target platforms.

Keep it simple.

---

## 66.4 Worker execution budget

The worker self-repair loop requires an eventual budget.

The exact semantics are not yet defined.

Possible dimensions include:

```text
attempt count
wall-clock time
agent turns
token/model budget
external orchestrator decision
```

Do not prematurely encode a complicated budgeting system.

Start with the smallest mechanism needed for actual orchestration.

---

# 67. Explicit v1 non-goals

Do not initially implement:

- database;
- daemon;
- HTTP API;
- web UI;
- cloud service;
- Kubernetes integration;
- distributed task queue;
- remote worker protocol;
- generic workflow engine;
- model marketplace;
- provider-specific agent integration;
- automatic model selection;
- elaborate telemetry;
- generic plugin architecture;
- public release infrastructure;
- automatic dependency installation.

These can be added later if real requirements justify them.

---

# 68. Decision rule for implementation

When choosing between two solutions, prefer the one that is:

1. smaller;
2. simpler;
3. easier to explain;
4. easier to test;
5. easier for coding agents to modify;
6. more deterministic;
7. based on the Go standard library;
8. dependent on fewer external packages;
9. easier to maintain;
10. easier to distribute.

Additional complexity requires an observed problem, not a hypothetical future
requirement.

---

# 69. Current repository

Development begins from:

```text
~/src/taskfactory
```

Git branch:

```text
main
```

Initial commit:

```text
fd991ee
chore: TaskFactory initial commit (empty on purpose)
```

The repository was intentionally initialized empty.

This document represents the accumulated architectural and technical decisions
made before implementation begins.

Once these decisions are committed to the repository, the repository becomes the
source of truth rather than this conversation.

---

# 70. Immediate next action

Use this specification to bootstrap the repository itself.

The first bootstrap should:

1. preserve this specification as `TASKFACTORY-SPEC.md`;
2. preserve the high-level process overview in `README.md`;
3. create the task lifecycle directories;
4. create the initial task contracts;
5. place only genuinely executable tasks in `ready/`;
6. leave unresolved work in `inbox/`;
7. create only the minimum Go scaffolding required by the first implementation
   task;
8. avoid speculative architecture;
9. begin dogfooding TaskFactory's task model immediately.

Do not implement the whole system in one large change.

The project should evolve through the same small, explicit, verifiable tasks
that TaskFactory is intended to manage.

---

# 71. Project tagline

**Plan intelligently. Specify precisely. Work in parallel. Verify locally.
Integrate carefully.**
