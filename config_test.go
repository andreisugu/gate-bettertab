package bettertab

import (
	"os"
	"path/filepath"
	"testing"

	spmanager "github.com/andreisugu/gate-stonyperms/manager"
)

func TestConfigStoreLoadAndMatch(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bettertab.toml")

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

func TestResolveRoleForUserStonyPerms(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bettertab.toml")

	testTOML := `
[roles.admin]
prefix = "&c[AdminCfg] "
weight = 500

[roles.vip]
prefix = "&a[VIPCfg] "
weight = 100

[roles.default]
prefix = "&7"
suffix = ""
weight = 0

[player_roles]
pinned = "admin"
`
	if err := os.WriteFile(configPath, []byte(testTOML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	store := NewConfigStore(configPath)
	if _, err := store.Load(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	effVip := &spmanager.EffectiveUser{
		PrimaryGroup: "vip",
		Prefix:       "&a[VIP] ",
		Groups:       []spmanager.EffectiveGroup{{Name: "vip", Weight: 100}},
	}
	role, roleName := store.ResolveRoleForUser("Pinned", effVip)
	if roleName != "admin" || role.Prefix != "&c[AdminCfg] " || role.Weight != 500 {
		t.Fatalf("expected player_roles override admin, got name=%q role=%+v", roleName, role)
	}

	effChaonius := &spmanager.EffectiveUser{
		PrimaryGroup: "chaonius",
		Prefix:       "&5[Chaonius] ",
		Suffix:       " &d*",
		Groups: []spmanager.EffectiveGroup{
			{Name: "chaonius", Weight: 130},
			{Name: "default", Weight: 0},
		},
	}
	role, roleName = store.ResolveRoleForUser("DynamicUser", effChaonius)
	if roleName != "chaonius" || role.Prefix != "&5[Chaonius] " || role.Suffix != " &d*" || role.Weight != 130 {
		t.Fatalf("unexpected resolved chaonius role: name=%q role=%+v", roleName, role)
	}

	effEmptyPrefix := &spmanager.EffectiveUser{
		PrimaryGroup: "vip",
		Prefix:       "",
		Groups:       []spmanager.EffectiveGroup{{Name: "vip", Weight: 0}},
	}
	role, roleName = store.ResolveRoleForUser("FallbackVip", effEmptyPrefix)
	if roleName != "vip" || role.Prefix != "&a[VIPCfg] " || role.Weight != 100 {
		t.Fatalf("expected config fallback for vip, got name=%q role=%+v", roleName, role)
	}

	effInherited := &spmanager.EffectiveUser{
		PrimaryGroup: "default",
		Prefix:       "&a[VIP] ",
		Groups: []spmanager.EffectiveGroup{
			{Name: "vip", Weight: 100},
			{Name: "default", Weight: 0},
		},
	}
	role, roleName = store.ResolveRoleForUser("InheritedUser", effInherited)
	if roleName != "vip" || role.Prefix != "&a[VIP] " || role.Weight != 100 {
		t.Fatalf("expected highest inherited vip, got name=%q role=%+v", roleName, role)
	}
}
