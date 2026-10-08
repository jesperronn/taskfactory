// Package taskvalidate checks task files against the on-disk v1 contract.
package taskvalidate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"taskfactory/internal/config"
)

var (
	filePattern       = regexp.MustCompile(`^(TF-[0-9]{3})-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$`)
	idPattern         = regexp.MustCompile(`^TF-[0-9]{3}$`)
	criterionPattern  = regexp.MustCompile(`^### C([1-9][0-9]*): (.+)$`)
	htmlPattern       = regexp.MustCompile(`<\s*/?\s*[A-Za-z!][^>]*>`)
	inlineCodePattern = regexp.MustCompile("`[^`]*`")
)

var states = []string{"inbox", "ready", "active", "failed", "archive"}

// Diagnostic is one stable, file-and-field-specific validation error.
type Diagnostic struct{ Path, Field, Message string }

func (d Diagnostic) Error() string { return fmt.Sprintf("%s: %s: %s", d.Path, d.Field, d.Message) }

type task struct {
	rel, state, id string
	deps           []string
}

// Validate checks all tasks below root, or reports only diagnostics belonging
// to selected. The complete tree is still read to enforce IDs and dependencies.
func Validate(root, selected string) []Diagnostic {
	root, _ = filepath.Abs(root)
	selectedAbs := ""
	if selected != "" {
		if filepath.IsAbs(selected) {
			selectedAbs, _ = filepath.Abs(selected)
		} else {
			selectedAbs, _ = filepath.Abs(filepath.Join(root, selected))
		}
	}
	var diagnostics []Diagnostic
	var tasksFound []task
	validStates := map[string]bool{}
	for _, state := range states {
		validStates[state] = true
		dir := filepath.Join(root, "tasks", state)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				diagnostics = append(diagnostics, Diagnostic{filepath.Join("tasks", state), "directory", err.Error()})
			}
			continue
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				diagnostics = append(diagnostics, Diagnostic{relPath(root, path), "file", err.Error()})
				continue
			}
			if info.IsDir() {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				diagnostics = append(diagnostics, Diagnostic{relPath(root, path), "file", err.Error()})
				continue
			}
			if entry.Name() == ".gitkeep" && len(data) == 0 {
				continue
			}
			if len(data) == 0 {
				diagnostics = append(diagnostics, Diagnostic{relPath(root, path), "heading", "empty task file must contain # TF-NNN: title"})
				continue
			}
			item := task{rel: relPath(root, path), state: state}
			if !utf8.Valid(data) {
				diagnostics = append(diagnostics, Diagnostic{item.rel, "encoding", "file is not valid UTF-8"})
				continue
			}
			if strings.Contains(string(data), "\r") {
				diagnostics = append(diagnostics, Diagnostic{item.rel, "line endings", "use LF line endings"})
			}
			if strings.Contains(string(data), "\t") {
				diagnostics = append(diagnostics, Diagnostic{item.rel, "format", "tabs are not allowed"})
			}
			match := filePattern.FindStringSubmatch(entry.Name())
			if match == nil {
				diagnostics = append(diagnostics, Diagnostic{item.rel, "filename", "must match TF-NNN-lowercase-slug.md"})
			} else {
				item.id = match[1]
			}
			lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
				diagnostics = append(diagnostics, Diagnostic{item.rel, "heading", "first line must be # TF-NNN: title"})
			} else {
				heading := regexp.MustCompile(`^# (TF-[0-9]{3}): (.+)$`).FindStringSubmatch(lines[0])
				if heading == nil {
					diagnostics = append(diagnostics, Diagnostic{item.rel, "heading", "must match # TF-NNN: non-empty title"})
				} else if strings.TrimSpace(heading[2]) == "" {
					diagnostics = append(diagnostics, Diagnostic{item.rel, "title", "must not be empty or whitespace"})
				} else if item.id != "" && heading[1] != item.id {
					diagnostics = append(diagnostics, Diagnostic{item.rel, "heading ID", fmt.Sprintf("%s does not match filename ID %s", heading[1], item.id)})
				} else if item.id == "" {
					item.id = heading[1]
				}
			}
			if state != "inbox" && utf8.Valid(data) {
				item.deps = validateContract(root, item, lines, &diagnostics)
			}
			tasksFound = append(tasksFound, item)
		}
	}
	if entries, err := os.ReadDir(filepath.Join(root, "tasks")); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() || validStates[entry.Name()] {
				continue
			}
			dir := filepath.Join(root, "tasks", entry.Name())
			children, _ := os.ReadDir(dir)
			for _, child := range children {
				if child.IsDir() {
					continue
				}
				data, e := os.ReadFile(filepath.Join(dir, child.Name()))
				if e == nil && len(data) > 0 {
					diagnostics = append(diagnostics, Diagnostic{filepath.ToSlash(filepath.Join("tasks", entry.Name(), child.Name())), "state", "task files must be under inbox, ready, active, failed, or archive"})
				}
			}
		}
	}
	byID := map[string][]task{}
	for _, item := range tasksFound {
		if item.id != "" {
			byID[item.id] = append(byID[item.id], item)
		}
	}
	for id, matches := range byID {
		if len(matches) > 1 {
			for _, item := range matches {
				diagnostics = append(diagnostics, Diagnostic{item.rel, "ID", fmt.Sprintf("duplicate task ID %s", id)})
			}
		}
	}
	graph := map[string][]string{}
	for _, item := range tasksFound {
		for _, dep := range item.deps {
			targets := byID[dep]
			if len(targets) != 1 {
				diagnostics = append(diagnostics, Diagnostic{item.rel, "dependency", fmt.Sprintf("%s must exist in exactly one task file", dep)})
				continue
			}
			if targets[0].state != "archive" {
				diagnostics = append(diagnostics, Diagnostic{item.rel, "dependency", fmt.Sprintf("%s is unresolved; dependency must be in tasks/archive", dep)})
			}
			graph[item.id] = append(graph[item.id], dep)
		}
	}
	for id, matches := range byID {
		if len(matches) == 1 && hasCycle(id, id, graph, map[string]bool{}) {
			diagnostics = append(diagnostics, Diagnostic{matches[0].rel, "dependency", "dependency cycle detected"})
		}
	}
	if selectedAbs != "" {
		filtered := diagnostics[:0]
		for _, d := range diagnostics {
			abs := filepath.Join(root, d.Path)
			if abs == selectedAbs {
				filtered = append(filtered, d)
			}
		}
		diagnostics = filtered
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Path != diagnostics[j].Path {
			return diagnostics[i].Path < diagnostics[j].Path
		}
		if diagnostics[i].Field != diagnostics[j].Field {
			return diagnostics[i].Field < diagnostics[j].Field
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
	return diagnostics
}

func validateContract(root string, item task, lines []string, ds *[]Diagnostic) []string {
	want := []string{"## Goal", "## Dependencies", "## Scope", "## Constraints", "## Success criteria", "## Verification"}
	if item.state == "active" {
		want = append(want, "## Claim")
	}
	sections := map[string][]string{}
	order := []string{}
	current := ""
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "## ") {
			current = line
			order = append(order, line)
			if _, ok := sections[current]; ok {
				*ds = append(*ds, Diagnostic{item.rel, current, "section must appear exactly once"})
			}
			sections[current] = nil
			continue
		}
		if strings.HasPrefix(line, "#") && (current != "## Success criteria" || !criterionPattern.MatchString(line)) {
			*ds = append(*ds, Diagnostic{item.rel, strings.TrimPrefix(current, "## "), "unsupported heading"})
		}
		plainLine := inlineCodePattern.ReplaceAllString(line, "")
		if current != "" && (strings.HasPrefix(strings.TrimSpace(line), "```") || htmlPattern.MatchString(plainLine) || strings.HasPrefix(strings.TrimSpace(line), "|") || regexp.MustCompile(`\[[^]]+\]\(`).MatchString(plainLine) || strings.HasPrefix(line, "  - ") || strings.HasPrefix(line, "\t- ")) {
			*ds = append(*ds, Diagnostic{item.rel, strings.TrimPrefix(current, "## "), "unsupported Markdown syntax"})
		}
		if current != "" {
			sections[current] = append(sections[current], line)
		}
	}
	idx := 0
	for _, h := range order {
		if h == "## Outcome" && item.state == "archive" {
			continue
		}
		if h == "## Claim" && (item.state == "archive" || item.state == "failed") && idx == len(want) {
			continue
		}
		if idx >= len(want) || h != want[idx] {
			*ds = append(*ds, Diagnostic{item.rel, "headings", fmt.Sprintf("unexpected or out-of-order heading %q", h)})
			continue
		}
		idx++
	}
	for _, h := range want {
		if _, ok := sections[h]; !ok {
			*ds = append(*ds, Diagnostic{item.rel, h, "required section is missing"})
		}
	}
	for _, h := range []string{"## Goal", "## Scope", "## Constraints", "## Verification"} {
		if bodyText(sections[h]) == "" {
			*ds = append(*ds, Diagnostic{item.rel, strings.TrimPrefix(h, "## "), "must contain non-empty prose"})
		}
	}
	deps := []string{}
	depLines := nonempty(sections["## Dependencies"])
	if len(depLines) == 1 && depLines[0] == "None" {
	} else {
		seen := map[string]bool{}
		for _, line := range depLines {
			if !strings.HasPrefix(line, "- ") {
				*ds = append(*ds, Diagnostic{item.rel, "Dependencies", "use None or - TF-NNN lines"})
				continue
			}
			id := strings.TrimPrefix(line, "- ")
			if !idPattern.MatchString(id) {
				*ds = append(*ds, Diagnostic{item.rel, "Dependencies", fmt.Sprintf("invalid dependency %q", id)})
				continue
			}
			if id == item.id {
				*ds = append(*ds, Diagnostic{item.rel, "Dependencies", "task cannot depend on itself"})
			}
			if seen[id] {
				*ds = append(*ds, Diagnostic{item.rel, "Dependencies", fmt.Sprintf("duplicate dependency %s", id)})
			}
			seen[id] = true
			deps = append(deps, id)
		}
		if len(depLines) == 0 {
			*ds = append(*ds, Diagnostic{item.rel, "Dependencies", "must contain None or one or more task IDs"})
		}
	}
	legacy := item.state == "archive" && legacyCriteria(sections["## Success criteria"])
	if !legacy {
		validateCriteria(item, sections["## Success criteria"], ds)
	}
	claim := sections["## Claim"]
	if item.state == "ready" && hasSection(order, "## Claim") {
		*ds = append(*ds, Diagnostic{item.rel, "Claim", "ready tasks must not contain a Claim block"})
	}
	if item.state == "active" || hasSection(order, "## Claim") {
		validateClaim(root, item, claim, ds)
	}
	if hasSection(order, "## Outcome") {
		if item.state != "archive" || order[len(order)-1] != "## Outcome" || (len(order) < 2 || (order[len(order)-2] != "## Verification" && order[len(order)-2] != "## Claim")) {
			*ds = append(*ds, Diagnostic{item.rel, "Outcome", "allowed only at the end of an archived task"})
		} else if bodyText(sections["## Outcome"]) == "" {
			*ds = append(*ds, Diagnostic{item.rel, "Outcome", "must contain non-empty prose"})
		}
	}
	return deps
}

func validateCriteria(item task, lines []string, ds *[]Diagnostic) {
	expected := 1
	active := false
	hasCheck := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := criterionPattern.FindStringSubmatch(line); m != nil {
			if strings.TrimSpace(m[2]) == "" {
				*ds = append(*ds, Diagnostic{item.rel, "Success criteria", "criterion text must not be empty or whitespace"})
			}
			if active && !hasCheck {
				*ds = append(*ds, Diagnostic{item.rel, "Success criteria", "criterion is missing its Check line"})
			}
			n := 0
			fmt.Sscanf(m[1], "%d", &n)
			if n != expected {
				*ds = append(*ds, Diagnostic{item.rel, "Success criteria", fmt.Sprintf("criterion sequence must use C%d", expected)})
			}
			expected++
			active = true
			hasCheck = false
			continue
		}
		if strings.HasPrefix(line, "Check: ") && active && !hasCheck && strings.TrimSpace(strings.TrimPrefix(line, "Check: ")) != "" {
			hasCheck = true
			continue
		}
		*ds = append(*ds, Diagnostic{item.rel, "Success criteria", fmt.Sprintf("unexpected line %q; expected criterion heading or Check", line)})
	}
	if active && !hasCheck {
		*ds = append(*ds, Diagnostic{item.rel, "Success criteria", "criterion is missing its Check line"})
	}
	if !active {
		*ds = append(*ds, Diagnostic{item.rel, "Success criteria", "must contain at least one criterion/check pair"})
	}
}

func legacyCriteria(lines []string) bool {
	found := false
	for _, line := range nonempty(lines) {
		if strings.HasPrefix(line, "- ") {
			found = true
			continue
		}
		if strings.HasPrefix(line, "Check: ") || strings.HasPrefix(line, "### ") {
			return false
		}
		if !found {
			return false
		}
	}
	return found
}
func validateClaim(root string, item task, lines []string, ds *[]Diagnostic) {
	required := []string{"Owner:", "Branch:", "Worktree:", "Base commit:", "Started at:"}
	values := map[string]string{}
	seen := map[string]bool{}
	for _, line := range nonempty(lines) {
		matched := false
		for _, key := range required {
			if strings.HasPrefix(line, key+" ") {
				if seen[key] {
					*ds = append(*ds, Diagnostic{item.rel, "Claim", key + " must appear once"})
				}
				values[key] = strings.TrimSpace(strings.TrimPrefix(line, key))
				seen[key] = true
				matched = true
				break
			}
		}
		if !matched {
			*ds = append(*ds, Diagnostic{item.rel, "Claim", fmt.Sprintf("invalid metadata line %q", line)})
		}
	}
	for _, key := range required {
		if values[key] == "" {
			*ds = append(*ds, Diagnostic{item.rel, "Claim", key + " must have a non-empty value"})
		}
	}
	base := values["Base commit:"]
	if !regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`).MatchString(base) {
		*ds = append(*ds, Diagnostic{item.rel, "Claim", "Base commit must be a full lowercase Git object ID"})
	}
	started := values["Started at:"]
	parsed, err := time.Parse(time.RFC3339, started)
	if err != nil || parsed.Location() != time.UTC || !strings.HasSuffix(started, "Z") {
		*ds = append(*ds, Diagnostic{item.rel, "Claim", "Started at must be an RFC 3339 UTC timestamp"})
	}
	worktree := values["Worktree:"]
	if !filepath.IsAbs(worktree) || filepath.Clean(worktree) != worktree {
		*ds = append(*ds, Diagnostic{item.rel, "Claim", "Worktree must be a normalized absolute path"})
	} else if loaded, err := config.Load(root); err == nil {
		configured := loaded.Git.WorktreeRoot
		if !filepath.IsAbs(configured) {
			configured = filepath.Join(root, configured)
		}
		configured, _ = filepath.Abs(configured)
		if resolved, e := filepath.EvalSymlinks(configured); e == nil {
			configured = resolved
		}
		want := filepath.Join(configured, item.id)
		want, _ = filepath.Abs(want)
		if resolved, e := filepath.EvalSymlinks(worktree); e == nil {
			worktree = resolved
		}
		if worktree != want {
			*ds = append(*ds, Diagnostic{item.rel, "Claim", fmt.Sprintf("Worktree must equal configured task path %s", want)})
		}
	}
}
func hasSection(order []string, name string) bool {
	for _, v := range order {
		if v == name {
			return true
		}
	}
	return false
}
func nonempty(lines []string) []string {
	out := []string{}
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}
func bodyText(lines []string) string {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
func hasCycle(start, current string, g map[string][]string, seen map[string]bool) bool {
	if seen[current] {
		return current == start
	}
	seen[current] = true
	for _, next := range g[current] {
		if next == start || hasCycle(start, next, g, seen) {
			return true
		}
	}
	delete(seen, current)
	return false
}
func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
