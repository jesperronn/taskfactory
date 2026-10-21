package paths

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestDefaultLayout pins every path of the default layout, state inside the
// project, to the literal it has always had.
func TestDefaultLayout(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "proj")
	l := For(root)
	if l.External() || l.Project != root || l.State != root {
		t.Fatalf("For(%q) = %+v, want the default layout", root, l)
	}
	j := func(parts ...string) string { return filepath.Join(append([]string{root}, parts...)...) }
	for name, c := range map[string][2]string{
		"Tasks":                  {l.Tasks(), j("tasks")},
		"TaskDir inbox":          {l.TaskDir("inbox"), j("tasks", "inbox")},
		"TaskDir archive":        {l.TaskDir("archive"), j("tasks", "archive")},
		"TaskFile":               {l.TaskFile("ready", "TF-001-a.md"), j("tasks", "ready", "TF-001-a.md")},
		"Runtime":                {l.Runtime(), j(".taskfactory")},
		"Config":                 {l.Config(), j(".taskfactory", "config.toml")},
		"Evidence":               {l.Evidence("TF-001"), j(".taskfactory", "evidence", "TF-001.jsonl")},
		"EvidenceDir":            {l.EvidenceDir(), j(".taskfactory", "evidence")},
		"IntegrationEvidence":    {l.IntegrationEvidence("TF-001"), j(".taskfactory", "integration-evidence", "TF-001.jsonl")},
		"Logs":                   {l.Logs(), j(".taskfactory", "logs")},
		"LogDir":                 {l.LogDir("TF-001"), j(".taskfactory", "logs", "TF-001")},
		"ClaimLock":              {l.ClaimLock(), j(".taskfactory", "claim.lock")},
		"IntegrationLock":        {l.IntegrationLock(), j(".taskfactory", "integration.lock")},
		"IntegrationStop":        {l.IntegrationStop(), j(".taskfactory", "integration-stop.json")},
		"DefaultWorktreeRoot":    {l.DefaultWorktreeRoot(), j(".taskfactory", "worktrees")},
		"TaskFileRel":            {l.TaskFileRel("failed", "x.md"), "tasks/failed/x.md"},
		"ConfigRel":              {ConfigRel(), ".taskfactory/config.toml"},
		"EvidenceRel":            {EvidenceRel("TF-001"), ".taskfactory/evidence/TF-001.jsonl"},
		"IntegrationEvidenceDir": {IntegrationEvidenceDirRel(), ".taskfactory/integration-evidence"},
		"LogsRel":                {LogsRel(), ".taskfactory/logs"},
		"WorktreesRel":           {WorktreesRel(), ".taskfactory/worktrees"},
	} {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
	if !reflect.DeepEqual(States, []string{"inbox", "ready", "active", "failed", "archive"}) {
		t.Errorf("States = %v", States)
	}
	want := []string{
		".taskfactory/claim.lock",
		".taskfactory/integration.lock",
		".taskfactory/evidence/",
		".taskfactory/integration-evidence/",
		".taskfactory/logs/",
		".taskfactory/worktrees/",
	}
	if got := RuntimeIgnoreLines(); !reflect.DeepEqual(got, want) {
		t.Errorf("RuntimeIgnoreLines = %v, want %v", got, want)
	}
}

func TestRegisteredLayoutIsPerProject(t *testing.T) {
	project := filepath.Join(string(filepath.Separator), "code")
	state := filepath.Join(string(filepath.Separator), "state")
	unregister := Register(Layout{Project: project, State: state})
	l := For(project)
	if !l.External() || l.TaskDir("ready") != filepath.Join(state, "tasks", "ready") {
		t.Errorf("registered layout = %+v", l)
	}
	if other := For(filepath.Join(string(filepath.Separator), "other")); other.External() {
		t.Errorf("an unregistered project got %+v", other)
	}
	unregister()
	if For(project).External() {
		t.Errorf("layout still registered after unregister")
	}
}

func TestConfiguredTasksDir(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "proj")
	l := Layout{Project: root, State: root, TasksDir: ".taskfactory/tasks"}
	for name, c := range map[string][2]string{
		"TasksRel":    {l.TasksRel(), ".taskfactory/tasks"},
		"TaskDirRel":  {l.TaskDirRel("ready"), ".taskfactory/tasks/ready"},
		"TaskFileRel": {l.TaskFileRel("failed", "x.md"), ".taskfactory/tasks/failed/x.md"},
		"Tasks":       {l.Tasks(), filepath.Join(root, ".taskfactory", "tasks")},
		"TaskDir":     {l.TaskDir("inbox"), filepath.Join(root, ".taskfactory", "tasks", "inbox")},
		"TaskFile":    {l.TaskFile("active", "x.md"), filepath.Join(root, ".taskfactory", "tasks", "active", "x.md")},
		"Default":     {Default(root).TasksRel(), "tasks"},
		"DefaultFn":   {DefaultTasksDir(), "tasks"},
	} {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
}
