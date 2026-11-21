<div align="center">

# 🌈 Gate BetterTab

**Cross-server TAB list, MiniMessage/RGB stack parser, Minecraft 1.21.2+ / 26.3 priority sorting, network `/nick` & live StonyPerms role integration for Minekube Gate.**

<p>
  <a href="https://github.com/andreisugu/gate-bettertab/actions/workflows/ci.yml"><img src="https://github.com/andreisugu/gate-bettertab/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/andreisugu/gate-bettertab/releases"><img src="https://img.shields.io/github/v/release/andreisugu/gate-bettertab?style=flat-square&label=Release" alt="Latest Release" /></a>
  <a href="https://pkg.go.dev/github.com/andreisugu/gate-bettertab"><img src="https://pkg.go.dev/badge/github.com/andreisugu/gate-bettertab.svg" alt="Go Reference" /></a>
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26" />
  <a href="https://github.com/minekube/gate"><img src="https://img.shields.io/badge/Platform-Minekube_Gate-7c3aed?style=flat-square" alt="Minekube Gate" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg?style=flat-square" alt="Apache 2.0" /></a>
</p>

<p>
  <a href="#-features">Features</a> •
  <a href="#-installation">Installation</a> •
  <a href="#-stonyperms-integration--sorting">StonyPerms & Sorting</a> •
  <a href="#-configuration">Configuration</a> •
  <a href="#-commands--permissions">Commands</a> •
  <a href="#-placeholders">Placeholders</a> •
  <a href="#-minekube-gate-extensions-suite">Gate Suite</a>
</p>

</div>

---

## ✨ Features

- **Cross-Server Global Tab List:** Automatically mirrors players across all connected backends (with real skin profiles, head layers, ping buckets, and clean removal on disconnect or server-isolated groups), synced immediately on server switch, disconnect, and StonyPerms updates.
- **True Minecraft 1.21.2+ / 26.3 Priority Sorting:** Evaluates multi-key `sorting_placeholders` (`%role_weight%`, `%username_lower%`, `-` / `+` / `desc:` / `asc:` modifiers) and emits positive descending `ListOrder` values so higher weights and alphabetical nicknames stay in exact order.
- **Zero-Spam Per-Viewer Diff Cache & Clean Toggle:** Only emits header/footer/entry update packets when rendered values change, and cleanly restores vanilla tab entries and headers when toggled off via `/bettertab toggle`.
- **Stack-Accurate MiniMessage, RGB & Bungee Hex Parser:** Preserves nested style and color stacks (`<gold><b>NETWORK</b> Proxy</gold>`), bold/italic inside `<rainbow>` & phase-animated `<gradient>`, `<click>` / `<hover:show_text:...>` tags, `&#RRGGBB`, and `&x&R&R&G&G&B&B` legacy codes.
- **Live `gate-stonyperms` (`v0.3.0`) + Standalone Role Integration:** Dynamically resolves prefixes, suffixes, weights, display names, and `%meta_<key>%` from StonyPerms `EffectiveUser` snapshots (with contextual `server=<name>` evaluation), alongside unified `/nick` & `/bettertab` storage and automatic `[roles]` fallback when StonyPerms is not installed.
- **Live Hot-Reload (`BETTERTAB_CONFIG`):** Reload `config/bettertab.toml` (or custom `BETTERTAB_CONFIG`) on the fly via `/bettertab reload`.

---

## 📦 Installation

### Using `gate-plugin-builder` / Pelican Egg
Add to your `plugins.txt` or `GATE_PLUGINS`:
```text
github.com/andreisugu/gate-bettertab
```

### In Go Code
```bash
go get github.com/andreisugu/gate-bettertab@v0.2.0
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

## 🔗 StonyPerms Integration & Sorting

- **Dynamic StonyPerms Role Sync:** When `gate-stonyperms` (`v0.3.0+`) is loaded in the proxy, BetterTab subscribes to StonyPerms updates and resolves `%prefix%`, `%suffix%`, `%role%`, `%role_display_name%`, `%role_weight%`, `%weight%`, `%meta_<key>%`, and network nicknames cleanly against live SQL/JSON storage without proxy restarts. If StonyPerms is absent, BetterTab falls back transparently to `[roles]` and `[player_roles]`.
- **Server Group Matching:** Each `[[groups]]` block matches connected backend names via exact names (`"lobby"`), catch-all wildcard (`"*"`), globs (`"lobby_*"`), or regex patterns (`"r=^smp-[0-9]+$"`).
- **Sorting Modifiers:** `sorting_placeholders` evaluates keys top-to-bottom. Prefix keys with `-` / `desc:` for descending order or `+` / `asc:` for ascending order (numeric fields such as `%role_weight%` default descending unless overridden; string fields such as `%username_lower%` sort alphabetically).

---

## ⚙️ Configuration

Generated at `config/bettertab.toml` (override via `BETTERTAB_CONFIG`):

```toml
update_interval_ms = 1000
sort_players = true
default_group = "default"

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

[player_roles]
# andrei = "owner"

[[groups]]
name = "default"
servers = ["*"] # Supports exact names, "*" wildcard, globs ("lobby_*"), or regexes ("r=^smp-[0-9]+$")
only_list_players_in_same_server = false
sorting_placeholders = ["%role_weight%", "%username_lower%"]

headers = [
    "<rainbow:!0><b>═══ Minecraft Proxy Network ═══</b></rainbow>\n<gray>Welcome back, <white>%username%</white>!</gray>",
    "<rainbow:!2><b>═══ Minecraft Proxy Network ═══</b></rainbow>\n<gray>Welcome back, <white>%username%</white>!</gray>",
    "<rainbow:!4><b>═══ Minecraft Proxy Network ═══</b></rainbow>\n<gray>Server: <aqua>%server%</aqua> | Ping: %ping_colored%</gray>",
    "<rainbow:!6><b>═══ Minecraft Proxy Network ═══</b></rainbow>\n<gray>Server: <aqua>%server%</aqua> | Ping: %ping_colored%</gray>",
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

**Aliases:** `/bettertab`, `/btab`, `/nick`, `/nickname` *(plus `/gatetab` quick help pointer)*

| Command | Permission | Description |
| :--- | :--- | :--- |
| `/bettertab reload` | `bettertab.reload` *(or `bettertab.admin`)* | Hot-reload `config/bettertab.toml` and refresh all viewers |
| `/nick <name>` / `/nickname <name>` | `bettertab.name` *(color tags require `bettertab.name.color`)* | Set a network nickname shown in tab list and chat |
| `/nick reset` / `/nickname reset` | Player | Remove your network nickname and show real username again |
| `/nick whois <name>` / `/bettertab whois <name>` | `bettertab.whois` *(or `bettertab.admin`)* | Look up the real username behind a nickname or vice versa |
| `/bettertab nick <player> <set <nick>\|reset>` | `bettertab.admin` | Set or reset another player's nickname |
| `/bettertab name <name>` / `/bettertab reset` | `bettertab.name` / `bettertab.reset` | Aliases for `/nick <name>` and `/nick reset` |
| `/bettertab toggle` | All | Toggle custom TAB formatting on/off cleanly |
| `/bettertab info` | All | Show active server, group, role, nickname, rendered prefix & ping |

---

## 📌 Placeholders

| Placeholder | Description |
| :--- | :--- |
| `%username%` / `%username_lower%` | Active display name (nickname if set, else real username) / lowercase |
| `%real_username%` / `%real_username_lower%` | Real Mojang/offline username / lowercase |
| `%nick%` / `%nickname%` | Active nickname (or real username if none) |
| `%server%` / `%raw_server%` | Backend name with / without `placeholder_replacements` |
| `%server_group%` | Active tab server group name |
| `%ping%` / `%ping_colored%` | Ping in ms (green `< 100ms`, yellow `< 200ms`, red `>= 200ms`) |
| `%players_online%` / `%max_players_online%` | Proxy-wide online count / configured max slots |
| `%local_players_online%` / `%server_online_players%` | Online players on the viewer's current backend |
| `%server_online_players_<srv>%` | Online players on specific backend `<srv>` |
| `%group_players_online%` / `%group_players_online_<grp>%` | Online players in current / named group `<grp>` |
| `%current_date%` / `%current_time%` | Current `YYYY-MM-DD` / `HH:mm:ss` *(plus `_day`, `_weekday`, `_month`, `_year`, `_hour`, `_minute`, `_second`)* |
| `%prefix%` / `%suffix%` / `%role%` / `%role_display_name%` / `%role_weight%` / `%weight%` | Resolved StonyPerms/config role metadata, padded weight & raw weight |
| `%meta_<key>%` | StonyPerms resolved user/group metadata value for `<key>` |
| `%condition: A <op> B ? C : D%` | Conditional expression evaluation (`==`, `!=`, `~=`, `contains`, `>=`, `<=`, `>`, `<`) |

---

## 🌐 Minekube Gate Extensions Suite

| Project | Role | Stack |
| :--- | :--- | :--- |
| [`gate-stonyperms`](https://github.com/andreisugu/gate-stonyperms) | Context-aware permissions, SQL/JSON storage, Web UI & v0.2 HTTP + SSE API | `Go 1.26` |
| [`stonyperms-bridge`](https://github.com/andreisugu/stonyperms-bridge) | Paper & Fabric standalone/bridge permissions, SQL/JSON, `/spb` & `/sp`, Vault & PAPI | `Java 25` |
| **[`gate-bettertab`](https://github.com/andreisugu/gate-bettertab)** *(this repo)* | Animated RGB / MiniMessage TAB lists, 1.21.2+ sorting, `/nick` & StonyPerms roles | `Go 1.26` |
| [`gate-discord4gate`](https://github.com/andreisugu/gate-discord4gate) | Bidirectional Discord <> Minecraft chat, slash commands & webhook avatars | `Go 1.26` |
| [`gate-hybridforwarding`](https://github.com/andreisugu/gate-hybridforwarding) | Per-backend Velocity Modern, Legacy BungeeCord, BungeeGuard & None forwarding | `Go 1.26` |
| [`gate-simplewhitelist`](https://github.com/andreisugu/gate-simplewhitelist) | Lock-free `atomic.Pointer` whitelist by username, UUID, IP & CIDR | `Go 1.26` |
| [`gate-skinschanger`](https://github.com/andreisugu/gate-skinschanger) | Resilient multi-provider & Mineskin URL skin switcher with live backend refresh | `Go 1.26` |
| [`gate-smartlimbo`](https://github.com/andreisugu/gate-smartlimbo) | Limbo failover, protected sandbox & batched backend auto-reconnect queues | `Go 1.26` |
| [`gate-plugin-builder`](https://github.com/andreisugu/gate-plugin-builder) | Zero-config static Gate bundler (`build.sh`) + Pelican / Pterodactyl egg | `Shell` |

---

## 📄 License

Licensed under the [Apache License 2.0](LICENSE) © Andrei.
