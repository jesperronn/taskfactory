// Package config loads and validates TaskFactory project configuration.
package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"taskfactory/internal/paths"
)

// DefaultBaseBranch is the base branch when git.base_branch is not set.
const DefaultBaseBranch = "main"

// DefaultRetryBudget is the number of attempts a task may use before requeue
// refuses it, when retry_budget is not set.
const DefaultRetryBudget = 5

const (
	protocolVersion  = 1
	maxParallelLimit = 4
)

// Config is the validated project configuration used by TaskFactory.
type Config struct {
	ProtocolVersion int
	// ProjectRoot is the code repository named by project_root. It is empty
	// unless the config sets it; it is required when state is external.
	ProjectRoot  string
	Workers      Workers
	Git          Git
	Integration  Integration
	Verification Verification
	// RetryBudget is the number of attempts a task may use before requeue
	// refuses to requeue it. Only requeue reads it.
	RetryBudget int
}

// Workers contains worker execution policy.
type Workers struct {
	MaxParallel int
}

// Git contains repository and worktree policy.
type Git struct {
	UseWorktrees        bool
	WorktreeRoot        string
	IntegrationStrategy string
	// BaseBranch is the local branch that claim branches from and integrate
	// fast-forwards. It defaults to "main".
	BaseBranch string
}

// Integration contains integration pipeline policy.
type Integration struct {
	StopOnMainFailure bool
}

// Verification contains ordered shell commands for each verification stage.
type Verification struct {
	Worker      []string
	Integration []string
	Main        []string
}

// Load reads .taskfactory/config.toml from projectRoot and returns its
// validated configuration. WorktreeRoot is normalized to an absolute path.
func Load(projectRoot string) (Config, error) {
	layout := paths.For(projectRoot)
	path := layout.Config()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, fmt.Errorf("configuration file %s is missing; run taskfactory init from the repository", path)
		}
		return Config{}, fmt.Errorf("read configuration file %s: %w", path, err)
	}

	parsed := fileConfig{}
	metadata, err := toml.Decode(string(data), &parsed)
	if err != nil {
		return Config{}, fmt.Errorf("parse configuration file %s: %w", path, err)
	}
	if unknown := metadata.Undecoded(); len(unknown) > 0 {
		key := unknown[0]
		if metadata.Type(key...) == "Hash" {
			return Config{}, fmt.Errorf("configuration file %s: unknown table [%s]", path, key.String())
		}
		return Config{}, fmt.Errorf("configuration file %s: unknown key %s", path, key.String())
	}

	config := Config{
		ProtocolVersion: protocolVersion,
		Workers:         Workers{MaxParallel: maxParallelLimit},
		Git: Git{
			UseWorktrees:        true,
			WorktreeRoot:        paths.WorktreesRel(),
			IntegrationStrategy: "ff-only",
			BaseBranch:          DefaultBaseBranch,
		},
		Integration: Integration{StopOnMainFailure: true},
		Verification: Verification{
			Worker:      []string{"go test ./..."},
			Integration: []string{"go test ./...", "go vet ./..."},
		},
		RetryBudget: DefaultRetryBudget,
	}
	if metadata.IsDefined("project_root") {
		config.ProjectRoot = parsed.ProjectRoot
	}
	if layout.External() && config.ProjectRoot == "" {
		return Config{}, fmt.Errorf("configuration file %s: project_root is required when the state directory is outside the project; set project_root to the code repository", path)
	}
	if metadata.IsDefined("tasks_dir") {
		if _, err := ValidateTasksDir(layout.State, path, parsed.TasksDir); err != nil {
			return Config{}, err
		}
	}
	if metadata.IsDefined("protocol_version") {
		config.ProtocolVersion = parsed.ProtocolVersion
	}
	if metadata.IsDefined("workers", "max_parallel") {
		config.Workers.MaxParallel = parsed.Workers.MaxParallel
	}
	if metadata.IsDefined("git", "use_worktrees") {
		config.Git.UseWorktrees = parsed.Git.UseWorktrees
	}
	if metadata.IsDefined("git", "worktree_root") {
		config.Git.WorktreeRoot = parsed.Git.WorktreeRoot
	}
	if metadata.IsDefined("git", "integration_strategy") {
		config.Git.IntegrationStrategy = parsed.Git.IntegrationStrategy
	}
	if metadata.IsDefined("git", "base_branch") {
		config.Git.BaseBranch = parsed.Git.BaseBranch
	}
	if metadata.IsDefined("integration", "stop_on_main_failure") {
		config.Integration.StopOnMainFailure = parsed.Integration.StopOnMainFailure
	}
	if metadata.IsDefined("verification", "worker") {
		config.Verification.Worker = parsed.Verification.Worker
	}
	if metadata.IsDefined("verification", "integration") {
		config.Verification.Integration = parsed.Verification.Integration
	}
	if metadata.IsDefined("verification", "main") {
		config.Verification.Main = parsed.Verification.Main
	}
	if metadata.IsDefined("retry_budget") {
		config.RetryBudget = parsed.RetryBudget
	}

	if config.ProtocolVersion != protocolVersion {
		return Config{}, fmt.Errorf("configuration file %s: protocol_version %d is unsupported; only version 1 is supported", path, config.ProtocolVersion)
	}
	if config.RetryBudget < 1 {
		return Config{}, fmt.Errorf("configuration file %s: retry_budget %d must be at least 1", path, config.RetryBudget)
	}
	if config.Workers.MaxParallel < 1 || config.Workers.MaxParallel > maxParallelLimit {
		return Config{}, fmt.Errorf("configuration file %s: workers.max_parallel %d is outside the allowed range 1..4", path, config.Workers.MaxParallel)
	}
	if !config.Git.UseWorktrees {
		return Config{}, fmt.Errorf("configuration file %s: git.use_worktrees must be true in protocol v1; isolated worker worktrees are required", path)
	}
	if config.Git.IntegrationStrategy != "ff-only" {
		return Config{}, fmt.Errorf("configuration file %s: git.integration_strategy must be \"ff-only\", got %q", path, config.Git.IntegrationStrategy)
	}
	if err := ValidateBaseBranch(config.Git.BaseBranch); err != nil {
		return Config{}, fmt.Errorf("configuration file %s: %w", path, err)
	}
	if config.Git.WorktreeRoot == "" {
		return Config{}, fmt.Errorf("configuration file %s: git.worktree_root must be a non-empty path", path)
	}
	for _, item := range []struct {
		key      string
		commands []string
	}{
		{key: "verification.worker", commands: config.Verification.Worker},
		{key: "verification.integration", commands: config.Verification.Integration},
		{key: "verification.main", commands: config.Verification.Main},
	} {
		if item.commands == nil && item.key == "verification.main" {
			continue
		}
		if err := validateCommands(item.key, item.commands); err != nil {
			return Config{}, fmt.Errorf("configuration file %s: %w", path, err)
		}
	}

	root, err := resolveWorktreeRoot(layout.State, config.Git.WorktreeRoot)
	if err != nil {
		return Config{}, fmt.Errorf("configuration file %s: git.worktree_root: %w", path, err)
	}
	config.Git.WorktreeRoot = root
	return config, nil
}

type fileConfig struct {
	ProjectRoot     string `toml:"project_root"`
	TasksDir        string `toml:"tasks_dir"`
	RetryBudget     int    `toml:"retry_budget"`
	ProtocolVersion int    `toml:"protocol_version"`
	Workers         struct {
		MaxParallel int `toml:"max_parallel"`
	} `toml:"workers"`
	Git struct {
		UseWorktrees        bool   `toml:"use_worktrees"`
		WorktreeRoot        string `toml:"worktree_root"`
		IntegrationStrategy string `toml:"integration_strategy"`
		BaseBranch          string `toml:"base_branch"`
	} `toml:"git"`
	Integration struct {
		StopOnMainFailure bool `toml:"stop_on_main_failure"`
	} `toml:"integration"`
	Verification struct {
		Worker      []string `toml:"worker"`
		Integration []string `toml:"integration"`
		Main        []string `toml:"main"`
	} `toml:"verification"`
}

// ProjectRootOf reads only project_root from the config in stateRoot, for
// discovery when state is outside the project. It errors, naming the key,
// when the file does not set it.
func ProjectRootOf(stateRoot string) (string, error) {
	path := paths.Default(stateRoot).Config()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("configuration file %s is missing; project_root must name the code repository", path)
		}
		return "", fmt.Errorf("read configuration file %s: %w", path, err)
	}
	var parsed struct {
		ProjectRoot string `toml:"project_root"`
	}
	if _, err := toml.Decode(string(data), &parsed); err != nil {
		return "", fmt.Errorf("parse configuration file %s: %w", path, err)
	}
	if parsed.ProjectRoot == "" {
		return "", fmt.Errorf("configuration file %s: project_root is required when the state directory is outside the project; set project_root to the code repository", path)
	}
	root := parsed.ProjectRoot
	if !filepath.IsAbs(root) {
		root = filepath.Join(stateRoot, root)
	}
	return filepath.Clean(root), nil
}

// TasksDirOf reads only tasks_dir from the config in stateRoot, for discovery
// before the full config is loaded. It validates the value the same way Load
// does and returns the default when the file or the key is absent.
func TasksDirOf(stateRoot string) (string, error) {
	cfgPath := paths.Default(stateRoot).Config()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return paths.DefaultTasksDir(), nil
		}
		return "", fmt.Errorf("read configuration file %s: %w", cfgPath, err)
	}
	var parsed struct {
		TasksDir *string `toml:"tasks_dir"`
	}
	if _, err := toml.Decode(string(data), &parsed); err != nil {
		return "", fmt.Errorf("parse configuration file %s: %w", cfgPath, err)
	}
	if parsed.TasksDir == nil {
		return paths.DefaultTasksDir(), nil
	}
	return ValidateTasksDir(stateRoot, cfgPath, *parsed.TasksDir)
}

// ValidateTasksDir checks a tasks_dir value and returns its cleaned slash
// form. Both loaders call it. configPath only names the file in errors.
func ValidateTasksDir(stateRoot, configPath, value string) (string, error) {
	bad := func() (string, error) {
		return "", fmt.Errorf("configuration file %s: tasks_dir must be a relative path inside the state directory", configPath)
	}
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || filepath.IsAbs(value) {
		return bad()
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return bad()
		}
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return bad()
	}
	runtime := paths.RuntimeRel()
	if clean == runtime {
		return bad()
	}
	if strings.HasPrefix(clean, runtime+"/") {
		reserved := []string{
			paths.EvidenceDirRel(), paths.IntegrationEvidenceDirRel(), paths.LogsRel(),
			paths.WorktreesRel(), paths.ConfigRel(), paths.ClaimLockRel(),
			paths.IntegrationLockRel(), paths.IntegrationStopRel(),
		}
		for _, r := range reserved {
			if clean == r || strings.HasPrefix(clean, r+"/") {
				return bad()
			}
		}
	}
	// A symlink that leaves the state root is rejected.
	if stateRoot != "" {
		realRoot, err := filepath.EvalSymlinks(stateRoot)
		if err != nil {
			realRoot = filepath.Clean(stateRoot)
		}
		ancestor := filepath.Join(stateRoot, filepath.FromSlash(clean))
		for {
			if _, err := os.Lstat(ancestor); err == nil {
				break
			}
			parent := filepath.Dir(ancestor)
			if parent == ancestor {
				break
			}
			ancestor = parent
		}
		if real, err := filepath.EvalSymlinks(ancestor); err == nil {
			rel, err := filepath.Rel(realRoot, real)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return bad()
			}
		}
	}
	return clean, nil
}

func validateCommands(key string, commands []string) error {
	if len(commands) == 0 {
		return fmt.Errorf("%s must contain at least one command", key)
	}
	for i, command := range commands {
		if command == "" {
			return fmt.Errorf("%s command %d must not be empty", key, i+1)
		}
	}
	return nil
}

func resolveWorktreeRoot(projectRoot, configured string) (string, error) {
	root := configured
	if !filepath.IsAbs(root) {
		root = filepath.Join(projectRoot, root)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	root = filepath.Clean(root)

	// Resolve the closest existing ancestor and append any missing components.
	missing := make([]string, 0)
	ancestor := root
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect %s: %w", ancestor, err)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", fmt.Errorf("cannot find an existing parent directory")
		}
		missing = append(missing, filepath.Base(ancestor))
		ancestor = parent
	}
	info, err := os.Stat(ancestor)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", ancestor, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("existing parent %s is not a directory", ancestor)
	}
	ancestor, err = filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks in %s: %w", ancestor, err)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		ancestor = filepath.Join(ancestor, missing[i])
	}
	return filepath.Clean(ancestor), nil
}

// ValidateBaseBranch checks a git.base_branch value: a plain local branch
// name that Git itself would accept. Errors name the key.
func ValidateBaseBranch(name string) error {
	bad := func(why string) error {
		return fmt.Errorf("git.base_branch %q is not a valid branch name: %s", name, why)
	}
	if name == "" {
		return bad("it is empty")
	}
	if strings.HasPrefix(name, "-") {
		return bad("it must not start with \"-\"")
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.Contains(name, "//") {
		return bad("it has an empty path component")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "@{") || name == "@" {
		return bad("it contains \"..\" or \"@{\"")
	}
	if strings.HasSuffix(name, ".") {
		return bad("it must not end with \".\"")
	}
	for _, r := range name {
		if r <= ' ' || r == 0x7f || strings.ContainsRune("~^:?*[\\", r) {
			return bad("it contains a space, control or special character")
		}
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return bad("a component must not start with \".\" or end with \".lock\"")
		}
	}
	return nil
}

// RetryBudgetOf reads only retry_budget from the config in stateRoot, for
// requeue without loading the whole config. It returns DefaultRetryBudget when
// the file or the key is absent and errors when the value is below 1.
func RetryBudgetOf(stateRoot string) (int, error) {
	cfgPath := paths.Default(stateRoot).Config()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultRetryBudget, nil
		}
		return 0, fmt.Errorf("read configuration file %s: %w", cfgPath, err)
	}
	var parsed struct {
		RetryBudget *int `toml:"retry_budget"`
	}
	if _, err := toml.Decode(string(data), &parsed); err != nil {
		return 0, fmt.Errorf("parse configuration file %s: %w", cfgPath, err)
	}
	if parsed.RetryBudget == nil {
		return DefaultRetryBudget, nil
	}
	if *parsed.RetryBudget < 1 {
		return 0, fmt.Errorf("configuration file %s: retry_budget %d must be at least 1", cfgPath, *parsed.RetryBudget)
	}
	return *parsed.RetryBudget, nil
}

// BaseBranchOf reads only git.base_branch from the config in stateRoot, for
// commands that need the branch without loading the whole config. It validates
// the value as Load does and returns the default when the file or key is absent.
func BaseBranchOf(stateRoot string) (string, error) {
	cfgPath := paths.Default(stateRoot).Config()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultBaseBranch, nil
		}
		return "", fmt.Errorf("read configuration file %s: %w", cfgPath, err)
	}
	var parsed struct {
		Git struct {
			BaseBranch *string `toml:"base_branch"`
		} `toml:"git"`
	}
	if _, err := toml.Decode(string(data), &parsed); err != nil {
		return "", fmt.Errorf("parse configuration file %s: %w", cfgPath, err)
	}
	if parsed.Git.BaseBranch == nil {
		return DefaultBaseBranch, nil
	}
	if err := ValidateBaseBranch(*parsed.Git.BaseBranch); err != nil {
		return "", fmt.Errorf("configuration file %s: %w", cfgPath, err)
	}
	return *parsed.Git.BaseBranch, nil
}
