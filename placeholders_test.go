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
		Role: Role{
			Prefix: "[Admin] ",
			Suffix: "",
			Weight: 500,
		},
		Group: &Group{
			PlaceholderReplacements: map[string]map[string]string{
				"%server%": {
					"smp": "<green>Survival</green>",
				},
			},
		},
	}

	template := "%prefix%%username% on %server% [%players_online%/%max_players_online%]"
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
}
