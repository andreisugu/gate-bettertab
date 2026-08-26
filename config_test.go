package velocitab

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigStoreLoadAndMatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "velocitab.toml")

	testTOML := `
update_interval_ms = 500
sort_players = true
default_group = "default"

[roles.admin]
prefix = "<red>[Admin] </red>"
weight = 100

[player_roles]
steve = "admin"

[[groups]]
name = "hub_group"
servers = ["lobby", "hub", "nanolimbo"]
format = "<aqua>%username%</aqua>"
headers = ["Header 1", "Header 2"]
footers = ["Footer 1"]

[[groups]]
name = "default"
servers = ["*"]
format = "<white>%username%</white>"
`
	if err := os.WriteFile(configPath, []byte(testTOML), 0o644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	store := NewConfigStore(configPath)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.UpdateIntervalMs != 500 {
		t.Errorf("Expected 500ms, got %d", cfg.UpdateIntervalMs)
	}

	// Match hub_group
	group := store.GetGroupForServer("lobby")
	if group == nil || group.Name != "hub_group" {
		t.Errorf("Expected hub_group for 'lobby', got %v", group)
	}

	// Match fallback default group
	groupDefault := store.GetGroupForServer("smp")
	if groupDefault == nil || groupDefault.Name != "default" {
		t.Errorf("Expected default for 'smp', got %v", groupDefault)
	}

	// Check player role
	role := store.GetRoleForPlayer("steve")
	if role.Weight != 100 {
		t.Errorf("Expected role weight 100 for steve, got %d", role.Weight)
	}
}
