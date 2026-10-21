package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"taskfactory/internal/skillinstall"
	"taskfactory/internal/ui"
	"taskfactory/skills"
)

const skillUsageLine = "usage: taskfactory skill <install|status> [--target claude|agents|dir:<path>] [--global]"

// skillHelp is printed by "skill --help".
const skillHelp = `Usage: taskfactory skill <install|status>

Install the agent skill that is embedded in this binary, or check whether an
installed copy is current. No network or release archive is involved, so the
version of the binary decides the skill text.

With --dry-run nothing changes.

Next step: taskfactory skill install, then taskfactory skill status.

Commands:
  install     write SKILL.md and .taskfactory-skill into the skill directory
  status      report whether the installed skill is present, missing or stale

Flags:
  --target <claude|agents|dir:<path>>  choose the skill directory
  --global    use $HOME instead of the project root
  --force     install only: replace an existing skill directory
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h  print this help and exit successfully

Exit codes:
  0  done, or the skill is present
  1  refused, missing or stale
  2  invalid usage
`

const skillInstallHelp = `Usage: taskfactory skill install [--target <claude|agents|dir:<path>>] [--global] [--force]

Write the embedded skill into the skill directory: SKILL.md and the stamp file
.taskfactory-skill, and nothing else. The default directory is
.claude/skills/taskfactory when .claude/ exists in the project root and
.agents/skills/taskfactory otherwise. It refuses when the directory exists
unless --force, which replaces that one directory.

Next step: taskfactory skill status

Flags:
  --target <claude|agents|dir:<path>>  choose the skill directory
  --global    use $HOME instead of the project root
  --force     replace an existing skill directory
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h  print this help and exit successfully

Exit codes:
  0  the skill was installed
  1  refused or failed
  2  invalid usage
`

const skillStatusHelp = `Usage: taskfactory skill status [--target <claude|agents|dir:<path>>] [--global]

Print each path inspected as "checked: <path>", then present, missing or stale.
The skill is present when SKILL.md matches the embedded skill and the stamp
agrees. Nothing is written.

Next step: taskfactory skill install

Flags:
  --target <claude|agents|dir:<path>>  choose the skill directory
  --global    use $HOME instead of the project root
  --verbose   write detail lines starting "verbose: " to stderr
  --help, -h  print this help and exit successfully

Exit codes:
  0  the skill is present
  1  the skill is missing or stale
  2  invalid usage
`

// skillHelpFor returns the help text for the args after "skill".
func skillHelpFor(args []string) string {
	if len(args) > 0 {
		switch args[0] {
		case "install":
			return skillInstallHelp
		case "status":
			return skillStatusHelp
		}
	}
	return skillHelp
}

type skillArgs struct {
	target string
	global bool
	force  bool
}

func parseSkillArgs(sub string, args []string) (skillArgs, error) {
	var a skillArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, val, hasVal := strings.Cut(arg, "=")
		switch {
		case name == "--target":
			if !hasVal {
				if i+1 >= len(args) {
					return a, errors.New("--target needs a value; " + skillUsageLine)
				}
				i++
				val = args[i]
			}
			a.target = val
		case name == "--global" && !hasVal:
			a.global = true
		case name == "--force" && !hasVal && sub == "install":
			a.force = true
		case strings.HasPrefix(arg, "-"):
			return a, fmt.Errorf("unknown flag %q; %s", arg, skillUsageLine)
		default:
			return a, fmt.Errorf("unexpected extra argument %q; %s", arg, skillUsageLine)
		}
	}
	if err := skillinstall.ValidateTarget(a.target, a.global); err != nil {
		return a, err
	}
	return a, nil
}

// skillCommand runs "taskfactory skill" and returns the exit code.
func skillCommand(args []string, common commonFlags, dry dryRun, stdout, stderr io.Writer) int {
	p := ui.For(os.Stderr)
	usageFail := func(msg string) int {
		fmt.Fprintln(stderr, errorLine(p, "taskfactory skill", msg))
		return exitUsage
	}
	if len(args) == 0 {
		return usageFail(skillUsageLine)
	}
	sub := args[0]
	if sub != "install" && sub != "status" {
		return usageFail(fmt.Sprintf("unknown subcommand %q; %s", sub, skillUsageLine))
	}
	a, err := parseSkillArgs(sub, args[1:])
	if err != nil {
		return usageFail(err.Error())
	}
	command := "skill-" + sub
	fail := func(err error) int {
		fmt.Fprintln(stderr, errorLine(p, "taskfactory skill "+sub, err.Error()))
		printNextOnFailure(stderr, p, command, "")
		return 1
	}
	dir, err := resolveSkillDir(a)
	if err != nil {
		return fail(err)
	}
	if sub == "status" {
		rep := skillinstall.Status(dir, skills.SkillMD)
		for _, c := range rep.Checked {
			fmt.Fprintf(stdout, "checked: %s\n", c)
		}
		switch rep.State {
		case skillinstall.Present:
			fmt.Fprintln(stdout, "present")
			return 0
		case skillinstall.Missing:
			fmt.Fprintln(stdout, "missing")
		default:
			fmt.Fprintf(stdout, "stale: installed %s, embedded %s\n", rep.Installed, rep.Embedded)
		}
		printNextOnFailure(stderr, p, command, "")
		return 1
	}
	opts := skillinstall.Options{
		Skill: skills.SkillMD, Version: version, Force: a.force, DryRun: common.DryRun,
		Verbose: verboseFunc(), Would: dry.would,
	}
	if err := skillinstall.Install(dir, opts); err != nil {
		return fail(err)
	}
	if common.DryRun {
		dry.done()
		return 0
	}
	fmt.Fprintf(stdout, "installed skill in %s\n", dir)
	printNextOnSuccess(stdout, ui.For(os.Stdout), command, "")
	return 0
}

func resolveSkillDir(a skillArgs) (string, error) {
	t := skillinstall.Target{Name: a.target, Global: a.global}
	if a.global {
		t.Home = os.Getenv("HOME")
	} else if !strings.HasPrefix(a.target, "dir:") {
		root, err := projectRoot()
		if err != nil {
			return "", skillinstall.ErrNoProject
		}
		t.Root = root
	}
	return skillinstall.ResolveDir(t)
}
