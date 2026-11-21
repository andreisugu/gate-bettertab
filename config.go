package bettertab

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	spmanager "github.com/andreisugu/gate-stonyperms/manager"
	"github.com/pelletier/go-toml/v2"
)

const defaultConfigTemplate = `# =============================================================================
#                 Gate BetterTab Configuration
# =============================================================================
# High-performance, customizable tab list and player list formatting for Gate.
# =============================================================================

# Main settings
update_interval_ms = 1000
sort_players = true
default_group = "default"

# -----------------------------------------------------------------------------
# Player Roles & Prefixes / Suffixes
# -----------------------------------------------------------------------------
# Higher weight means the role appears higher in the TAB list sort order.
[roles.owner]
prefix = "<dark_red>[Owner] </dark_red>"
suffix = ""
display_name = "Owner"
weight = 1000

[roles.admin]
prefix = "<red>[Admin] </red>"
suffix = ""
display_name = "Admin"
weight = 500

[roles.mod]
prefix = "<gold>[Mod] </gold>"
suffix = ""
display_name = "Mod"
weight = 250

[roles.vip]
prefix = "<green>[VIP] </green>"
suffix = ""
display_name = "VIP"
weight = 100

[roles.default]
prefix = "<gray>"
suffix = "</gray>"
display_name = "Player"
weight = 0

# Assign specific players to roles (lowercase username)
[player_roles]
# andrei = "owner"

# -----------------------------------------------------------------------------
# Server Groups
# -----------------------------------------------------------------------------
# Define tab list formatting for specific backend server groups.
# Server patterns support exact names, "*" wildcard, globs ("lobby_*"), or regexes ("r=^smp-[0-9]+$").
# If multiple headers/footers are provided, they will cycle smoothly across ticks.

[[groups]]
name = "default"
servers = ["*"]
only_list_players_in_same_server = false
sorting_placeholders = ["%role_weight%", "%username_lower%"]

# Animated header frames (cycles every update_interval_ms)
headers = [
    "<rainbow:!0><b>═══ Minecraft Proxy Network ═══</b></rainbow>\n<gray>Welcome back, <white>%username%</white>!</gray>",
    "<rainbow:!2><b>═══ Minecraft Proxy Network ═══</b></rainbow>\n<gray>Welcome back, <white>%username%</white>!</gray>",
    "<rainbow:!4><b>═══ Minecraft Proxy Network ═══</b></rainbow>\n<gray>Server: <aqua>%server%</aqua> | Ping: %ping_colored%</gray>",
    "<rainbow:!6><b>═══ Minecraft Proxy Network ═══</b></rainbow>\n<gray>Server: <aqua>%server%</aqua> | Ping: %ping_colored%</gray>",
]

# Animated footer frames
footers = [
    "<gray>Players Online: <green>%players_online%</green>/<dark_gray>%max_players_online%</dark_gray></gray>\n<gold>store.example.com</gold>",
    "<gray>Players on <aqua>%server%</aqua>: <yellow>%local_players_online%</yellow></gray>\n<aqua>discord.gg/example</aqua>",
]

# Tablist player entry formatting
format = "<gray>[<aqua>%server%</aqua>]</gray> %prefix%<white>%username%</white>%suffix%"

# Custom replacements for placeholders (e.g. pretty server badges)
[groups.placeholder_replacements]
"%server%" = { "smp" = "<green>SMP</green>", "lobby" = "<aqua>Lobby</aqua>", "survival" = "<gold>Survival</gold>", "limbo" = "<gray>Limbo</gray>" }
`

// Role represents prefix, suffix, display name, and sorting weight.
type Role struct {
	Prefix      string `toml:"prefix"`
	Suffix      string `toml:"suffix"`
	DisplayName string `toml:"display_name"`
	Weight      int    `toml:"weight"`
}

// Group represents a server group tablist definition.
type Group struct {
	Name                        string                       `toml:"name"`
	Servers                     []string                     `toml:"servers"`
	Headers                     []string                     `toml:"headers"`
	Footers                     []string                     `toml:"footers"`
	Format                      string                       `toml:"format"`
	OnlyListPlayersInSameServer bool                         `toml:"only_list_players_in_same_server"`
	SortingPlaceholders         []string                     `toml:"sorting_placeholders"`
	PlaceholderReplacements     map[string]map[string]string `toml:"placeholder_replacements"`
	serverRegexes               []*regexp.Regexp
	serverGlobs                 []string
}

// MatchesServer checks if a server matches any of the group's server patterns.
func (g *Group) MatchesServer(serverName string) bool {
	if len(g.Servers) == 0 {
		return true
	}
	clean := strings.ToLower(strings.TrimSpace(serverName))
	for _, s := range g.Servers {
		trimmed := strings.TrimSpace(s)
		if trimmed == "*" || strings.EqualFold(trimmed, clean) {
			return true
		}
	}
	for _, glob := range g.serverGlobs {
		if matched, err := path.Match(glob, clean); err == nil && matched {
			return true
		}
	}
	for _, r := range g.serverRegexes {
		if r.MatchString(clean) {
			return true
		}
	}
	return false
}

// Config represents the complete BetterTab configuration.
type Config struct {
	UpdateIntervalMs int               `toml:"update_interval_ms"`
	SortPlayers      bool              `toml:"sort_players"`
	DefaultGroup     string            `toml:"default_group"`
	Roles            map[string]Role   `toml:"roles"`
	PlayerRoles      map[string]string `toml:"player_roles"`
	Groups           []Group           `toml:"groups"`
}

// ConfigStore provides thread-safe configuration management.
type ConfigStore struct {
	path string
	mu   sync.RWMutex
	cfg  *Config
}

// NewConfigStore creates a new ConfigStore.
func NewConfigStore(path string) *ConfigStore {
	return &ConfigStore{path: path}
}

// Path returns the active config file path.
func (cs *ConfigStore) Path() string {
	return cs.path
}

// Load loads or creates the configuration file.
func (cs *ConfigStore) Load() (*Config, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if _, err := os.Stat(cs.path); os.IsNotExist(err) {
		dir := filepath.Dir(cs.path)
		if dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0o755)
		}
		if err := os.WriteFile(cs.path, []byte(defaultConfigTemplate), 0o644); err != nil {
			return nil, fmt.Errorf("failed to create default bettertab config: %w", err)
		}
	}

	data, err := os.ReadFile(cs.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read bettertab config %q: %w", cs.path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML config %q: %w", cs.path, err)
	}

	if cfg.UpdateIntervalMs <= 0 {
		cfg.UpdateIntervalMs = 1000
	} else if cfg.UpdateIntervalMs < 50 {
		cfg.UpdateIntervalMs = 50
	}
	if cfg.DefaultGroup == "" {
		cfg.DefaultGroup = "default"
	}
	if cfg.Roles == nil {
		cfg.Roles = make(map[string]Role)
	}
	if cfg.PlayerRoles == nil {
		cfg.PlayerRoles = make(map[string]string)
	}

	// Normalize player roles to lowercase
	normalizedPR := make(map[string]string, len(cfg.PlayerRoles))
	for player, role := range cfg.PlayerRoles {
		normalizedPR[strings.ToLower(strings.TrimSpace(player))] = strings.ToLower(strings.TrimSpace(role))
	}
	cfg.PlayerRoles = normalizedPR

	// Compile glob & regex server matchers
	for i := range cfg.Groups {
		g := &cfg.Groups[i]
		g.serverRegexes = make([]*regexp.Regexp, 0, len(g.Servers))
		g.serverGlobs = make([]string, 0, len(g.Servers))
		for _, rawPattern := range g.Servers {
			pattern := strings.TrimSpace(rawPattern)
			if pattern == "" || pattern == "*" {
				continue
			}
			if expr, isExplicitRegex := cutPrefixFold(pattern, "r="); isExplicitRegex {
				if r, err := regexp.Compile("(?i)" + expr); err == nil {
					g.serverRegexes = append(g.serverRegexes, r)
				}
				continue
			}
			if !strings.ContainsAny(pattern, "^$[]()+{}\\|") && strings.ContainsAny(pattern, "*?") {
				g.serverGlobs = append(g.serverGlobs, strings.ToLower(pattern))
				continue
			}
			if strings.ContainsAny(pattern, "^$[]()+?{}.\\|") {
				if r, err := regexp.Compile("(?i)" + pattern); err == nil {
					g.serverRegexes = append(g.serverRegexes, r)
				}
			}
		}
	}

	cs.cfg = &cfg
	return &cfg, nil
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):], true
	}
	return s, false
}

// GetGroupForServer finds the matching Group for a given server name.
func (cs *ConfigStore) GetGroupForServer(serverName string) *Group {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if cs.cfg == nil || len(cs.cfg.Groups) == 0 {
		return nil
	}

	for i := range cs.cfg.Groups {
		g := &cs.cfg.Groups[i]
		if !strings.EqualFold(g.Name, cs.cfg.DefaultGroup) && g.MatchesServer(serverName) {
			return g
		}
	}

	// Fallback to default group
	for i := range cs.cfg.Groups {
		if strings.EqualFold(cs.cfg.Groups[i].Name, cs.cfg.DefaultGroup) {
			return &cs.cfg.Groups[i]
		}
	}

	return &cs.cfg.Groups[0]
}

// GetRoleForPlayer returns the role associated with a player name.
func (cs *ConfigStore) GetRoleForPlayer(username string) Role {
	role, _ := cs.GetRoleWithInfo(username)
	return role
}

// GetRoleWithInfo returns the role and its role key name.
func (cs *ConfigStore) GetRoleWithInfo(username string) (Role, string) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if cs.cfg == nil {
		return Role{Weight: 0}, "default"
	}

	cleanUser := strings.ToLower(strings.TrimSpace(username))
	if roleName, ok := cs.cfg.PlayerRoles[cleanUser]; ok {
		if role, found := cs.findRoleLocked(roleName); found {
			return role, roleName
		}
	}

	if defaultRole, found := cs.findRoleLocked("default"); found {
		return defaultRole, "default"
	}

	return Role{Weight: 0}, "default"
}

// ResolveRoleForUser resolves a player's Role and role name from BetterTab config
// and an optional live StonyPerms EffectiveUser snapshot.
func (cs *ConfigStore) ResolveRoleForUser(username string, user *spmanager.EffectiveUser) (Role, string) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	cleanUser := strings.ToLower(strings.TrimSpace(username))
	if cs.cfg != nil && cleanUser != "" {
		if roleName, ok := cs.cfg.PlayerRoles[cleanUser]; ok {
			if role, found := cs.findRoleLocked(roleName); found {
				return role, roleName
			}
		}
	}

	if user != nil {
		roleName := strings.ToLower(strings.TrimSpace(user.PrimaryGroup))
		spWeight := 0
		spDisplayName := ""
		foundPrimary := false
		for _, g := range user.Groups {
			if strings.EqualFold(g.Name, roleName) {
				spWeight = g.Weight
				spDisplayName = strings.TrimSpace(g.DisplayName)
				foundPrimary = true
				break
			}
		}
		if (roleName == "" || roleName == "default") && len(user.Groups) > 0 {
			roleName = strings.ToLower(strings.TrimSpace(user.Groups[0].Name))
			spWeight = user.Groups[0].Weight
			spDisplayName = strings.TrimSpace(user.Groups[0].DisplayName)
			foundPrimary = true
		}
		if roleName == "" {
			roleName = "default"
		}

		cfgRole, inCfg := cs.findRoleLocked(roleName)
		role := Role{
			Weight:      spWeight,
			DisplayName: spDisplayName,
		}
		if (!foundPrimary || spWeight == 0) && inCfg && cfgRole.Weight != 0 {
			role.Weight = cfgRole.Weight
		}
		if role.DisplayName == "" && inCfg && cfgRole.DisplayName != "" {
			role.DisplayName = cfgRole.DisplayName
		}

		switch {
		case user.Prefix != "":
			role.Prefix = user.Prefix
		case inCfg && cfgRole.Prefix != "":
			role.Prefix = cfgRole.Prefix
		default:
			if defRole, ok := cs.findRoleLocked("default"); ok {
				role.Prefix = defRole.Prefix
			}
		}

		switch {
		case user.Suffix != "":
			role.Suffix = user.Suffix
		case inCfg && cfgRole.Suffix != "":
			role.Suffix = cfgRole.Suffix
		default:
			if defRole, ok := cs.findRoleLocked("default"); ok && roleName == "default" {
				role.Suffix = defRole.Suffix
			}
		}
		return role, roleName
	}

	if cs.cfg != nil {
		if defaultRole, found := cs.findRoleLocked("default"); found {
			return defaultRole, "default"
		}
	}

	return Role{Weight: 0}, "default"
}

func (cs *ConfigStore) findRoleLocked(name string) (Role, bool) {
	if cs.cfg == nil || len(cs.cfg.Roles) == 0 {
		return Role{}, false
	}
	if role, ok := cs.cfg.Roles[name]; ok {
		return role, true
	}
	clean := strings.ToLower(strings.TrimSpace(name))
	for k, v := range cs.cfg.Roles {
		if strings.EqualFold(k, clean) {
			return v, true
		}
	}
	return Role{}, false
}

// GetConfig returns the current Config snapshot.
func (cs *ConfigStore) GetConfig() *Config {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.cfg
}
