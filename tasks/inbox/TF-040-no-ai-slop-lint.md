# TF-040: Lint for AI-generated prose tells

## Goal

Keep TaskFactory's task files and documentation free of mechanical AI-slop
phrasing, using mdsmith, so the written contracts stay precise and plain.

## Dependencies

- TF-038 must conclude with mdsmith adopted and `.mdsmith.yml` in place.

## Scope

mdsmith ships a `no-llm-tells` convention: MDS056 blocks a curated vocabulary of
tell words and phrases, MDS055 blocks banned sentence openers, and MDS023 and
MDS024 tighten readability budgets. A project selects only one `convention:`
key, so combining it with the `github` convention from TF-038 needs either a
user-defined convention (`conventions:` in `.mdsmith.yml`) that merges both rule
sets, or the relevant rules copied into the project `rules:` block. Pick the
simpler option and verify the merge with `mdsmith kinds resolve <file>`. Run it
over `docs/`, `README.md` and `tasks/**/*.md`, review every finding by hand, and
classify each as a true tell to reword, a false positive to allow-list, or a
domain term the project must keep. Decide which scope fails `bin/lint` (for
example new and changed files only) and which stays advisory until the backlog
is cleaned. Add project terms through the rule's append-only lists.

## Constraints

Do not change the meaning of specifications or acceptance criteria while
rewording. Do not weaken other lint rules to make room. Keep rewording of
existing archived tasks out of scope; archived records are history.

## Notes

Related to TF-039 (readability rule for workers). Keep the two decisions
separate: this one is about style tells, TF-039 is about worker context size.
Unrefined; promote to ready once TF-038 has landed and a first run shows how
many findings there are.
