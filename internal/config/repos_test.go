package config

import "testing"

func TestAddRepoNew(t *testing.T) {
	var cfg UserConfig
	if err := cfg.AddRepo("clientA", "/path/to/a", false); err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	if cfg.Repos["clientA"] != "/path/to/a" {
		t.Errorf("Repos[clientA] = %q, want /path/to/a", cfg.Repos["clientA"])
	}
}

func TestAddRepoCollisionWithoutForce(t *testing.T) {
	cfg := UserConfig{Repos: map[string]string{"clientA": "/old/path"}}
	err := cfg.AddRepo("clientA", "/new/path", false)
	if err == nil {
		t.Fatal("expected error overwriting an existing nickname without --force")
	}
	if cfg.Repos["clientA"] != "/old/path" {
		t.Errorf("Repos[clientA] = %q, should be unchanged after a rejected overwrite", cfg.Repos["clientA"])
	}
}

func TestAddRepoCollisionWithForce(t *testing.T) {
	cfg := UserConfig{Repos: map[string]string{"clientA": "/old/path"}}
	if err := cfg.AddRepo("clientA", "/new/path", true); err != nil {
		t.Fatalf("AddRepo with force: %v", err)
	}
	if cfg.Repos["clientA"] != "/new/path" {
		t.Errorf("Repos[clientA] = %q, want /new/path", cfg.Repos["clientA"])
	}
}

func TestRemoveRepoUnknown(t *testing.T) {
	var cfg UserConfig
	if err := cfg.RemoveRepo("nope"); err == nil {
		t.Fatal("expected error removing an unknown nickname")
	}
}

func TestRemoveRepoClearsDefault(t *testing.T) {
	cfg := UserConfig{Repos: map[string]string{"clientA": "/a"}, DefaultRepo: "clientA"}
	if err := cfg.RemoveRepo("clientA"); err != nil {
		t.Fatalf("RemoveRepo: %v", err)
	}
	if cfg.DefaultRepo != "" {
		t.Errorf("DefaultRepo = %q, want empty after removing the default nickname", cfg.DefaultRepo)
	}
	if _, exists := cfg.Repos["clientA"]; exists {
		t.Error("clientA should have been removed from Repos")
	}
}

func TestRemoveRepoKeepsUnrelatedDefault(t *testing.T) {
	cfg := UserConfig{Repos: map[string]string{"clientA": "/a", "clientB": "/b"}, DefaultRepo: "clientB"}
	if err := cfg.RemoveRepo("clientA"); err != nil {
		t.Fatalf("RemoveRepo: %v", err)
	}
	if cfg.DefaultRepo != "clientB" {
		t.Errorf("DefaultRepo = %q, want clientB (unaffected by removing a different nickname)", cfg.DefaultRepo)
	}
}

func TestSetDefaultUnknown(t *testing.T) {
	var cfg UserConfig
	if err := cfg.SetDefault("nope"); err == nil {
		t.Fatal("expected error setting default to an unknown nickname")
	}
}

func TestSetDefaultValid(t *testing.T) {
	cfg := UserConfig{Repos: map[string]string{"clientA": "/a"}}
	if err := cfg.SetDefault("clientA"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if cfg.DefaultRepo != "clientA" {
		t.Errorf("DefaultRepo = %q, want clientA", cfg.DefaultRepo)
	}
}
