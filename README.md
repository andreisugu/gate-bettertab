<div align="center">

# 🌈 Gate BetterTab

**Animated RGB / MiniMessage TAB lists, per-server groups, role sorting & dynamic placeholders for Minekube Gate.**

<p>
  <a href="https://github.com/andreisugu/gate-bettertab/actions/workflows/ci.yml"><img src="https://github.com/andreisugu/gate-bettertab/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://pkg.go.dev/github.com/andreisugu/gate-bettertab"><img src="https://pkg.go.dev/badge/github.com/andreisugu/gate-bettertab.svg" alt="Go Reference" /></a>
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26" />
  <a href="https://github.com/minekube/gate"><img src="https://img.shields.io/badge/Platform-Minekube_Gate-7c3aed?style=flat-square" alt="Minekube Gate" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg?style=flat-square" alt="Apache 2.0" /></a>
</p>

<p>
  <a href="#-features">Features</a> •
  <a href="#-installation">Installation</a> •
  <a href="#-configuration">Configuration</a> •
  <a href="#-commands--permissions">Commands</a> •
  <a href="#-placeholders">Placeholders</a> •
  <a href="#-minekube-gate-extensions-suite">Gate Suite</a>
</p>

</div>

---

## ✨ Features

- **Full RGB, MiniMessage & Legacy Formatting:** Supports `<rainbow>`, `<gradient:#from:#to>`, `<#RRGGBB>`, MiniMessage tags (`<gold>`, `<b>`, `<i>`), and classic `&a` / `&#RRGGBB` codes.
- **Animated Headers & Footers:** Cycle multiple header/footer frames smoothly on `update_interval_ms`.
- **Per-Server Group Profiles & Isolation:** Define distinct tab layouts per backend regex group and optionally show only players on the same backend (`only_list_players_in_same_server`).
- **Role Priority Sorting:** Order entries by role weight, custom sorting placeholders, and username directly in the proxy tab list.
- **Custom Nicknames & Player Toggle:** Players can personalize or toggle their custom tab display without backend plugins.
- **Live Hot-Reload:** Update `config/bettertab.toml` and refresh every player's tab list instantly via `/bettertab reload`.

---

## 📦 Installation

### Using `gate-plugin-builder` / Pelican Egg
Add to your `plugins.txt` or `GATE_PLUGINS`:
```text
github.com/andreisugu/gate-bettertab
```

### In Go Code
```bash
go get github.com/andreisugu/gate-bettertab
```

```go
package main

import (
	bettertab "github.com/andreisugu/gate-bettertab"
	"go.minekube.com/gate/cmd/gate"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

func main() {
	proxy.Plugins = append(proxy.Plugins, bettertab.Plugin)
	gate.Execute()
}
```

---

## ⚙️ Configuration

Generated at `config/bettertab.toml`:

```toml
update_interval_ms = 1000
sort_players = true
default_group = "default"

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

[player_roles]
# andrei = "owner"

[[groups]]
name = "default"
servers = ["*"]
only_list_players_in_same_server = false
sorting_placeholders = ["%role_weight%", "%username_lower%"]

headers = [
    "<rainbow>═══ Minecraft Proxy Network ═══</rainbow>\n<gray>Welcome back, <white>%username%</white>!</gray>",
    "<rainbow>═══ Minecraft Proxy Network ═══</rainbow>\n<gray>Server: <aqua>%server%</aqua> | Ping: <green>%ping%ms</green></gray>",
]

footers = [
    "<gray>Players Online: <green>%players_online%</green>/<dark_gray>%max_players_online%</dark_gray></gray>\n<gold>store.example.com</gold>",
    "<gray>Players on <aqua>%server%</aqua>: <yellow>%local_players_online%</yellow></gray>\n<aqua>discord.gg/example</aqua>",
]

format = "<gray>[<aqua>%server%</aqua>]</gray> %prefix%<white>%username%</white>%suffix%"

[groups.placeholder_replacements]
"%server%" = { "smp" = "<green>SMP</green>", "lobby" = "<aqua>Lobby</aqua>", "survival" = "<gold>Survival</gold>", "limbo" = "<gray>Limbo</gray>" }
```

---

## ⌨️ Commands & Permissions

**Aliases:** `/bettertab`, `/btab` *(plus `/gatetab` quick help pointer)*

| Command | Permission | Description |
| :--- | :--- | :--- |
| `/bettertab reload` | `bettertab.reload` *(or `bettertab.admin`)* | Hot-reload `config/bettertab.toml` and refresh all viewers |
| `/nick <name>` / `/nickname <name>` | `bettertab.name` *(color tags require `bettertab.name.color`)* | Set a network nickname shown in the tab list and in chat on every server |
| `/nick reset` / `/nickname reset` | Player | Remove the network nickname and show the real username again |
| `/bettertab name <name>` | `bettertab.name` *(color tags require `bettertab.name.color`)* | Alias for `/nick <name>` |
| `/bettertab reset` | `bettertab.reset` *(or `bettertab.name`)* | Alias for `/nick reset` |
| `/bettertab toggle` | All | Toggle custom TAB formatting on/off |
| `/bettertab info` | All | Show active server, group, role, and ping |

---

## 📌 Placeholders

| Placeholder | Description |
| :--- | :--- |
| `%username%` / `%username_lower%` | Player username (or custom TAB name) / lowercase |
| `%server%` / `%raw_server%` | Backend name with / without `placeholder_replacements` |
| `%server_group%` | Active tab server group name |
| `%ping%` / `%ping_colored%` | Ping in ms (green `< 100ms`, yellow `< 200ms`, red `>= 200ms`) |
| `%players_online%` / `%max_players_online%` | Proxy-wide online count / configured max slots |
| `%local_players_online%` / `%server_online_players%` | Online players on the viewer's current backend |
| `%server_online_players_<srv>%` | Online players on specific backend `<srv>` |
| `%group_players_online%` / `%group_players_online_<grp>%` | Online players in current / named group `<grp>` |
| `%current_date%` / `%current_time%` | Current `YYYY-MM-DD` / `HH:mm:ss` *(plus `_day`, `_weekday`, `_month`, `_year`, `_hour`, `_minute`, `_second`)* |
| `%prefix%` / `%suffix%` / `%role%` / `%role_display_name%` / `%role_weight%` | Configured role metadata & padded weight |
| `%condition: A < B ? C : D%` | Conditional expression evaluation |

---

## 🌐 Minekube Gate Extensions Suite

| Project | Role | Stack |
| :--- | :--- | :--- |
| [`gate-stonyperms`](https://github.com/andreisugu/gate-stonyperms) | LuckPerms-style permissions, contexts, tracks & v0.2 R/W HTTP + SSE API | `Go 1.26` |
| [`stonyperms-bridge`](https://github.com/andreisugu/stonyperms-bridge) | Paper & Fabric permission adapters, live SSE, `/spb` commands, Vault & PAPI | `Java 25` |
| **[`gate-bettertab`](https://github.com/andreisugu/gate-bettertab)** *(this repo)* | Animated RGB / MiniMessage TAB lists, server groups, weights & placeholders | `Go 1.26` |
| [`gate-discord4gate`](https://github.com/andreisugu/gate-discord4gate) | Bidirectional Discord <> Minecraft chat, slash commands & webhook avatars | `Go 1.26` |
| [`gate-hybridforwarding`](https://github.com/andreisugu/gate-hybridforwarding) | Per-backend Velocity Modern, Legacy BungeeCord, BungeeGuard & None forwarding | `Go 1.26` |
| [`gate-simplewhitelist`](https://github.com/andreisugu/gate-simplewhitelist) | Lock-free `atomic.Pointer` whitelist by username, UUID, IP & CIDR | `Go 1.26` |
| [`gate-skinschanger`](https://github.com/andreisugu/gate-skinschanger) | Resilient multi-provider & Mineskin URL skin switcher with live backend refresh | `Go 1.26` |
| [`gate-smartlimbo`](https://github.com/andreisugu/gate-smartlimbo) | Limbo failover, protected sandbox & batched backend auto-reconnect queues | `Go 1.26` |
| [`gate-plugin-builder`](https://github.com/andreisugu/gate-plugin-builder) | Zero-config static Gate bundler (`build.sh`) + Pelican / Pterodactyl egg | `Shell` |

---

## 📄 License

Licensed under the [Apache License 2.0](LICENSE) © Andrei.
