// Package paths is the single place that knows where TaskFactory keeps task
// state. A Layout separates the project root (the code repository: branches,
// worktrees, fast-forward merges and checks) from the state root (tasks/, the
// .taskfactory evidence, locks and logs, and config.toml). In the default
// layout both are the same directory and every path is what it always was.
package paths

import (
	"path/filepath"
	"sync"
)

// States are the task state directories under tasks/, in lifecycle order.
var States = []string{"inbox", "ready", "active", "failed", "archive"}

const (
	tasksName       = "tasks"
	runtimeName     = ".taskfactory"
	configName      = "config.toml"
	evidenceName    = "evidence"
	integEvidence   = "integration-evidence"
	logsName        = "logs"
	worktreesName   = "worktrees"
	claimLockName   = "claim.lock"
	integLockName   = "integration.lock"
	integStopName   = "integration-stop.json"
	defaultWorktree = runtimeName + "/" + worktreesName
)

// Layout holds the two roots. Both are absolute, cleaned directories.
type Layout struct {
	Project string
	State   string
}

var (
	mu       sync.Mutex
	registry = map[string]Layout{}
)

// Default is the layout with state inside the project.
func Default(root string) Layout { return Layout{Project: root, State: root} }

// Register makes For(l.Project) return l and returns a function that removes
// the registration. A command registers once after discovering the roots.
func Register(l Layout) (unregister func()) {
	keys := keysOf(l.Project)
	mu.Lock()
	for _, k := range keys {
		registry[k] = l
	}
	mu.Unlock()
	return func() {
		mu.Lock()
		for _, k := range keys {
			delete(registry, k)
		}
		mu.Unlock()
	}
}

func keysOf(root string) []string {
	keys := []string{filepath.Clean(root)}
	if r, err := filepath.EvalSymlinks(root); err == nil && filepath.Clean(r) != keys[0] {
		keys = append(keys, filepath.Clean(r))
	}
	return keys
}

// For returns the layout for a project root: the registered one, else the
// default layout with state in the project.
func For(projectRoot string) Layout {
	mu.Lock()
	defer mu.Unlock()
	if len(registry) == 0 {
		return Default(projectRoot)
	}
	if l, ok := registry[filepath.Clean(projectRoot)]; ok {
		return l
	}
	if r, err := filepath.EvalSymlinks(projectRoot); err == nil {
		if l, ok := registry[filepath.Clean(r)]; ok {
			return l
		}
	}
	return Default(projectRoot)
}

// External reports whether state lives outside the project.
func (l Layout) External() bool { return filepath.Clean(l.State) != filepath.Clean(l.Project) }

// TasksRel is the slash-separated tasks directory relative to the state root.
func TasksRel() string { return tasksName }

// TaskDirRel is tasks/<state> relative to the state root, slash separated.
func TaskDirRel(state string) string { return tasksName + "/" + state }

// TaskFileRel is tasks/<state>/<name> relative to the state root.
func TaskFileRel(state, name string) string { return TaskDirRel(state) + "/" + name }

// RuntimeRel is the .taskfactory directory relative to the state root.
func RuntimeRel() string { return runtimeName }

// ConfigRel is the config file relative to the state root, slash separated.
func ConfigRel() string { return runtimeName + "/" + configName }

// EvidenceRel is the worker evidence file relative to the state root.
func EvidenceRel(id string) string { return runtimeName + "/" + evidenceName + "/" + id + ".jsonl" }

// EvidenceDirRel is the worker evidence directory relative to the state root.
func EvidenceDirRel() string { return runtimeName + "/" + evidenceName }

// IntegrationEvidenceDirRel is the integration evidence directory, relative.
func IntegrationEvidenceDirRel() string { return runtimeName + "/" + integEvidence }

// LogsRel is the logs directory relative to the state root.
func LogsRel() string { return runtimeName + "/" + logsName }

// WorktreesRel is the default worktree directory relative to the state root.
func WorktreesRel() string { return defaultWorktree }

// ClaimLockRel and IntegrationLockRel name the lock files, relative.
func ClaimLockRel() string       { return runtimeName + "/" + claimLockName }
func IntegrationLockRel() string { return runtimeName + "/" + integLockName }

// IntegrationStopRel names the stop marker, relative.
func IntegrationStopRel() string { return runtimeName + "/" + integStopName }

func (l Layout) abs(rel string) string { return filepath.Join(l.State, filepath.FromSlash(rel)) }

// Tasks is the tasks directory.
func (l Layout) Tasks() string { return l.abs(TasksRel()) }

// TaskDir is tasks/<state>.
func (l Layout) TaskDir(state string) string { return l.abs(TaskDirRel(state)) }

// TaskFile is tasks/<state>/<name>.
func (l Layout) TaskFile(state, name string) string { return l.abs(TaskFileRel(state, name)) }

// Runtime is the .taskfactory directory.
func (l Layout) Runtime() string { return l.abs(RuntimeRel()) }

// Config is the config.toml path.
func (l Layout) Config() string { return l.abs(ConfigRel()) }

// Evidence is the worker evidence file for a task.
func (l Layout) Evidence(id string) string { return l.abs(EvidenceRel(id)) }

// EvidenceDir is the worker evidence directory.
func (l Layout) EvidenceDir() string { return l.abs(EvidenceDirRel()) }

// IntegrationEvidenceDir is the integration evidence directory.
func (l Layout) IntegrationEvidenceDir() string { return l.abs(IntegrationEvidenceDirRel()) }

// IntegrationEvidence is the integration evidence file for a task.
func (l Layout) IntegrationEvidence(id string) string {
	return filepath.Join(l.IntegrationEvidenceDir(), id+".jsonl")
}

// Logs is the logs directory.
func (l Layout) Logs() string { return l.abs(LogsRel()) }

// LogDir is the log directory of one task.
func (l Layout) LogDir(id string) string { return filepath.Join(l.Logs(), id) }

// ClaimLock and IntegrationLock are the lock file paths.
func (l Layout) ClaimLock() string       { return l.abs(ClaimLockRel()) }
func (l Layout) IntegrationLock() string { return l.abs(IntegrationLockRel()) }

// IntegrationStop is the stop marker path.
func (l Layout) IntegrationStop() string { return l.abs(IntegrationStopRel()) }

// DefaultWorktreeRoot is the worktree directory used when config names none.
func (l Layout) DefaultWorktreeRoot() string { return l.abs(WorktreesRel()) }

// RuntimeIgnoreLines are the project .gitignore lines that hide runtime files,
// in the order init writes them. config.toml is deliberately absent.
func RuntimeIgnoreLines() []string {
	return []string{
		ClaimLockRel(),
		IntegrationLockRel(),
		EvidenceDirRel() + "/",
		IntegrationEvidenceDirRel() + "/",
		LogsRel() + "/",
		WorktreesRel() + "/",
	}
}
