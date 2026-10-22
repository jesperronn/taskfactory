# TF-076: First release v1.0.0-rc.1

## Goal

Prepare the first release, `v1.0.0-rc.1`, and record the checklist the owner
runs to confirm the whole mechanism on GitHub. Only the owner can finish this
task end to end.

## Dependencies

- TF-074
- TF-075

## Scope

Prepare and record a checklist in `docs/release.md` (a section "First
release"), then the owner runs it. Owner-performed steps are marked as such:

1. (Worker) Run `bin/release v1.0.0-rc.1` as a dry run on a clean main and
   record the output.
2. (Owner) Set `tag.gpgSign` to true in the git config, or decide to use
   `--allow-unsigned`.
3. (Owner) Run `bin/release v1.0.0-rc.1 --tag`, review, then push the tag
   (`git push origin v1.0.0-rc.1`) or use `--publish`. Never both paths.
4. (Owner) Check the Actions run and the release page: GitHub marks it a
   prerelease, it lists four binary archives, the skill archive and
   `SHA256SUMS`, and the checksums verify.
5. (Owner) Download one archive and run `taskfactory --version`, expecting
   `taskfactory version v1.0.0-rc.1`.
6. (Owner) After the release exists, verify the first real use of
   `taskfactory skill install` (TF-095) against it. Listed here so it is not
   forgotten.
7. (Worker) Record the outcome in the task and remove "planned" wording about
   release from the README and SKILL.md.

## Constraints

Workers never create the real tag, push or publish. Do not publish anything
without an explicit owner ask. Do not claim steps 2 to 5 passed; the owner
reports them. Record what is unverified.

## Success criteria

### C1: The checklist exists in the release document

Check: grep -q 'First release' docs/release.md

### C2: The dry run command is documented with the version

Check: grep -q 'bin/release v1.0.0-rc.1' docs/release.md

### C3: README drops the planned release policy wording

Check: test "$(grep -c 'planned release policy' README.md)" -eq 0

### C4: README drops the bin/release does not exist wording

Check: test "$(grep -c 'does not exist yet' README.md)" -eq 0

### C5: README states the bin/release dry run exists

Check: grep -q 'bin/release dry run' README.md

### C6: SKILL.md names the bin/release command

Check: grep -q 'bin/release' skills/taskfactory/SKILL.md

### C7: The skill archive test passes

Check: bin/skill.test.sh

### C8: Docs lint clean

Check: bin/lint docs/release.md README.md skills/taskfactory/SKILL.md

## Verification

Workers run the checks and the dry run and report exit codes. Owner-only
steps 2 to 5 are verified by the owner, who reports the Actions run URL, the
release page state and the printed version. Do not archive until the owner
confirms.
