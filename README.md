# Gate Velocitab

[![Go Reference](https://pkg.go.dev/badge/github.com/andreisugu/gate-velocitab.svg)](https://pkg.go.dev/github.com/andreisugu/gate-velocitab)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

An ultra-high performance, customizable **TAB list & player list formatting extension** for the [Gate Minecraft proxy](https://github.com/minekube/gate), inspired by [Velocitab](https://github.com/WiIIiam278/Velocitab).

---

## ✨ Features

- 🌈 **Full RGB, MiniMessage & Legacy Formatting**: Supports `<rainbow>`, `<gradient:#from:#to>`, `<#RRGGBB>`, `<gold>`, `<b>`, `<i>`, and classic `&a`, `&l`, `&#RRGGBB` color codes.
- 🎬 **Animated Headers & Footers**: Define multiple frames in arrays — they cycle smoothly at your configured update interval.
- 🏷️ **Custom Server Groups**: Group backend servers together and display completely unique TAB menus for each group.
- 👑 **Role System & Prefixes/Suffixes**: Define custom prefixes, suffixes, and sorting weights for Admins, Mods, VIPs, and players.
- 🔢 **Native List Ordering & Sorting**: Automatically sorts player entries by role weight and username.
- 🌐 **Global or Per-Server Isolation**: Choose whether players see all network players across all servers, or only players on their current backend server.
- ⚡ **Zero Backend Plugins Required**: Works 100% on the proxy level with zero overhead and lock-free thread safety.
- 🔄 **Live Hot-Reload**: Update headers, footers, formats, and groups in-game with `/velocitab reload`.

---

## 📦 Installation

### In Pelican / Pterodactyl Panel
Add the plugin package to your **Gate Go Plugins (List)** variable:
```text
github.com/andreisugu/gate-velocitab
```
*(Combine with other plugins: `github.com/andreisugu/gate-simplewhitelist, github.com/andreisugu/gate-hybridforwarding, github.com/andreisugu/gate-velocitab`)*

### In Go Code
```bash
go get github.com/andreisugu/gate-velocitab
```

```go
package main

import (
	"github.com/andreisugu/gate-velocitab"
	"go.minekube.com/gate/cmd/gate"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

func main() {
	proxy.Plugins = append(proxy.Plugins,
		velocitab.Plugin,
	)
	gate.Execute()
}
```

---

## ⚙️ Configuration (`config/velocitab.toml`)

```toml
# Main settings
update_interval_ms = 1000
sort_players = true
default_group = "default"

# Define roles and sorting priority (higher weight = higher in the TAB list)
[roles.owner]
prefix = "<dark_red>[Owner] </dark_red>"
weight = 1000

[roles.admin]
prefix = "<red>[Admin] </red>"
weight = 500

[roles.vip]
prefix = "<green>[VIP] </green>"
weight = 100

[roles.default]
prefix = "<gray>"
suffix = "</gray>"
weight = 0

# Assign players to roles
[player_roles]
# player_name = "admin"

# Server groups
[[groups]]
name = "default"
servers = ["*"]
only_list_players_in_same_server = false
sorting_placeholders = ["%role_weight%", "%username_lower%"]

# Animated header frames (cycles smoothly)
headers = [
    "<rainbow>═══ Minecraft Proxy Network ═══</rainbow>\n<gray>Welcome back, <white>%username%</white>!</gray>",
    "<rainbow>═══ Minecraft Proxy Network ═══</rainbow>\n<gray>Server: <aqua>%server%</aqua> | Ping: <green>%ping%ms</green></gray>",
]

# Animated footer frames
footers = [
    "<gray>Players Online: <green>%players_online%</green>/<dark_gray>%max_players_online%</dark_gray></gray>\n<gold>store.example.com</gold>",
    "<gray>Players on <aqua>%server%</aqua>: <yellow>%local_players_online%</yellow></gray>\n<aqua>discord.gg/example</aqua>",
]

# Player format in the TAB list
format = "<gray>[<aqua>%server%</aqua>]</gray> %prefix%<white>%username%</white>%suffix%"

# Custom replacements (e.g. pretty server badges)
[groups.placeholder_replacements]
"%server%" = { "smp" = "<green>SMP</green>", "lobby" = "<aqua>Lobby</aqua>", "survival" = "<gold>Survival</gold>", "limbo" = "<gray>Limbo</gray>" }
```

---

## 📌 Supported Placeholders

| Placeholder | Description |
| :--- | :--- |
| `%username%` | Player's username |
| `%username_lower%` | Lowercase player username |
| `%server%` | Current backend server name (with replacements applied) |
| `%raw_server%` | Raw backend server name without replacements |
| `%ping%` | Player's connection ping in milliseconds |
| `%players_online%` | Total online players connected to the proxy |
| `%max_players_online%` | Max player limit set in proxy config |
| `%local_players_online%`| Players on the current backend server |
| `%group_players_online%`| Players in the current tab group |
| `%current_date%` | Formatted current date (`YYYY-MM-DD`) |
| `%current_time%` | Formatted current time (`HH:mm:ss`) |
| `%prefix%` | Player's role prefix |
| `%suffix%` | Player's role suffix |
| `%role_weight%` | Player's role sorting weight |

---

## 🎮 In-Game Commands

| Command | Permission | Description |
| :--- | :--- | :--- |
| `/velocitab reload` | Console / Op | Hot-reloads `config/velocitab.toml` instantly without restarting the proxy. |
| `/gatetab reload` | Console / Op | Alias for `/velocitab reload`. |

---

## 📄 License

[MIT](LICENSE) © Andrei
