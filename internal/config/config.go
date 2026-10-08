// Package config loads and validates TaskFactory project configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	configPath       = ".taskfactory/config.toml"
	protocolVersion  = 1
	maxParallelLimit = 4
)

// Config is the validated project configuration used by TaskFactory.
type Config struct {
	ProtocolVersion int
	Workers         Workers
	Git             Git
	Integration     Integration
	Verification    Verification
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
	path := filepath.Join(projectRoot, configPath)
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
			WorktreeRoot:        ".taskfactory/worktrees",
			IntegrationStrategy: "ff-only",
		},
		Integration: Integration{StopOnMainFailure: true},
		Verification: Verification{
			Worker:      []string{"go test ./..."},
			Integration: []string{"go test ./...", "go vet ./..."},
		},
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

	if config.ProtocolVersion != protocolVersion {
		return Config{}, fmt.Errorf("configuration file %s: protocol_version %d is unsupported; only version 1 is supported", path, config.ProtocolVersion)
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

	root, err := resolveWorktreeRoot(projectRoot, config.Git.WorktreeRoot)
	if err != nil {
		return Config{}, fmt.Errorf("configuration file %s: git.worktree_root: %w", path, err)
	}
	config.Git.WorktreeRoot = root
	return config, nil
}

type fileConfig struct {
	ProtocolVersion int `toml:"protocol_version"`
	Workers         struct {
		MaxParallel int `toml:"max_parallel"`
	} `toml:"workers"`
	Git struct {
		UseWorktrees        bool   `toml:"use_worktrees"`
		WorktreeRoot        string `toml:"worktree_root"`
		IntegrationStrategy string `toml:"integration_strategy"`
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
