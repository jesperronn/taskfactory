// Command taskfactory is the TaskFactory CLI. It coordinates software work into
// explicit, verifiable tasks that coding agents can execute safely in parallel.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"taskfactory/internal/claim"
	"taskfactory/internal/config"
	"taskfactory/internal/fail"
	"taskfactory/internal/integrate"
	"taskfactory/internal/promote"
	"taskfactory/internal/requeue"
	"taskfactory/internal/taskvalidate"
	"taskfactory/internal/ui"
	"taskfactory/internal/verify"
	"taskfactory/internal/work"
)

// version is printed by --version. It defaults to "dev" for plain go build or
// go run; bin/build overrides it at link time with -ldflags "-X main.version=...".
var version = "dev"

// commandSummary is one top-level command: its name, its usage synopsis and a
// short description shown in the top-level usage.
type commandSummary struct {
	name     string
	synopsis string
	summary  string
}

// commandSummaries lists every command in top-level usage order.
var commandSummaries = []commandSummary{
	{"init", "init [path]", "initialize project config"},
	{"status", "status", "show tasks by state"},
	{"validate", "validate [path...]", "check task files"},
	{"claim", "claim <ID> --owner <name>", "claim a ready task"},
	{"verify", "verify <ID>", "run a task's checks"},
	{"integrate", "integrate <ID>", "fast-forward a verified task"},
	{"check-main", "check-main", "recheck a stopped main"},
	{"promote", "promote <ID>", "move an inbox task to ready"},
	{"fail", "fail <ID> --outcome <FAILED|BLOCKED>", "move a claimed task to failed"},
	{"requeue", "requeue <ID> [--to ready|inbox] [--discard-claim]", "return a failed task to ready or inbox"},
	{"work", "work <ID> --adapter <omp|pi|claude> --model <id>", "launch a local worker on a claimed task"},
	{"completion", "completion <bash|zsh>", "print a shell completion script"},
	{"skill", "skill <install|status>", "install or check the embedded agent skill"},
}

// commandHelps maps each command in commandSummaries to its per-command help.
var commandHelps = map[string]string{
	"init":       initHelp,
	"status":     statusHelp,
	"validate":   validateHelp,
	"claim":      claimHelp,
	"verify":     verifyHelp,
	"integrate":  integrateHelp,
	"check-main": checkMainHelp,
	"promote":    promoteHelp,
	"fail":       failHelp,
	"requeue":    requeueHelp,
	"work":       workHelp,
	"completion": completionHelp,
	"skill":      skillHelp,
}

// styleArguments colors the words after a subcommand name: flags starting with
// "--" are cyan and everything else, such as <ID>, is dim. Words are joined with
// the original spaces so the plain text is unchanged.
func styleArguments(p ui.Painter, rest string) string {
	words := strings.Split(rest, " ")
	for i, word := range words {
		if strings.HasPrefix(word, "--") {
			words[i] = p.Cyan(word)
		} else {
			words[i] = p.Dim(word)
		}
	}
	return strings.Join(words, " ")
}

// styleHelp applies color to a per-command help constant: the subcommand name is
// yellow, flags are cyan, placeholders are dim and headings are bold. With color
// disabled the result is byte-identical to text.
func styleHelp(p ui.Painter, text string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(text, "\n") {
		body := strings.TrimSuffix(line, "\n")
		newline := line[len(body):]
		switch {
		case strings.HasPrefix(body, "Usage: taskfactory "):
			rest := strings.TrimPrefix(body, "Usage: taskfactory ")
			name := strings.Fields(rest)[0]
			b.WriteString(p.Bold("Usage:") + " " + p.Cyan("taskfactory") + " " + p.Yellow(name) + p.Dim(rest[len(name):]) + newline)
		case body == "Flags:" || body == "Exit codes:":
			b.WriteString(p.Bold(body) + newline)
		case strings.HasPrefix(body, "  --"):
			rest := body[2:]
			flag, tail := rest, ""
			if k := strings.Index(rest, "  "); k >= 0 {
				flag, tail = rest[:k], rest[k:]
			}
			styled := p.Cyan(flag)
			if i := strings.Index(flag, " <"); i >= 0 {
				styled = p.Cyan(flag[:i]) + p.Dim(flag[i:])
			}
			b.WriteString("  " + styled + tail + newline)
		default:
			b.WriteString(line)
		}
	}
	return b.String()
}

// wantsHelp reports whether args contain a help flag in any position.
func wantsHelp(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-h", "-help", "--h", "--help":
			return true
		}
	}
	return false
}

// usage returns the help text printed by --help and -h. It lists every command
// with a description aligned in one column, and documents only the global flags
// --help and --version. Styling is applied only when p is enabled, so plain
// output is unchanged when color is off.
func usage(p ui.Painter) string {
	synopsisWidth := 0
	for _, command := range commandSummaries {
		if len(command.synopsis) > synopsisWidth {
			synopsisWidth = len(command.synopsis)
		}
	}

	var b strings.Builder
	b.WriteString("taskfactory coordinates software work into explicit, verifiable tasks.\n\n")
	b.WriteString(p.Bold("Usage:") + "\n")
	b.WriteString("  " + p.Cyan("taskfactory") + " " + p.Dim("[global flags]") + "\n")
	b.WriteString("  " + p.Cyan("taskfactory") + " " + p.Dim("<command> [flags]") + "\n\n")
	b.WriteString(p.Bold("Commands:") + "\n")
	for _, command := range commandSummaries {
		rest := strings.TrimPrefix(command.synopsis, command.name)
		padding := strings.Repeat(" ", synopsisWidth-len(command.synopsis)+2)
		b.WriteString("  " + p.Yellow(command.name) + styleArguments(p, rest) + padding + command.summary + "\n")
	}
	b.WriteString("\n" + p.Bold("Global flags:") + "\n")
	b.WriteString("  " + p.Cyan("--help") + "     print this usage and exit successfully\n")
	b.WriteString("  " + p.Cyan("--version") + "  print the CLI version string and exit successfully\n\n")
	b.WriteString("Run \"taskfactory <command> --help\" for details on one command.\n")
	return b.String()
}

// errorLine formats a stderr error as "<prefix>: <msg>" with a red prefix when
// color is enabled for that stream.
func errorLine(p ui.Painter, prefix, msg string) string {
	return p.Red(prefix) + ": " + msg
}

// exitUsage is the exit code returned when global flags are used incorrectly or
// an unknown flag is supplied.
const exitUsage = 2

const defaultConfig = `protocol_version = 1

[workers]
max_parallel = 4

[git]
use_worktrees = true
worktree_root = ".taskfactory/worktrees"
integration_strategy = "ff-only"

[integration]
stop_on_main_failure = true

[verification]
worker = ["go test ./..."]
integration = ["go test ./...", "go vet ./..."]
`

var taskStates = []string{"inbox", "ready", "active", "failed", "archive"}

// verifyHelp is printed by "verify --help" and "verify -h".
const verifyHelp = `Usage: taskfactory verify <ID>

Run the success-criteria checks of the claimed task <ID> and record each outcome
in .taskfactory/evidence/<ID>.jsonl. After the attempt is recorded it prints one
summary line on stdout, such as "verify TF-001: PASS (3 checks)" or
"verify TF-001: FAIL (1 of 3 checks failed)". The command exits 1 when any
check fails.

With --dry-run nothing changes.

Next step: taskfactory integrate <ID> after a passing verify.

Flags:
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h  print this help and exit successfully

Exit codes:
  0  every check passed
  1  a check failed or verification could not run
  2  invalid usage
`

// integrateHelp is printed by "integrate --help" and "integrate -h".
const integrateHelp = `Usage: taskfactory integrate <ID>

Fast-forward main to the verified task <ID> using the configured integration
strategy, after running the integration verification commands. On success it
prints "integrated <ID>". A failed check or fast-forward stops integration and
is reported on stderr.

With --dry-run nothing changes.

Next step: taskfactory status after a successful integrate.

Flags:
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h  print this help and exit successfully

Exit codes:
  0  the task was integrated
  1  integration failed
  2  invalid usage
`

// checkMainHelp is printed by "check-main --help" and "check-main -h".
const checkMainHelp = `Usage: taskfactory check-main

Recheck main after integration has stopped it by running the integration
verification commands again. It prints "check-main: ok" when they pass.

With --dry-run nothing changes.

Next step: taskfactory integrate <ID> once main passes.

Flags:
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h  print this help and exit successfully

Exit codes:
  0  main passes the integration checks
  1  a check failed or main could not be checked
  2  invalid usage
`

// promoteHelp is printed by "promote --help" and "promote -h".
const promoteHelp = `Usage: taskfactory promote <ID>

Promote the inbox task <ID> to tasks/ready when its complete executable contract
validates. The task file moves unchanged from tasks/inbox to tasks/ready, the
inbox, ready, active and failed task files are validated, and only the two task
paths are committed. Any refusal or failure restores the file to tasks/inbox.

The commit runs as a plain git commit and follows your own Git signing
configuration. TaskFactory never disables or overrides signing.

With --dry-run nothing changes.

Next step: taskfactory claim <ID> --owner <name> after a promote.

Flags:
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h  print this help and exit successfully

Exit codes:
  0  the task was promoted and committed
  1  promotion was refused or failed; the file was restored
  2  invalid usage
`

// failHelp is printed by "fail --help" and "fail -h".
const failHelp = `Usage: taskfactory fail <ID> --outcome <FAILED|BLOCKED>

Move the claimed active task <ID> to tasks/failed. The last attempt evidence
record must have the same outcome. Without evidence, --reason is required and
its text becomes the commit message body. The task file moves unchanged, the
inbox, ready, active and failed task files are validated, and only the two task
paths are committed. Any refusal or failure restores the file to tasks/active.
The command never archives a task and never changes the evidence file.

The commit runs as a plain git commit and follows your own Git signing
configuration. TaskFactory never disables or overrides signing.

With --dry-run nothing changes.

Next step: taskfactory requeue <ID> --to ready after a fail.

Flags:
  --outcome <FAILED|BLOCKED>  outcome that must match the last evidence record
  --reason <text>             single-line reason, only when no evidence exists
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h                  print this help and exit successfully

Exit codes:
  0  the task was moved and committed
  1  the command was refused or failed; the file was restored
  2  invalid usage
`

// requeueHelp is printed by "requeue --help" and "requeue -h". Inside a project
// the claimed failed task IDs are appended.
const requeueHelp = `Usage: taskfactory requeue <ID> [--to ready|inbox] [--discard-claim]

Return the failed task <ID> to tasks/ready (the default) or tasks/inbox. The task
file moves unchanged from tasks/failed. For ready, the complete executable
contract must validate; for inbox no contract check applies. The task tree is
validated and only the two task paths are committed. Any refusal or failure
restores the file to tasks/failed.

A failed task that still has a Claim block is never requeued automatically: it
is flagged for a human decision and the command refuses, listing every claimed
failed task. Make that decision with --discard-claim: it removes the whole
Claim block from the task file and then proceeds as for an unclaimed task. The
commit message body records the discarded claim (owner, branch, base commit and
worktree path) so nothing is lost. On a task without a Claim block the flag is
accepted and nothing is discarded.

After the requeue commit, requeue removes the task's old worktree and branch so
the task can be claimed again: the ones recorded in the Claim block, or the
conventional .taskfactory/worktrees/<ID> and task/<ID> when there is no Claim
block. It does so only when that is safe: the worktree is clean (no changes, no
untracked files), the branch has no commits outside local main, and the
worktree is inside .taskfactory/worktrees and is not the main checkout. These
checks run first; if one fails requeue exits 1 and changes nothing, and prints
the manual commands: git worktree remove <worktree path>, then git branch -d
<branch> (or git branch -D <branch> if you decide to discard the commits). The
cleanup never forces anything and never uses -D. Unmerged worker commits always
need your decision; failed tasks usually carry worker commits, so the refusal is
the common case and the manual commands remain the way. A missing worktree or
branch is not an error. If the cleanup fails after the commit, requeue exits 1
and says the task was requeued and what was left behind.

The attempt counter is reset by renaming
.taskfactory/evidence/<ID>.jsonl to .taskfactory/evidence/<ID>.attempts-<N>.jsonl,
where N is its highest attempt number, so the next attempt starts at 1. The
evidence bytes are never edited or deleted, the rename never overwrites an
existing file, and the renamed file is never committed. The output and the commit
message body name the new path.

The commit runs as a plain git commit and follows your own Git signing
configuration. TaskFactory never disables or overrides signing.

With --dry-run nothing changes.

Next step: taskfactory claim <ID> --owner <name> after a requeue to ready.

Flags:
  --to <ready|inbox>  target state, ready by default
  --discard-claim     remove the Claim block of a claimed failed task first
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h          print this help and exit successfully

Exit codes:
  0  the task was moved and committed
  1  the command was refused or failed; the file was restored, or the task was
     requeued but removing its old worktree or branch failed (stated in the error)
  2  invalid usage
`

// workHelp is printed by "work --help" and "work -h".
const workHelp = `Usage: taskfactory work <ID> --adapter <omp|pi|claude> --model <id>

Launch the chosen local worker adapter in the worktree of the claimed active
task <ID>. The adapter, the model and, for claude, the haiku model are always
named explicitly: there is no default and no fallback. The adapter preflight
runs first; a refusal prints each failed check and changes nothing. The prompt
is built from the task file and the worker instructions. The adapter output is
copied to .taskfactory/logs/<ID>/<adapter>-<time>.log when the run ends, and the
log path is printed first. Only one adapter may be given.

work never commits, stages, verifies, integrates or fails a task. A worker exit
of 0 is a claim only: run "taskfactory verify <ID>" next. A blocked or stalled
run prints a "taskfactory fail <ID> --outcome BLOCKED --reason ..." suggestion
that you may run yourself; work does not run it.

With --dry-run nothing changes.

Next step: taskfactory verify <ID> after the worker exits.

Flags:
  --adapter <omp|pi|claude>  worker harness (required, exactly once)
  --model <id>               explicit model id, no whitespace (required)
  --haiku-model <id>         haiku-tier model id (required for claude only)
  --timeout <duration>       wall-clock bound, at least 1s (default 10m)
  --endpoint <host:port>     oMLX server address (default 127.0.0.1:8000)
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h                 print this help and exit successfully

Exit codes:
  0  the worker exited 0 (verify next)
  1  refused, preflight failed, blocked, stalled or the worker exited non-zero
  2  invalid usage
`

func workTask(args []string, stderrStyle ui.Painter, common commonFlags, dry dryRun) int {
	id, opts, err := work.ParseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory work", err.Error()))
		return exitUsage
	}
	opts.DryRun, opts.Verbose, opts.Would = common.DryRun, verboseFunc(), dry.would
	root, err := projectRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory work", err.Error()))
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	code, err := work.Execute(ctx, root, id, opts, work.Deps{})
	if err != nil {
		fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory work", err.Error()))
	}
	if common.DryRun && err == nil && code == 0 {
		dry.done()
	}
	return code
}

func failTask(args []string, common commonFlags, dry dryRun) error {
	id, opts, err := fail.ParseArgs(args)
	if err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	failed, err := fail.FailWith(root, id, opts, fail.CommonOptions{DryRun: common.DryRun, Verbose: verboseFunc(), Would: dry.would})
	if err != nil {
		return err
	}
	if common.DryRun {
		dry.done()
		return nil
	}
	fmt.Fprintf(os.Stdout, "failed %s to %s\n", id, failed)
	printNextOnSuccess(os.Stdout, ui.For(os.Stdout), "fail", id)
	return nil
}

func requeueTask(args []string, common commonFlags, dry dryRun) error {
	id, target, opts, err := requeue.ParseOptions(args)
	if err != nil {
		return err
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	opts.DryRun, opts.Verbose, opts.Would = common.DryRun, verboseFunc(), dry.would
	res, err := requeue.RequeueWith(root, id, target, opts)
	if err == nil && common.DryRun {
		dry.done()
		return nil
	}
	if err != nil {
		var cleanup requeue.CleanupError
		if errors.As(err, &cleanup) {
			printRequeued(id, opts, res)
		}
		return err
	}
	printRequeued(id, opts, res)
	command := "requeue"
	if target == "inbox" {
		command = "requeue-inbox"
	}
	printNextOnSuccess(os.Stdout, ui.For(os.Stdout), command, id)
	return nil
}

// printRequeued reports a successful requeue, including what the safe cleanup
// removed. It is also used when the cleanup failed after the commit.
func printRequeued(id string, opts requeue.Options, res requeue.Result) {
	fmt.Fprintf(os.Stdout, "requeued %s to %s\n", id, res.Path)
	if opts.DiscardClaim {
		if res.Discarded == "" {
			fmt.Fprintf(os.Stdout, "%s had no Claim block; nothing was discarded\n", id)
		} else {
			fmt.Fprintf(os.Stdout, "discarded the Claim block (recorded in the commit message)\n")
		}
	}
	if res.Aside != "" {
		fmt.Fprintf(os.Stdout, "attempt counter reset: the %d earlier attempt(s) moved to %s; the next attempt starts at 1\n", res.Attempts, res.Aside)
	}
	if res.RemovedWorktree != "" {
		fmt.Fprintf(os.Stdout, "removed worktree %s\n", res.RemovedWorktree)
	}
	if res.RemovedBranch != "" {
		fmt.Fprintf(os.Stdout, "removed branch %s\n", res.RemovedBranch)
	}
	if res.RemovedWorktree == "" && res.RemovedBranch == "" {
		fmt.Fprintf(os.Stdout, "no old worktree or branch to remove\n")
	}
}

// requeueHelpText returns requeueHelp. Inside a project it appends the claimed
// failed task IDs; outside a project it is the static text only.
func requeueHelpText() string {
	root, err := projectRoot()
	if err != nil {
		return requeueHelp
	}
	ids, err := requeue.ClaimedFailed(root)
	if err != nil {
		return requeueHelp
	}
	if len(ids) == 0 {
		return requeueHelp + "\nClaimed failed tasks in this project: none.\n"
	}
	return requeueHelp + "\nClaimed failed tasks in this project (human decision needed): " + strings.Join(ids, ", ") + "\n"
}

func promoteTask(args []string, common commonFlags, dry dryRun) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	ready, err := promote.PromoteWith(root, args[0], promote.Options{DryRun: common.DryRun, Verbose: verboseFunc(), Would: dry.would})
	if err != nil {
		return err
	}
	if common.DryRun {
		dry.done()
		return nil
	}
	fmt.Fprintf(os.Stdout, "promoted %s to %s\n", args[0], ready)
	printNextOnSuccess(os.Stdout, ui.For(os.Stdout), "promote", args[0])
	return nil
}

func main() {
	fs := flag.NewFlagSet("taskfactory", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	// Keep flag.Parse from printing the full usage text on invalid input.
	fs.Usage = func() {}
	// Color is decided per stream: stdout for help, stderr for errors.
	stdoutStyle := ui.For(os.Stdout)
	stderrStyle := ui.For(os.Stderr)

	help := fs.Bool("help", false, "print this usage and exit successfully")
	showVersion := fs.Bool("version", false, "print the CLI version string and exit successfully")

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory", err.Error()))
		os.Exit(exitUsage)
	}

	if *help {
		fmt.Fprint(os.Stdout, usage(stdoutStyle))
		return
	}

	if *showVersion {
		fmt.Fprintln(os.Stdout, "taskfactory version "+version)
		return
	}

	args := fs.Args()
	var common commonFlags
	if len(args) > 0 && args[0] == "__complete" {
		os.Exit(completeHelper(args[1:], os.Stdout))
	}
	if len(args) > 0 && wantsHelp(args[1:]) {
		if args[0] == "skill" {
			fmt.Fprint(os.Stdout, styleHelp(stdoutStyle, skillHelpFor(args[1:])))
			return
		}
		if args[0] == "requeue" {
			fmt.Fprint(os.Stdout, styleHelp(stdoutStyle, requeueHelpText()))
			return
		}
		if text, ok := commandHelps[args[0]]; ok {
			fmt.Fprint(os.Stdout, styleHelp(stdoutStyle, text))
			return
		}
	}
	if len(args) > 0 {
		if _, known := commandHelps[args[0]]; known && args[0] != "completion" {
			rest, flags, err := extractCommonFlags(args[1:])
			if err != nil {
				fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory "+args[0], err.Error()))
				os.Exit(exitUsage)
			}
			args = append([]string{args[0]}, rest...)
			common = flags
			verboseOn = flags.Verbose
			hintsOff = flags.DryRun
		}
	}
	dry := dryRun{out: os.Stdout}
	if len(args) >= 1 && args[0] == "completion" {
		os.Exit(completionCommand(args[1:], os.Stdout, os.Stderr, stdoutIsTerminal()))
	}
	if len(args) >= 1 && args[0] == "skill" {
		os.Exit(skillCommand(args[1:], common, dry, os.Stdout, os.Stderr))
	}
	if len(args) >= 1 && args[0] == "init" {
		if len(args) > 2 {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory init", fmt.Sprintf("unexpected extra argument %q; usage: taskfactory init [path]", args[2])))
			os.Exit(exitUsage)
		}
		target := ""
		if len(args) == 2 {
			target = args[1]
			if strings.HasPrefix(target, "-") {
				fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory init", fmt.Sprintf("unknown flag %q; usage: taskfactory init [path]", target)))
				os.Exit(exitUsage)
			}
		}
		if err := initializeProject(target, common.DryRun, dry); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory init", err.Error()))
			os.Exit(1)
		}
		if common.DryRun {
			dry.done()
			return
		}
		printNextOnSuccess(os.Stdout, stdoutStyle, "init", target)
		return
	}
	if len(args) >= 1 && args[0] == "validate" {
		if err := validateProject(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory validate", err.Error()))
			var usage usageError
			if errors.As(err, &usage) {
				os.Exit(exitUsage)
			}
			printNextOnFailure(os.Stderr, stderrStyle, "validate", "")
			os.Exit(1)
		}
		if common.DryRun {
			dry.done()
		}
		return
	}
	if len(args) == 1 && args[0] == "status" {
		if err := statusProject(); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory status", err.Error()))
			printNextOnFailure(os.Stderr, stderrStyle, "status", "")
			os.Exit(1)
		}
		if common.DryRun {
			dry.done()
		}
		return
	}
	if len(args) > 0 && args[0] == "claim" {
		if err := claimTask(args[1:], common, dry); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory claim", err.Error()))
			printNextOnFailure(os.Stderr, stderrStyle, "claim", "")
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "verify" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory verify", "usage: taskfactory verify <ID>"))
			os.Exit(exitUsage)
		}
		if looksLikePath(args[1]) {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory verify", "usage: taskfactory verify <ID>; did you mean `taskfactory validate <path>`?"))
			os.Exit(exitUsage)
		}
		root, err := projectRoot()
		var result verify.Result
		if err == nil {
			result, err = verify.VerifyWith(root, args[1], verify.Options{DryRun: common.DryRun, Verbose: verboseFunc(), Would: dry.would})
		}
		if result.Outcome != "" {
			fmt.Fprintln(os.Stdout, result.Summary(stdoutStyle))
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory verify", err.Error()))
			printNextOnFailure(os.Stderr, stderrStyle, "verify", args[1])
			os.Exit(1)
		}
		if common.DryRun {
			dry.done()
			return
		}
		printNextOnSuccess(os.Stdout, stdoutStyle, "verify", args[1])
		return
	}
	if len(args) > 0 && args[0] == "integrate" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory integrate", "usage: taskfactory integrate <ID>"))
			os.Exit(exitUsage)
		}
		root, err := projectRoot()
		if err == nil {
			err = integrate.RunWith(root, args[1], integrate.Options{DryRun: common.DryRun, Verbose: verboseFunc(), Would: dry.would})
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory integrate", err.Error()))
			printNextOnFailure(os.Stderr, stderrStyle, "integrate", args[1])
			os.Exit(1)
		}
		if common.DryRun {
			dry.done()
			return
		}
		fmt.Fprintf(os.Stdout, "integrated %s\n", args[1])
		printNextOnSuccess(os.Stdout, stdoutStyle, "integrate", args[1])
		return
	}
	if len(args) > 0 && args[0] == "promote" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory promote", "usage: taskfactory promote <ID>"))
			os.Exit(exitUsage)
		}
		if err := promoteTask(args[1:], common, dry); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory promote", err.Error()))
			var usage promote.UsageError
			if errors.As(err, &usage) {
				os.Exit(exitUsage)
			}
			printNextOnFailure(os.Stderr, stderrStyle, "promote", args[1])
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "fail" {
		if err := failTask(args[1:], common, dry); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory fail", err.Error()))
			var usage fail.UsageError
			if errors.As(err, &usage) {
				os.Exit(exitUsage)
			}
			printNextOnFailure(os.Stderr, stderrStyle, "fail", "")
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "requeue" {
		if err := requeueTask(args[1:], common, dry); err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory requeue", err.Error()))
			var usage requeue.UsageError
			if errors.As(err, &usage) {
				os.Exit(exitUsage)
			}
			printNextOnFailure(os.Stderr, stderrStyle, "requeue", "")
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "work" {
		code := workTask(args[1:], stderrStyle, common, dry)
		os.Exit(code)
	}
	if len(args) > 0 && args[0] == "check-main" {
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory check-main", "usage: taskfactory check-main"))
			os.Exit(exitUsage)
		}
		root, err := projectRoot()
		if err == nil {
			err = integrate.CheckMainWith(root, integrate.Options{DryRun: common.DryRun, Verbose: verboseFunc(), Would: dry.would})
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, errorLine(stderrStyle, "taskfactory check-main", err.Error()))
			printNextOnFailure(os.Stderr, stderrStyle, "check-main", "")
			os.Exit(1)
		}
		if common.DryRun {
			dry.done()
			return
		}
		fmt.Fprintln(os.Stdout, "check-main: ok")
		printNextOnSuccess(os.Stdout, stdoutStyle, "check-main", "")
		return
	}

	fmt.Fprint(os.Stderr, usage(stderrStyle))
	os.Exit(exitUsage)
}

// claimHelp is printed by "claim --help" and "claim -h".
const claimHelp = `Usage: taskfactory claim <ID> --owner <name>

Claim the ready task <ID> for the named worker. The task file moves from
tasks/ready to tasks/active and records the claim details, including the owner.
The owner must be non-empty.

With --dry-run nothing changes.

Next step: taskfactory work <ID> --adapter <name> --model <id> after a claim.

Flags:
  --owner <name>  worker identifier (required)
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h      print this help and exit successfully

Exit codes:
  0  the task was claimed
  1  the claim was refused or the arguments were invalid
`

func claimTask(args []string, common commonFlags, dry dryRun) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: taskfactory claim <ID> --owner <name>")
	}
	id := args[0]
	fs := flag.NewFlagSet("claim", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	owner := fs.String("owner", "", "worker identifier")
	if err := fs.Parse(args[1:]); err != nil {
		return fmt.Errorf("usage: taskfactory claim <ID> --owner <name>: %w", err)
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: taskfactory claim <ID> --owner <name>")
	}
	if strings.TrimSpace(*owner) == "" {
		return fmt.Errorf("--owner must be non-empty")
	}
	root, err := projectRoot()
	if err != nil {
		return err
	}
	if err := claim.ClaimWith(root, id, *owner, claim.Options{DryRun: common.DryRun, Verbose: verboseFunc(), Would: dry.would}); err != nil {
		return err
	}
	if common.DryRun {
		dry.done()
		return nil
	}
	fmt.Fprintf(os.Stdout, "claimed %s for %s\n", id, *owner)
	printNextOnSuccess(os.Stdout, ui.For(os.Stdout), "claim", id)
	return nil
}

// validateHelp is printed by "validate --help" and "validate -h".
const validateHelp = `Usage: taskfactory validate [path...]

Validate task files. Inbox files are checked loosely, because inbox proposals
are intentionally rough: "taskfactory promote" applies the full ready contract.
In a ready task, Dependencies is None or lines of "- TF-NNN" and nothing else.
With no arguments, validate every task file in tasks/inbox
and tasks/ready. Each path is a task file or a folder inside the project's tasks
directory; a folder selects the task files directly inside it. "tasks" selects
every state except the archive: inbox, ready, active and failed. Relative paths
are resolved from the project root. Task ID uniqueness and dependency references
are checked across the whole tree, including tasks/archive, but only diagnostics
for the selected files are reported. Archived files are not contract-checked
unless named, but an archived file whose task ID cannot be read, or that is
empty or not UTF-8, is always reported as an archive read error.

--dry-run is accepted and has no effect, because this command only reads.

Next step: taskfactory promote <ID> or claim <ID> after a clean validate.

Flags:
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h  print this help and exit successfully

Exit codes:
  0  every selected task file is valid
  1  a selected task file is invalid or the project configuration is invalid
  2  invalid usage, such as a path outside the tasks directory
`

// usageError marks invalid validate arguments, which exit with exitUsage.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func validateProject(args []string) error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	if _, err := config.Load(root); err != nil {
		return err
	}
	files, err := selectValidateFiles(root, args)
	if err != nil {
		return err
	}
	diagnostics := taskvalidate.ValidateFiles(root, files)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.Error())
	}
	if len(diagnostics) > 0 {
		return fmt.Errorf("%d validation error(s)", len(diagnostics))
	}
	style := ui.For(os.Stdout)
	if len(args) == 0 {
		fmt.Fprintln(os.Stdout, style.Green("inbox and ready tasks are valid"))
	} else {
		fmt.Fprintln(os.Stdout, style.Green(fmt.Sprintf("%d task file(s) valid", len(files))))
		if len(args) == 1 && len(files) == 1 && strings.HasPrefix(filepath.ToSlash(files[0]), "tasks/inbox/") {
			if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(files[0]))); err == nil && info.Mode().IsRegular() && !isDirArg(root, args[0]) {
				fmt.Fprintln(os.Stdout, "note: inbox files are checked loosely; promote applies the full ready contract")
			}
		}
	}
	printNextOnSuccess(os.Stdout, style, "validate", "")
	return nil
}

// selectValidateFiles resolves validate arguments to slash-separated task file
// paths relative to root. No arguments selects inbox and ready.
func selectValidateFiles(root string, args []string) ([]string, error) {
	if len(args) == 0 {
		var files []string
		for _, state := range []string{"inbox", "ready"} {
			listed, err := listDirectoryFiles(root, filepath.Join("tasks", state))
			if err != nil {
				return nil, err
			}
			files = append(files, listed...)
		}
		return files, nil
	}
	seen := map[string]bool{}
	var files []string
	for _, arg := range args {
		listed, err := resolveValidateArg(root, arg)
		if err != nil {
			return nil, err
		}
		for _, file := range listed {
			if !seen[file] {
				seen[file] = true
				files = append(files, file)
			}
		}
	}
	return files, nil
}

// resolveValidateArg maps one validate argument to the task files it selects.
func resolveValidateArg(root, arg string) ([]string, error) {
	outside := usageError{"task path must resolve inside the project tasks directory"}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	path := arg
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, usageError{fmt.Sprintf("resolve task path: %v", err)}
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if !insideTasks(root, path) && !insideTasks(resolvedRoot, path) {
			return nil, outside
		}
		return nil, usageError{fmt.Sprintf("task path %s cannot be read", arg)}
	}
	path = resolved
	rel, err := filepath.Rel(resolvedRoot, path)
	if err != nil || !insideTasks(resolvedRoot, path) {
		return nil, outside
	}
	rel = filepath.ToSlash(rel)
	info, err := os.Stat(path)
	if err != nil {
		return nil, usageError{fmt.Sprintf("task path %s cannot be read", arg)}
	}
	if info.IsDir() {
		if rel == "tasks" {
			// The archive is read for IDs and dependencies but is not a
			// contract-checked selection; name tasks/archive to check it.
			files, err := listTreeFiles(resolvedRoot, rel)
			if err != nil {
				return nil, err
			}
			var kept []string
			for _, file := range files {
				if !strings.HasPrefix(file, "tasks/archive/") {
					kept = append(kept, file)
				}
			}
			return kept, nil
		}
		return listDirectoryFiles(resolvedRoot, rel)
	}
	if !info.Mode().IsRegular() || !strings.HasSuffix(rel, ".md") {
		return nil, usageError{fmt.Sprintf("%s is not a task file", rel)}
	}
	return []string{rel}, nil
}

// insideTasks reports whether path is the tasks directory under root or below it.
func insideTasks(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel == "tasks" || strings.HasPrefix(rel, "tasks/")
}

// listDirectoryFiles returns the regular files directly inside the directory
// rel, a slash-separated path relative to root. A missing directory has none.
func listDirectoryFiles(root, rel string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, rel+"/"+entry.Name())
	}
	return files, nil
}

// listTreeFiles returns every regular file below the directory rel.
func listTreeFiles(root, rel string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(rel)), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			child, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(child))
		}
		return nil
	})
	return files, err
}

// looksLikePath reports whether a verify argument is a file path rather than a
// task ID, so the user can be pointed at validate.
func looksLikePath(arg string) bool {
	return strings.Contains(arg, "/") || strings.HasSuffix(arg, ".md")
}

// statusHelp is printed by "status --help" and "status -h".
const statusHelp = `Usage: taskfactory status

Print the number of task files in each state: inbox, ready, active, failed and
archive. The inbox, ready, active and failed task files are validated first,
while the archive is read for task IDs and dependencies only; any diagnostics
are printed to stderr and no task file is changed.

--dry-run is accepted and has no effect, because this command only reads.

Next step: taskfactory validate after the counts are printed.

Flags:
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h  print this help and exit successfully

Exit codes:
  0  the counts were printed
  1  the configuration, a state directory or a task file is invalid
  2  invalid usage
`

func statusProject() error {
	root, err := projectRoot()
	if err != nil {
		return err
	}
	if _, err := config.Load(root); err != nil {
		return err
	}
	for _, state := range taskStates {
		dir := filepath.Join(root, "tasks", state)
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("task state directory %s: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("task state path %s is not a directory", dir)
		}
	}
	diagnostics := taskvalidate.Validate(root, "")
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.Error())
	}
	if len(diagnostics) > 0 {
		return fmt.Errorf("%d validation error(s)", len(diagnostics))
	}
	counts := make(map[string]int, len(taskStates))
	for _, state := range taskStates {
		entries, err := os.ReadDir(filepath.Join(root, "tasks", state))
		if err != nil {
			return fmt.Errorf("read task state directory %s: %w", filepath.Join(root, "tasks", state), err)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == ".gitkeep" {
				continue
			}
			counts[state]++
		}
	}
	for _, state := range taskStates {
		fmt.Fprintf(os.Stdout, "%s: %d\n", state, counts[state])
	}
	printNextOnSuccess(os.Stdout, ui.For(os.Stdout), "status", "")
	return nil
}

func projectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determine current directory: %w", err)
	}
	verbosef("git -C %s rev-parse --path-format=absolute --show-toplevel", cwd)
	command := exec.Command("git", "-C", cwd, "rev-parse", "--path-format=absolute", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("current directory is not inside a Git working tree")
	}
	rootValue := filepath.Clean(strings.TrimSuffix(string(output), "\n"))
	root, err := filepath.Abs(rootValue)
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		return "", fmt.Errorf("resolve Git project root: %w", err)
	}
	verbosef("project root %s", root)
	return root, nil
}

// initHelp is printed by "init --help" and "init -h".
const initHelp = `Usage: taskfactory init [path]

Create the project configuration and the task state directories in the Git
repository that contains path, which may be any directory inside the project
(relative or absolute; a symlink is followed). Without path, the current
directory is used. An existing valid configuration
is kept unchanged, and missing task state directories are restored. It also
creates .gitignore, or appends to it, with the TaskFactory runtime paths
(locks, evidence, logs and worktrees) that are not already listed; other lines
are left alone and .taskfactory/config.toml is never ignored. Commit the
configuration and .gitignore together before the first claim.

With --dry-run nothing changes.

Next step: the Next steps block that init prints leads to promote and claim.

Arguments:
  path        a directory inside the Git working tree to initialize
              (default: the current directory)

Flags:
  --verbose   write detail lines starting "verbose: " to stderr
  --dry-run   print what would change as "would: " lines and change nothing
  --help, -h  print this help and exit successfully

Exit codes:
  0  the project is initialized
  1  initialization failed
  2  invalid usage
`

// initDryRun is the plan of an init dry run: every would line goes to out, and
// nothing is created. A nil *initDryRun means a real run.
type initDryRun struct {
	out  dryRun
	root string
	seen map[string]bool
}

// wouldCreate reports one action once, naming the path relative to the root.
func (p *initDryRun) wouldCreate(path string) {
	rel, err := filepath.Rel(p.root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	if p.seen[rel] {
		return
	}
	p.seen[rel] = true
	p.out.would("create %s", rel)
}

func initializeProject(target string, dryRunOn bool, out dryRun) error {
	dir := target
	if target == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("determine current directory: %w", err)
		}
		dir = cwd
	} else {
		info, err := os.Stat(target)
		if err != nil {
			return fmt.Errorf("path %s does not exist or cannot be read", target)
		}
		if !info.IsDir() {
			return fmt.Errorf("path %s is not a directory", target)
		}
	}
	verbosef("git -C %s rev-parse --path-format=absolute --show-toplevel", dir)
	command := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		if target == "" {
			return fmt.Errorf("current directory is not inside a Git working tree")
		}
		return fmt.Errorf("path %s is not inside a Git working tree", target)
	}
	root := filepath.Clean(strings.TrimSuffix(string(output), "\n"))
	root, _ = filepath.Abs(root)
	if root == "" {
		return fmt.Errorf("Git returned an empty project root")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve Git project root: %w", err)
	}
	verbosef("project root %s", root)
	var plan *initDryRun
	if dryRunOn {
		plan = &initDryRun{out: out, root: root, seen: map[string]bool{}}
	}
	return initializeAt(root, plan)
}

func initializeAt(root string, plan *initDryRun) error {
	if err := initializeConfigAt(root, plan); err != nil {
		return err
	}
	return ensureGitignore(root, plan)
}

// gitignoreComment and runtimeIgnorePaths are what init keeps in the project
// .gitignore. config.toml is deliberately absent: it must stay trackable.
const gitignoreComment = "# TaskFactory runtime files"

var runtimeIgnorePaths = []string{
	".taskfactory/claim.lock",
	".taskfactory/integration.lock",
	".taskfactory/evidence/",
	".taskfactory/integration-evidence/",
	".taskfactory/logs/",
	".taskfactory/worktrees/",
}

// ensureGitignore appends the missing runtime paths to the project .gitignore,
// creating it when absent. Existing lines are never changed or reordered, and a
// rerun with every path present writes nothing.
func ensureGitignore(root string, plan *initDryRun) error {
	path := filepath.Join(root, ".gitignore")
	var existing []byte
	verbosef("read %s", path)
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file; refusing to modify it", path)
		}
		existing, err = os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
	case !os.IsNotExist(err):
		return fmt.Errorf("inspect %s: %w", path, err)
	}
	present := map[string]bool{}
	for _, line := range strings.Split(string(existing), "\n") {
		present[strings.TrimSpace(line)] = true
	}
	var missing []string
	for _, p := range runtimeIgnorePaths {
		if !present[p] {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	var add strings.Builder
	if len(existing) > 0 {
		if existing[len(existing)-1] != '\n' {
			add.WriteString("\n")
		}
		if !present[gitignoreComment] {
			add.WriteString("\n")
		}
	}
	if !present[gitignoreComment] {
		add.WriteString(gitignoreComment + "\n")
	}
	for _, p := range missing {
		add.WriteString(p + "\n")
	}
	if plan != nil {
		if len(existing) == 0 && info == nil {
			plan.out.would("create .gitignore")
		}
		for _, p := range missing {
			plan.out.would("create ignore line %s in .gitignore", p)
		}
		return nil
	}
	verbosef("write %s", path)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := file.WriteString(add.String()); err != nil {
		_ = file.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

func initializeConfigAt(root string, plan *initDryRun) error {
	configPath := filepath.Join(root, ".taskfactory", "config.toml")
	if _, err := os.Stat(configPath); err == nil {
		if _, err := config.Load(root); err != nil {
			return fmt.Errorf("existing configuration conflicts with protocol v1: %w", err)
		}
		return createTaskStateDirectories(root, plan)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect configuration file %s: %w", configPath, err)
	}

	if err := ensureProjectDirectory(root, filepath.Dir(configPath), plan); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	if err := createTaskStateDirectories(root, plan); err != nil {
		return err
	}
	if plan != nil {
		plan.wouldCreate(configPath)
		return nil
	}
	verbosef("write %s", configPath)
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			if _, loadErr := config.Load(root); loadErr == nil {
				return createTaskStateDirectories(root, nil)
			} else {
				return fmt.Errorf("existing configuration conflicts with protocol v1: %w", loadErr)
			}
		}
		return fmt.Errorf("create configuration file %s: %w", configPath, err)
	}
	if _, err := file.WriteString(defaultConfig); err != nil {
		_ = file.Close()
		_ = os.Remove(configPath)
		return fmt.Errorf("write configuration file %s: %w", configPath, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(configPath)
		return fmt.Errorf("close configuration file %s: %w", configPath, err)
	}
	return nil
}

func createTaskStateDirectories(root string, plan *initDryRun) error {
	tasksDir := filepath.Join(root, "tasks")
	if err := ensureProjectDirectory(root, tasksDir, plan); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}
	for _, state := range taskStates {
		path := filepath.Join(tasksDir, state)
		if err := ensureProjectDirectory(root, path, plan); err != nil {
			return fmt.Errorf("create task state directory %s: %w", path, err)
		}
	}
	return nil
}

func ensureProjectDirectory(root, path string, plan *initDryRun) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path %s is outside Git project %s", path, root)
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%s is a symlink; refusing to write outside the Git project", current)
			}
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", current)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect %s: %w", current, err)
		}
		if plan != nil {
			plan.wouldCreate(current)
			continue
		}
		verbosef("create directory %s", current)
		if err := os.Mkdir(current, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", current, err)
		}
	}
	return nil
}

// isDirArg reports whether a validate argument names a directory.
func isDirArg(root, arg string) bool {
	path := arg
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
