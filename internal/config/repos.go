package config

import "fmt"

// AddRepo adds or updates a nickname -> path mapping. If nickname
// already exists and force is false, returns an error rather than
// silently overwriting it.
func (cfg *UserConfig) AddRepo(nickname, path string, force bool) error {
	if cfg.Repos == nil {
		cfg.Repos = map[string]string{}
	}
	if _, exists := cfg.Repos[nickname]; exists && !force {
		return fmt.Errorf("repo nickname %q already exists; use --force to overwrite", nickname)
	}
	cfg.Repos[nickname] = path
	return nil
}

// RemoveRepo removes nickname, clearing DefaultRepo if it pointed at the
// removed nickname. Returns an error if nickname doesn't exist.
func (cfg *UserConfig) RemoveRepo(nickname string) error {
	if _, exists := cfg.Repos[nickname]; !exists {
		return fmt.Errorf("unknown repo nickname %q", nickname)
	}
	delete(cfg.Repos, nickname)
	if cfg.DefaultRepo == nickname {
		cfg.DefaultRepo = ""
	}
	return nil
}

// SetDefault sets DefaultRepo to nickname. Returns an error if nickname
// doesn't exist.
func (cfg *UserConfig) SetDefault(nickname string) error {
	if _, exists := cfg.Repos[nickname]; !exists {
		return fmt.Errorf("unknown repo nickname %q", nickname)
	}
	cfg.DefaultRepo = nickname
	return nil
}
