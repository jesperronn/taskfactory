# TF-076: First release v1.0.0-rc.1

## Goal

Cut the first release, `v1.0.0-rc.1`, and confirm the whole mechanism on
GitHub. Only a human can finish this task end to end.

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
6. (Owner) After the release exists, the first real use of
   `taskfactory skill install` (TF-079) against it is verified; that is a
   TF-079 human check, listed here so it is not forgotten.
7. (Worker) Record the outcome in the task and remove "planned" wording about
   release from the README and SKILL.md.

## Constraints

Workers never create the real tag, push or publish. Do not claim steps 2 to 5
passed; the owner reports them. Record what is unverified.

## Success criteria

### C1: The checklist exists in the release document

Check: grep -q 'First release' docs/release.md

### C2: The dry run command is documented with the version

Check: grep -q 'bin/release v1.0.0-rc.1' docs/release.md

### C3: The skill no longer says release is planned

Check: bin/skill.test.sh

### C4: Docs lint clean

Check: bin/lint docs/release.md README.md skills/taskfactory/SKILL.md

## Verification

Workers run the checks and the dry run and report exit codes. Owner-only
steps 2 to 5 are verified by the owner, who reports the Actions run URL, the
release page state and the printed version. Do not archive until the owner
confirms.
