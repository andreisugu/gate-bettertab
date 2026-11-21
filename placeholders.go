package bettertab

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.minekube.com/gate/pkg/edition/java/proxy"
)

var (
	conditionRegex   = regexp.MustCompile(`(?i)%condition:\s*([^\?]+)\s*\?\s*([^:]*)\s*:\s*([^%]*)%`)
	serverCountRegex = regexp.MustCompile(`(?i)%server_online_players_([a-zA-Z0-9_\-]+)%`)
	groupCountRegex  = regexp.MustCompile(`(?i)%group_players_online_([a-zA-Z0-9_\-]+)%`)
	metaRegex        = regexp.MustCompile(`(?i)%meta_([a-zA-Z0-9_.\-]+)%`)
)

// PlaceholderContext provides metadata for placeholder evaluation.
type PlaceholderContext struct {
	Player           proxy.Player
	RealUsername     string
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
	Meta             map[string]string
}

// ReplacePlaceholders evaluates and replaces all built-in, contextual, and conditional placeholders.
func ReplacePlaceholders(template string, ctx *PlaceholderContext) string {
	if template == "" {
		return ""
	}
	if ctx == nil {
		ctx = &PlaceholderContext{}
	}

	result := template

	// 1. Dynamic player values
	realUsername := ctx.RealUsername
	if realUsername == "" && ctx.Player != nil {
		realUsername = ctx.Player.Username()
	}
	displayUsername := realUsername
	if ctx.CustomName != "" {
		displayUsername = ctx.CustomName
	}
	nickName := displayUsername
	usernameLower := strings.ToLower(stripNickColors(displayUsername))
	realUsernameLower := strings.ToLower(realUsername)

	pingVal := 0
	pingStr := "0"
	if ctx.Player != nil {
		pingMs := ctx.Player.Ping().Milliseconds()
		if pingMs > 0 {
			pingVal = int(pingMs)
			pingStr = strconv.Itoa(pingVal)
		}
	}

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
	if ctx.Group != nil && ctx.Group.Name != "" {
		groupName = ctx.Group.Name
	}

	roleDisplayName := strings.TrimSpace(ctx.Role.DisplayName)
	if roleDisplayName == "" {
		roleDisplayName = strings.TrimSpace(ctx.RoleName)
	}
	if roleDisplayName == "" {
		roleDisplayName = "Player"
	}

	// 2. Custom placeholder replacements (case-insensitive across any configured key)
	if ctx.Group != nil && len(ctx.Group.PlaceholderReplacements) > 0 {
		rawLookup := map[string]string{
			"%server%":            serverName,
			"%raw_server%":        serverName,
			"%server_group%":      groupName,
			"%role%":              ctx.RoleName,
			"%role_display_name%": roleDisplayName,
			"%username%":          displayUsername,
			"%real_username%":     realUsername,
			"%nick%":              nickName,
			"%nickname%":          nickName,
		}
		for phKey, valMap := range ctx.Group.PlaceholderReplacements {
			lowerKey := strings.ToLower(strings.TrimSpace(phKey))
			currentVal, ok := rawLookup[lowerKey]
			if !ok {
				if metaKey, isMeta := strings.CutPrefix(lowerKey, "%meta_"); isMeta && strings.HasSuffix(metaKey, "%") {
					currentVal = lookupMetaValue(ctx.Meta, strings.TrimSuffix(metaKey, "%"))
					ok = true
				}
			}
			if !ok {
				continue
			}
			if replacement, found := lookupReplacementCaseInsensitive(valMap, currentVal); found {
				result = replacePlaceholderCaseInsensitive(result, phKey, replacement)
			}
		}
	}

	// 3. Time formatting
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("15:04:05")

	// 4. Parameterized server, group & StonyPerms meta placeholders
	result = serverCountRegex.ReplaceAllStringFunc(result, func(m string) string {
		sub := serverCountRegex.FindStringSubmatch(m)
		if len(sub) > 1 && ctx.ServerCounts != nil {
			targetSrv := strings.ToLower(sub[1])
			if count, ok := lookupIntCaseInsensitive(ctx.ServerCounts, targetSrv); ok {
				return strconv.Itoa(count)
			}
		}
		return "0"
	})

	result = groupCountRegex.ReplaceAllStringFunc(result, func(m string) string {
		sub := groupCountRegex.FindStringSubmatch(m)
		if len(sub) > 1 && ctx.GroupCounts != nil {
			targetGrp := strings.ToLower(sub[1])
			if count, ok := lookupIntCaseInsensitive(ctx.GroupCounts, targetGrp); ok {
				return strconv.Itoa(count)
			}
		}
		return "0"
	})

	result = metaRegex.ReplaceAllStringFunc(result, func(m string) string {
		sub := metaRegex.FindStringSubmatch(m)
		if len(sub) > 1 {
			return lookupMetaValue(ctx.Meta, sub[1])
		}
		return ""
	})

	// 5. Substitute core placeholders
	result = strings.ReplaceAll(result, "%username%", displayUsername)
	result = strings.ReplaceAll(result, "%username_lower%", usernameLower)
	result = strings.ReplaceAll(result, "%real_username%", realUsername)
	result = strings.ReplaceAll(result, "%real_username_lower%", realUsernameLower)
	result = strings.ReplaceAll(result, "%nick%", nickName)
	result = strings.ReplaceAll(result, "%nickname%", nickName)
	result = strings.ReplaceAll(result, "%server%", serverName)
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
	result = strings.ReplaceAll(result, "%prefix%", ctx.Role.Prefix)
	result = strings.ReplaceAll(result, "%suffix%", ctx.Role.Suffix)
	result = strings.ReplaceAll(result, "%role%", ctx.RoleName)
	result = strings.ReplaceAll(result, "%role_display_name%", roleDisplayName)
	result = strings.ReplaceAll(result, "%role_weight%", fmt.Sprintf("%04d", ctx.Role.Weight))
	result = strings.ReplaceAll(result, "%weight%", strconv.Itoa(ctx.Role.Weight))

	// 6. Conditional expressions: %condition: A <op> B ? TrueVal : FalseVal%
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

func lookupMetaValue(meta map[string]string, key string) string {
	if len(meta) == 0 || key == "" {
		return ""
	}
	if val, ok := meta[key]; ok {
		return val
	}
	for k, v := range meta {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func lookupReplacementCaseInsensitive(m map[string]string, key string) (string, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return "", false
}

func lookupIntCaseInsensitive(m map[string]int, key string) (int, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return 0, false
}

func replacePlaceholderCaseInsensitive(s, ph, replacement string) string {
	if strings.Contains(s, ph) {
		return strings.ReplaceAll(s, ph, replacement)
	}
	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(ph))
	if err != nil {
		return s
	}
	return re.ReplaceAllString(s, replacement)
}

func trimConditionOperand(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func evaluateCondition(expr string) bool {
	switch {
	case strings.Contains(expr, "=="):
		parts := strings.SplitN(expr, "==", 2)
		return trimConditionOperand(parts[0]) == trimConditionOperand(parts[1])
	case strings.Contains(expr, "!="):
		parts := strings.SplitN(expr, "!=", 2)
		return trimConditionOperand(parts[0]) != trimConditionOperand(parts[1])
	case strings.Contains(expr, "~="):
		parts := strings.SplitN(expr, "~=", 2)
		return strings.EqualFold(trimConditionOperand(parts[0]), trimConditionOperand(parts[1]))
	case strings.Contains(strings.ToLower(expr), " contains "):
		idx := strings.Index(strings.ToLower(expr), " contains ")
		left := trimConditionOperand(expr[:idx])
		right := trimConditionOperand(expr[idx+len(" contains "):])
		return strings.Contains(strings.ToLower(left), strings.ToLower(right))
	case strings.Contains(expr, ">="):
		parts := strings.SplitN(expr, ">=", 2)
		v1, err1 := strconv.ParseFloat(trimConditionOperand(parts[0]), 64)
		v2, err2 := strconv.ParseFloat(trimConditionOperand(parts[1]), 64)
		return err1 == nil && err2 == nil && v1 >= v2
	case strings.Contains(expr, "<="):
		parts := strings.SplitN(expr, "<=", 2)
		v1, err1 := strconv.ParseFloat(trimConditionOperand(parts[0]), 64)
		v2, err2 := strconv.ParseFloat(trimConditionOperand(parts[1]), 64)
		return err1 == nil && err2 == nil && v1 <= v2
	case strings.Contains(expr, ">"):
		parts := strings.SplitN(expr, ">", 2)
		v1, err1 := strconv.ParseFloat(trimConditionOperand(parts[0]), 64)
		v2, err2 := strconv.ParseFloat(trimConditionOperand(parts[1]), 64)
		return err1 == nil && err2 == nil && v1 > v2
	case strings.Contains(expr, "<"):
		parts := strings.SplitN(expr, "<", 2)
		v1, err1 := strconv.ParseFloat(trimConditionOperand(parts[0]), 64)
		v2, err2 := strconv.ParseFloat(trimConditionOperand(parts[1]), 64)
		return err1 == nil && err2 == nil && v1 < v2
	}
	return false
}
