package velocitab

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// PlaceholderContext provides contextual metadata for placeholder evaluation.
type PlaceholderContext struct {
	Player             proxy.Player
	ServerName         string
	ProxyPlayerCount   int
	MaxPlayers         int
	LocalPlayerCount   int
	GroupPlayerCount   int
	Role               Role
	Group              *Group
}

// ReplacePlaceholders replaces all built-in and group-specific placeholders in a template.
func ReplacePlaceholders(template string, ctx *PlaceholderContext) string {
	if template == "" {
		return ""
	}

	result := template

	// 1. Dynamic player values
	username := ""
	usernameLower := ""
	pingStr := "0"

	if ctx.Player != nil {
		username = ctx.Player.Username()
		usernameLower = strings.ToLower(username)
		pingMs := ctx.Player.Ping().Milliseconds()
		if pingMs < 0 {
			pingMs = 0
		}
		pingStr = strconv.FormatInt(pingMs, 10)
	}

	serverName := ctx.ServerName
	if serverName == "" && ctx.Player != nil && ctx.Player.CurrentServer() != nil {
		serverName = ctx.Player.CurrentServer().Server().ServerInfo().Name()
	}

	// 2. Custom placeholder replacements (e.g. pretty server badges)
	prettyServer := serverName
	if ctx.Group != nil && ctx.Group.PlaceholderReplacements != nil {
		if serverReplacements, ok := ctx.Group.PlaceholderReplacements["%server%"]; ok {
			if replacement, found := serverReplacements[serverName]; found {
				prettyServer = replacement
			}
		}
	}

	// 3. Time formatting
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("15:04:05")

	// 4. Substitute placeholders
	result = strings.ReplaceAll(result, "%username%", username)
	result = strings.ReplaceAll(result, "%username_lower%", usernameLower)
	result = strings.ReplaceAll(result, "%server%", prettyServer)
	result = strings.ReplaceAll(result, "%raw_server%", serverName)
	result = strings.ReplaceAll(result, "%ping%", pingStr)
	result = strings.ReplaceAll(result, "%players_online%", strconv.Itoa(ctx.ProxyPlayerCount))
	result = strings.ReplaceAll(result, "%max_players_online%", strconv.Itoa(ctx.MaxPlayers))
	result = strings.ReplaceAll(result, "%local_players_online%", strconv.Itoa(ctx.LocalPlayerCount))
	result = strings.ReplaceAll(result, "%group_players_online%", strconv.Itoa(ctx.GroupPlayerCount))
	result = strings.ReplaceAll(result, "%current_date%", dateStr)
	result = strings.ReplaceAll(result, "%current_time%", timeStr)
	result = strings.ReplaceAll(result, "%prefix%", ctx.Role.Prefix)
	result = strings.ReplaceAll(result, "%suffix%", ctx.Role.Suffix)
	result = strings.ReplaceAll(result, "%role_weight%", fmt.Sprintf("%04d", ctx.Role.Weight))

	return result
}
