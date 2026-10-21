package main

import (
	"fmt"
	"io"
	"strings"

	"taskfactory/internal/ui"
)

// hint is the success and failure text of one command. An empty string means
// the command prints no hint for that outcome. Text is a command in plain
// words after "next: "; <ID> is replaced by the task ID when one is known.
type hint struct {
	success string
	failure string
}

// nextSteps is the single source of every next-step hint. The "work" entry is
// printed by internal/work on its own code path; a test pins that the text
// there reads identically. "requeue-inbox" is requeue --to inbox.
var nextSteps = map[string]hint{
	"status":        {"taskfactory validate", "run taskfactory init at the repo root"},
	"validate":      {"taskfactory promote <ID> or claim <ID>", "fix the files, rerun taskfactory validate"},
	"new":           {"edit the file, then taskfactory validate and taskfactory promote <ID>", "give a title with letters or digits, run inside the project"},
	"promote":       {"taskfactory claim <ID> --owner <name>", "fix tasks/inbox, rerun taskfactory promote <ID>"},
	"claim":         {"taskfactory work <ID> --adapter <name> --model <id>", "taskfactory status shows the task state"},
	"work":          {"taskfactory verify <ID>", ""},
	"verify":        {"taskfactory integrate <ID>", "fix the checks, rerun taskfactory verify <ID>"},
	"integrate":     {"taskfactory status", "rerun taskfactory verify <ID>, then integrate <ID>"},
	"check-main":    {"taskfactory integrate <ID>", "fix main, rerun taskfactory check-main"},
	"fail":          {"taskfactory requeue <ID> --to ready", "taskfactory status shows the task state"},
	"requeue":       {"taskfactory claim <ID> --owner <name>", "taskfactory status shows the task state"},
	"completion":    {"add it to your shell profile, see README", "taskfactory completion bash or taskfactory completion zsh"},
	"skill-install": {"taskfactory skill status", "rerun with --force to replace it, or choose --target"},
	"skill-status":  {"", "taskfactory skill install"},
	"requeue-inbox": {"taskfactory promote <ID>", "taskfactory status shows the task state"},
}

// initNextSteps are the lines of the block init prints. Step 2 names the target
// directory when init was given a path.
func initNextSteps(target string) []string {
	commit := "Commit .taskfactory/config.toml, .gitignore and tasks/ first."
	if target != "" {
		commit = "Commit .taskfactory/config.toml, .gitignore and tasks/ first, in " + target + "."
	}
	return []string{
		"Next steps:",
		"  1. Review .taskfactory/config.toml and set the verification commands.",
		"  2. " + commit,
		"  3. Write a task in tasks/inbox, then run: taskfactory promote <ID>",
		"  4. Run: taskfactory claim <ID> --owner <name>",
		"  5. Optional: install the agent skill with taskfactory skill install (check with " + nextSteps["skill-install"].success + ")",
	}
}

func renderHint(p ui.Painter, text, id string) string {
	if text == "" {
		return ""
	}
	if id != "" {
		text = strings.ReplaceAll(text, "<ID>", id)
	}
	return p.Dim("next:") + " " + text
}

// nextOnSuccess returns the success hint line for command, without a trailing
// newline, or "" when there is none. The command "init" returns the multi-line
// Next steps block; id is then the init target path, or "".
func nextOnSuccess(p ui.Painter, command, id string) string {
	if command == "init" {
		return strings.Join(initNextSteps(id), "\n")
	}
	return renderHint(p, nextSteps[command].success, id)
}

// nextOnFailure returns the failure hint line for command, or "" when there is
// none.
func nextOnFailure(p ui.Painter, command, id string) string {
	return renderHint(p, nextSteps[command].failure, id)
}

func printNextOnSuccess(w io.Writer, p ui.Painter, command, id string) {
	if hintsOff {
		return
	}
	if line := nextOnSuccess(p, command, id); line != "" {
		fmt.Fprintln(w, line)
	}
}

func printNextOnFailure(w io.Writer, p ui.Painter, command, id string) {
	if hintsOff {
		return
	}
	if line := nextOnFailure(p, command, id); line != "" {
		fmt.Fprintln(w, line)
	}
}
