package bettertab

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.minekube.com/gate/pkg/edition/java/proxy"
)

var conditionRegex = regexp.MustCompile(`(?i)%condition:\s*([^\?]+)\s*\?\s*([^:]+)\s*:\s*([^%]+)%`)
var serverCountRegex = regexp.MustCompile(`(?i)%server_online_players_([a-zA-Z0-9_\-]+)%`)
var groupCountRegex = regexp.MustCompile(`(?i)%group_players_online_([a-zA-Z0-9_\-]+)%`)

// PlaceholderContext provides metadata for placeholder evaluation.
type PlaceholderContext struct {
	Player           proxy.Player
	CustomName       string
	ServerName       string
	ProxyPlayerCount int
	MaxPlayers       int
	LocalPlayerCount int
	GroupPlayerCount int
	ServerCounts     map[string]int
	GroupCounts      map[string]int
	Role             Role
	RoleName         string
	Group            *Group
}

// ReplacePlaceholders evaluates and replaces all built-in, contextual, and conditional placeholders.
func ReplacePlaceholders(template string, ctx *PlaceholderContext) string {
	if template == "" {
		return ""
	}

	result := template

	// 1. Dynamic player values
	username := ""
	usernameLower := ""
	pingVal := 0
	pingStr := "0"

	if ctx.Player != nil {
		if ctx.CustomName != "" {
			username = ctx.CustomName
		} else {
			username = ctx.Player.Username()
		}
		usernameLower = strings.ToLower(username)
		pingMs := ctx.Player.Ping().Milliseconds()
		if pingMs > 0 {
			pingVal = int(pingMs)
			pingStr = strconv.Itoa(pingVal)
		}
	}

	// Colored ping placeholder
	pingColored := "<green>" + pingStr + "ms</green>"
	if pingVal >= 200 {
		pingColored = "<red>" + pingStr + "ms</red>"
	} else if pingVal >= 100 {
		pingColored = "<yellow>" + pingStr + "ms</yellow>"
	}

	serverName := ctx.ServerName
	if serverName == "" && ctx.Player != nil && ctx.Player.CurrentServer() != nil {
		serverName = ctx.Player.CurrentServer().Server().ServerInfo().Name()
	}

	groupName := "default"
	if ctx.Group != nil {
		groupName = ctx.Group.Name
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

	// 4. Parameterized server & group player count placeholders
	result = serverCountRegex.ReplaceAllStringFunc(result, func(m string) string {
		sub := serverCountRegex.FindStringSubmatch(m)
		if len(sub) > 1 && ctx.ServerCounts != nil {
			targetSrv := strings.ToLower(sub[1])
			return strconv.Itoa(ctx.ServerCounts[targetSrv])
		}
		return "0"
	})

	result = groupCountRegex.ReplaceAllStringFunc(result, func(m string) string {
		sub := groupCountRegex.FindStringSubmatch(m)
		if len(sub) > 1 && ctx.GroupCounts != nil {
			targetGrp := strings.ToLower(sub[1])
			return strconv.Itoa(ctx.GroupCounts[targetGrp])
		}
		return "0"
	})

	// 5. Substitute core placeholders
	result = strings.ReplaceAll(result, "%username%", username)
	result = strings.ReplaceAll(result, "%username_lower%", usernameLower)
	result = strings.ReplaceAll(result, "%server%", prettyServer)
	result = strings.ReplaceAll(result, "%raw_server%", serverName)
	result = strings.ReplaceAll(result, "%server_group%", groupName)
	result = strings.ReplaceAll(result, "%ping%", pingStr)
	result = strings.ReplaceAll(result, "%ping_colored%", pingColored)
	result = strings.ReplaceAll(result, "%players_online%", strconv.Itoa(ctx.ProxyPlayerCount))
	result = strings.ReplaceAll(result, "%max_players_online%", strconv.Itoa(ctx.MaxPlayers))
	result = strings.ReplaceAll(result, "%local_players_online%", strconv.Itoa(ctx.LocalPlayerCount))
	result = strings.ReplaceAll(result, "%server_online_players%", strconv.Itoa(ctx.LocalPlayerCount))
	result = strings.ReplaceAll(result, "%group_players_online%", strconv.Itoa(ctx.GroupPlayerCount))

	// Date & Time specific formats
	result = strings.ReplaceAll(result, "%current_date%", dateStr)
	result = strings.ReplaceAll(result, "%current_date_day%", now.Format("02"))
	result = strings.ReplaceAll(result, "%current_date_weekday%", now.Format("Monday"))
	result = strings.ReplaceAll(result, "%current_date_month%", now.Format("01"))
	result = strings.ReplaceAll(result, "%current_date_year%", now.Format("2006"))
	result = strings.ReplaceAll(result, "%current_time%", timeStr)
	result = strings.ReplaceAll(result, "%current_time_hour%", now.Format("15"))
	result = strings.ReplaceAll(result, "%current_time_minute%", now.Format("04"))
	result = strings.ReplaceAll(result, "%current_time_second%", now.Format("05"))

	// Role & permissions
	roleDisplayName := ctx.RoleName
	if roleDisplayName == "" {
		roleDisplayName = "Player"
	}
	result = strings.ReplaceAll(result, "%prefix%", ctx.Role.Prefix)
	result = strings.ReplaceAll(result, "%suffix%", ctx.Role.Suffix)
	result = strings.ReplaceAll(result, "%role%", ctx.RoleName)
	result = strings.ReplaceAll(result, "%role_display_name%", roleDisplayName)
	result = strings.ReplaceAll(result, "%role_weight%", fmt.Sprintf("%04d", ctx.Role.Weight))

	// 6. Conditional expressions: %condition: A < B ? TrueVal : FalseVal%
	result = conditionRegex.ReplaceAllStringFunc(result, func(m string) string {
		sub := conditionRegex.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		expr := strings.TrimSpace(sub[1])
		trueVal := strings.TrimSpace(sub[2])
		falseVal := strings.TrimSpace(sub[3])

		if evaluateCondition(expr) {
			return trueVal
		}
		return falseVal
	})

	return result
}

func evaluateCondition(expr string) bool {
	// Support ==, !=, >=, <=, >, <, contains
	if strings.Contains(expr, "==") {
		parts := strings.SplitN(expr, "==", 2)
		return strings.TrimSpace(parts[0]) == strings.TrimSpace(parts[1])
	}
	if strings.Contains(expr, "!=") {
		parts := strings.SplitN(expr, "!=", 2)
		return strings.TrimSpace(parts[0]) != strings.TrimSpace(parts[1])
	}
	if strings.Contains(expr, ">=") {
		parts := strings.SplitN(expr, ">=", 2)
		v1, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		v2, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil {
			return v1 >= v2
		}
	}
	if strings.Contains(expr, "<=") {
		parts := strings.SplitN(expr, "<=", 2)
		v1, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		v2, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil {
			return v1 <= v2
		}
	}
	if strings.Contains(expr, ">") {
		parts := strings.SplitN(expr, ">", 2)
		v1, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		v2, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil {
			return v1 > v2
		}
	}
	if strings.Contains(expr, "<") {
		parts := strings.SplitN(expr, "<", 2)
		v1, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		v2, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil {
			return v1 < v2
		}
	}
	return false
}
