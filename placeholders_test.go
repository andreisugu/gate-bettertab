package bettertab

import (
	"strings"
	"testing"
)

func TestReplacePlaceholders(t *testing.T) {
	ctx := &PlaceholderContext{
		RealUsername:     "RealSteve",
		CustomName:       "&bNickSteve",
		ServerName:       "smp",
		ProxyPlayerCount: 15,
		MaxPlayers:       100,
		LocalPlayerCount: 5,
		GroupPlayerCount: 5,
		ServerCounts: map[string]int{
			"lobby": 10,
			"smp":   5,
		},
		Role: Role{
			Prefix:      "[Admin] ",
			Suffix:      "",
			DisplayName: "Administrator",
			Weight:      500,
		},
		RoleName: "Admin",
		Meta: map[string]string{
			"claims": "2500",
		},
		Group: &Group{
			Name: "default",
			PlaceholderReplacements: map[string]map[string]string{
				"%server%": {
					"SMP": "<green>Survival</green>",
				},
				"%role%": {
					"admin": "<red>Staff</red>",
				},
			},
		},
	}

	template := "%prefix%%username% (%real_username%, %nick%, %role_display_name%, %role%, claims=%meta_claims%) on %server% [%players_online%/%max_players_online%] (Lobby: %server_online_players_lobby%)"
	res := ReplacePlaceholders(template, ctx)

	if !strings.Contains(res, "[Admin]") {
		t.Errorf("Expected prefix, got %q", res)
	}
	if !strings.Contains(res, "&bNickSteve (RealSteve, &bNickSteve, Administrator, <red>Staff</red>, claims=2500)") {
		t.Errorf("Expected nick/real_username/role/meta substitutions, got %q", res)
	}
	if !strings.Contains(res, "<green>Survival</green>") {
		t.Errorf("Expected replaced server name '<green>Survival</green>', got %q", res)
	}
	if !strings.Contains(res, "15/100") {
		t.Errorf("Expected player counts '15/100', got %q", res)
	}
	if !strings.Contains(res, "Lobby: 10") {
		t.Errorf("Expected parameterized server count 'Lobby: 10', got %q", res)
	}
}

func TestConditionalPlaceholders(t *testing.T) {
	ctx := &PlaceholderContext{
		LocalPlayerCount: 20,
		RoleName:         "VIP",
	}

	if got := ReplacePlaceholders("%condition:%local_players_online% > 10 ? Busy : Quiet%", ctx); got != "Busy" {
		t.Errorf("Expected 'Busy', got %q", got)
	}
	if got := ReplacePlaceholders("%condition:%local_players_online% < 10 ? Small : Large%", ctx); got != "Large" {
		t.Errorf("Expected 'Large', got %q", got)
	}
	if got := ReplacePlaceholders("%condition:%role% ~= vip ? Donor : Regular%", ctx); got != "Donor" {
		t.Errorf("Expected 'Donor', got %q", got)
	}
	if got := ReplacePlaceholders("%condition:%role% contains VI ? Yes : No%", ctx); got != "Yes" {
		t.Errorf("Expected 'Yes', got %q", got)
	}
}
