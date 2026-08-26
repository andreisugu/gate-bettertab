package velocitab

import (
	"strings"
	"testing"
)

func TestReplacePlaceholders(t *testing.T) {
	ctx := &PlaceholderContext{
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
			Prefix: "[Admin] ",
			Suffix: "",
			Weight: 500,
		},
		RoleName: "Admin",
		Group: &Group{
			Name: "default",
			PlaceholderReplacements: map[string]map[string]string{
				"%server%": {
					"smp": "<green>Survival</green>",
				},
			},
		},
	}

	template := "%prefix%%username% on %server% [%players_online%/%max_players_online%] (Lobby: %server_online_players_lobby%)"
	res := ReplacePlaceholders(template, ctx)

	if !strings.Contains(res, "[Admin]") {
		t.Errorf("Expected prefix, got %q", res)
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
	}

	templateTrue := "%condition:%local_players_online% > 10 ? Busy : Quiet%"
	resTrue := ReplacePlaceholders(templateTrue, ctx)
	if resTrue != "Busy" {
		t.Errorf("Expected 'Busy', got %q", resTrue)
	}

	templateFalse := "%condition:%local_players_online% < 10 ? Small : Large%"
	resFalse := ReplacePlaceholders(templateFalse, ctx)
	if resFalse != "Large" {
		t.Errorf("Expected 'Large', got %q", resFalse)
	}
}
