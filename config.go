package velocitab

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"
)

const defaultConfigTemplate = `# =============================================================================
#                 Gate Velocitab Configuration
# =============================================================================
# High-performance, customizable tab list and player list formatting.
# =============================================================================

# Main settings
update_interval_ms = 1000
sort_players = true
default_group = "default"

# -----------------------------------------------------------------------------
# Player Roles & Prefixes / Suffixes
# -----------------------------------------------------------------------------
# Define role weights, prefixes, and suffixes.
# Higher weight means higher priority in the TAB list sort order.
[roles.owner]
prefix = "<dark_red>[Owner] </dark_red>"
suffix = ""
weight = 1000

[roles.admin]
prefix = "<red>[Admin] </red>"
suffix = ""
weight = 500

[roles.mod]
prefix = "<gold>[Mod] </gold>"
suffix = ""
weight = 250

[roles.vip]
prefix = "<green>[VIP] </green>"
suffix = ""
weight = 100

[roles.default]
prefix = "<gray>"
suffix = "</gray>"
weight = 0

# Assign specific players to roles (lowercase username)
[player_roles]
# andrei = "owner"

# -----------------------------------------------------------------------------
# Server Groups
# -----------------------------------------------------------------------------
# You can define multiple groups for different backend servers.
# If multiple headers/footers are provided, they will cycle as an animation.

[[groups]]
name = "default"
servers = ["*"] # Matches all servers by default
only_list_players_in_same_server = false
sorting_placeholders = ["%role_weight%", "%username_lower%"]

# Animated header frames (cycles every update_interval_ms)
headers = [
    "<rainbow>═══ Andrei's Minecraft Network ═══</rainbow>\n<gray>Welcome back, <white>%username%</white>!</gray>",
    "<rainbow>═══ Andrei's Minecraft Network ═══</rainbow>\n<gray>Server: <aqua>%server%</aqua> | Ping: <green>%ping%ms</green></gray>",
]

# Animated footer frames
footers = [
    "<gray>Players Online: <green>%players_online%</green>/<dark_gray>%max_players_online%</dark_gray></gray>\n<gold>store.example.com</gold>",
    "<gray>Players on <aqua>%server%</aqua>: <yellow>%local_players_online%</yellow></gray>\n<aqua>discord.gg/example</aqua>",
]

# Player format in the TAB list
format = "<gray>[<aqua>%server%</aqua>]</gray> %prefix%<white>%username%</white>%suffix%"

# Custom replacements for placeholders (e.g. pretty server badges)
[groups.placeholder_replacements]
"%server%" = { "smp" = "<green>SMP</green>", "lobby" = "<aqua>Lobby</aqua>", "nanolimbo" = "<gray>Limbo</gray>", "pumpkin" = "<gold>Pumpkin</gold>" }
`

// Role represents prefix, suffix, and sorting weight.
type Role struct {
	Prefix string `toml:"prefix"`
	Suffix string `toml:"suffix"`
	Weight int    `toml:"weight"`
}

// Group represents a server group tablist definition.
type Group struct {
	Name                         string                       `toml:"name"`
	Servers                      []string                     `toml:"servers"`
	Headers                      []string                     `toml:"headers"`
	Footers                      []string                     `toml:"footers"`
	Format                       string                       `toml:"format"`
	OnlyListPlayersInSameServer  bool                         `toml:"only_list_players_in_same_server"`
	SortingPlaceholders          []string                     `toml:"sorting_placeholders"`
	PlaceholderReplacements      map[string]map[string]string `toml:"placeholder_replacements"`
	serverRegexes                []*regexp.Regexp
}

// MatchesServer checks if a server matches any of the group's server patterns.
func (g *Group) MatchesServer(serverName string) bool {
	if len(g.Servers) == 0 {
		return true
	}
	clean := strings.ToLower(strings.TrimSpace(serverName))
	for _, s := range g.Servers {
		if s == "*" || strings.EqualFold(s, clean) {
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

// Config represents the complete Velocitab configuration.
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
			return nil, fmt.Errorf("failed to create default velocitab config: %w", err)
		}
	}

	data, err := os.ReadFile(cs.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read velocitab config %q: %w", cs.path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML config %q: %w", cs.path, err)
	}

	if cfg.UpdateIntervalMs <= 0 {
		cfg.UpdateIntervalMs = 1000
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

	// Compile regexes for group servers
	for i := range cfg.Groups {
		g := &cfg.Groups[i]
		g.serverRegexes = make([]*regexp.Regexp, 0, len(g.Servers))
		for _, pattern := range g.Servers {
			if strings.ContainsAny(pattern, "^$[]()+?{}.\\|") && pattern != "*" {
				if r, err := regexp.Compile("(?i)" + pattern); err == nil {
					g.serverRegexes = append(g.serverRegexes, r)
				}
			}
		}
	}

	cs.cfg = &cfg
	return &cfg, nil
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
		if g.Name != cs.cfg.DefaultGroup && g.MatchesServer(serverName) {
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
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if cs.cfg == nil {
		return Role{Weight: 0}
	}

	cleanUser := strings.ToLower(strings.TrimSpace(username))
	if roleName, ok := cs.cfg.PlayerRoles[cleanUser]; ok {
		if role, found := cs.cfg.Roles[roleName]; found {
			return role
		}
	}

	if defaultRole, found := cs.cfg.Roles["default"]; found {
		return defaultRole
	}

	return Role{Weight: 0}
}

// GetConfig returns the current Config snapshot.
func (cs *ConfigStore) GetConfig() *Config {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.cfg
}
