package bettertab

import (
	"fmt"
	"regexp"
	"strings"

	guuid "github.com/google/uuid"
	"go.minekube.com/brigodier"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/util/uuid"

	stonyperms "github.com/andreisugu/gate-stonyperms"
)

const (
	// NicknameMetaKey is the StonyPerms user metadata key read by BetterTab and
	// by the Paper/Fabric bridges for chat and display names.
	NicknameMetaKey = "nickname"
	maxNicknameRaw  = 64
)

var (
	nickVisiblePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,16}$`)
	nickColorCode      = regexp.MustCompile(`(?i)&[0-9a-fk-or]`)
	nickColorCodes     = "0123456789abcdefklmnorABCDEFKLMNOR"

	reservedNicknames = map[string]struct{}{
		"admin":    {},
		"chaonius": {},
		"console":  {},
		"helper":   {},
		"mod":      {},
		"mojang":   {},
		"owner":    {},
		"server":   {},
		"staff":    {},
		"support":  {},
		"system":   {},
	}
)

// NicknameFor returns the global network nickname stored in StonyPerms user metadata.
func NicknameFor(id guuid.UUID) string {
	return NicknameForWithContext(id, nil)
}

// NicknameForWithContext returns the network nickname stored in StonyPerms user metadata for a context.
func NicknameForWithContext(id guuid.UUID, contexts map[string]string) string {
	mgr := stonyperms.GetManager()
	if mgr == nil {
		return ""
	}
	eff, _, _ := mgr.EffectiveUser(id, contexts, "default")
	if eff == nil {
		return ""
	}
	return strings.TrimSpace(eff.Meta[NicknameMetaKey])
}

// ResolveNickname returns the active nickname from StonyPerms or standalone PlayerStateManager.
func ResolveNickname(id uuid.UUID, contexts map[string]string, states *PlayerStateManager) string {
	if nick := NicknameForWithContext(guuid.UUID(id), contexts); nick != "" {
		return nick
	}
	if states != nil {
		if nick, ok := states.GetCustomName(id); ok {
			return nick
		}
	}
	return ""
}

func stripNickColors(value string) string {
	return nickColorCode.ReplaceAllString(value, "")
}

func hasNickColors(value string) bool {
	return strings.ContainsAny(value, "&§")
}

func normalizeColorCodes(value string) string {
	b := []byte(value)
	for i := 0; i+1 < len(b); i++ {
		if b[i] != '&' {
			continue
		}
		code := b[i+1]
		if strings.ContainsRune(nickColorCodes, rune(code)) {
			b[i+1] = byte(strings.ToLower(string(code))[0])
			i++
		}
	}
	return string(b)
}

func normalizeNickname(raw string, allowColor bool) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("name cannot be empty")
	}
	if len(value) > maxNicknameRaw {
		return "", fmt.Errorf("name is too long")
	}
	if strings.ContainsAny(value, "§<>{}\n\r\t") {
		return "", fmt.Errorf("use plain text with optional & color codes")
	}
	if hasNickColors(value) && !allowColor {
		return "", fmt.Errorf("your rank cannot use colors in a nickname")
	}
	visible := stripNickColors(value)
	if strings.Contains(visible, "&") {
		return "", fmt.Errorf("invalid color code")
	}
	if !nickVisiblePattern.MatchString(visible) {
		return "", fmt.Errorf("use 3-16 letters, numbers or underscores")
	}
	if _, reserved := reservedNicknames[strings.ToLower(visible)]; reserved {
		return "", fmt.Errorf("that name is reserved")
	}
	return normalizeColorCodes(value), nil
}

func nicknameConflict(p *proxy.Proxy, states *PlayerStateManager, selfID uuid.UUID, visible string) string {
	if mgr := stonyperms.GetManager(); mgr != nil {
		if other := mgr.GetUserByNameCopy(visible); other != nil && other.UUID != guuid.UUID(selfID) {
			return "that name belongs to another player"
		}
		users, _, _ := mgr.ListUsers("", 0, 100000)
		for _, other := range users {
			if other.UUID == guuid.UUID(selfID) {
				continue
			}
			otherVis := stripNickColors(strings.TrimSpace(other.Meta[NicknameMetaKey]))
			if otherVis != "" && strings.EqualFold(otherVis, visible) {
				return "that nickname is already in use"
			}
		}
	}

	if p != nil {
		for _, other := range p.Players() {
			if other.ID() == selfID {
				continue
			}
			if strings.EqualFold(other.Username(), visible) {
				return "that name belongs to another player"
			}
			otherNick := stripNickColors(ResolveNickname(other.ID(), nil, states))
			if otherNick != "" && strings.EqualFold(otherNick, visible) {
				return "that nickname is already in use"
			}
		}
	}
	return ""
}

func setTargetNickname(
	p *proxy.Proxy,
	states *PlayerStateManager,
	targetID uuid.UUID,
	targetUsername string,
	raw string,
	allowColor bool,
) (string, error) {
	value, err := normalizeNickname(raw, allowColor)
	if err != nil {
		return "", err
	}
	if conflict := nicknameConflict(p, states, targetID, stripNickColors(value)); conflict != "" {
		return "", fmt.Errorf("%s", conflict)
	}

	if mgr := stonyperms.GetManager(); mgr != nil {
		mgr.GetOrCreateUser(guuid.UUID(targetID), targetUsername)
		user := mgr.GetUserCopy(guuid.UUID(targetID))
		if user == nil {
			return "", fmt.Errorf("could not load the permission profile")
		}
		user.SetMeta(NicknameMetaKey, value)
		if err := mgr.SaveUser(user); err != nil {
			return "", fmt.Errorf("could not save the nickname: %v", err)
		}
		if states != nil {
			states.ResetCustomName(targetID)
		}
		return value, nil
	}

	if states != nil {
		states.SetCustomName(targetID, value)
		return value, nil
	}
	return "", fmt.Errorf("nickname storage is not available right now")
}

func clearTargetNickname(states *PlayerStateManager, targetID uuid.UUID) (bool, error) {
	cleared := false
	if mgr := stonyperms.GetManager(); mgr != nil {
		user := mgr.GetUserCopy(guuid.UUID(targetID))
		if user != nil && user.UnsetMeta(NicknameMetaKey) {
			if err := mgr.SaveUser(user); err != nil {
				return false, fmt.Errorf("could not clear the nickname: %v", err)
			}
			cleared = true
		}
	}
	if states != nil && states.ResetCustomName(targetID) {
		cleared = true
	}
	return cleared, nil
}

func sendNickInfo(player proxy.Player, states *PlayerStateManager) error {
	current := ResolveNickname(player.ID(), nil, states)
	if current == "" {
		return player.SendMessage(FormatText("&b[Nick] &7No nickname set. Use &f/nick <name>&7."))
	}
	return player.SendMessage(FormatText(fmt.Sprintf("&b[Nick] &7Current nickname: &f%s&7. Use &f/nick reset&7 to remove it.", current)))
}

func resolveWhois(p *proxy.Proxy, states *PlayerStateManager, query string) (string, string, bool) {
	clean := strings.TrimSpace(query)
	if clean == "" {
		return "", "", false
	}
	if p != nil {
		for _, online := range p.Players() {
			nick := ResolveNickname(online.ID(), nil, states)
			if strings.EqualFold(online.Username(), clean) ||
				(nick != "" && strings.EqualFold(stripNickColors(nick), clean)) {
				return online.Username(), nick, true
			}
		}
	}
	if mgr := stonyperms.GetManager(); mgr != nil {
		users, _, _ := mgr.ListUsers("", 0, 100000)
		for _, u := range users {
			nick := strings.TrimSpace(u.Meta[NicknameMetaKey])
			if strings.EqualFold(u.Username, clean) ||
				(nick != "" && strings.EqualFold(stripNickColors(nick), clean)) {
				return u.Username, nick, true
			}
		}
	}
	return "", "", false
}

func requireNickPermission(player proxy.Player) error {
	if player.HasPermission("bettertab.name") ||
		player.HasPermission("bettertab.admin") ||
		player.HasPermission("*") {
		return nil
	}
	return fmt.Errorf("you need the VIP rank or higher to set a nickname")
}

func canUseColorNick(src command.Source) bool {
	return src.HasPermission("bettertab.name.color") ||
		src.HasPermission("bettertab.admin") ||
		src.HasPermission("*")
}

func nickCommand(name string, p *proxy.Proxy, states *PlayerStateManager, tabMgr *TabManager) brigodier.LiteralNodeBuilder {
	apply := func(cmdCtx *command.Context) error {
		player, ok := cmdCtx.Source.(proxy.Player)
		if !ok {
			return cmdCtx.Source.SendMessage(FormatText("&cOnly players can set their own nickname."))
		}
		if err := requireNickPermission(player); err != nil {
			return cmdCtx.Source.SendMessage(FormatText("&c[Nick] " + err.Error() + "."))
		}
		value, err := setTargetNickname(p, states, player.ID(), player.Username(), cmdCtx.String("nickname"), canUseColorNick(player))
		if err != nil {
			return cmdCtx.Source.SendMessage(FormatText("&c[Nick] " + err.Error() + "."))
		}
		if tabMgr != nil {
			tabMgr.UpdateAll()
		}
		return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&a[Nick] Your network nickname is now &f%s&a. It shows in chat and the tab list on every server.", value)))
	}

	reset := func(cmdCtx *command.Context) error {
		player, ok := cmdCtx.Source.(proxy.Player)
		if !ok {
			return cmdCtx.Source.SendMessage(FormatText("&cOnly players can reset their own nickname."))
		}
		cleared, err := clearTargetNickname(states, player.ID())
		if err != nil {
			return cmdCtx.Source.SendMessage(FormatText("&c[Nick] " + err.Error() + "."))
		}
		if !cleared {
			return cmdCtx.Source.SendMessage(FormatText("&7[Nick] You do not have a nickname set."))
		}
		if tabMgr != nil {
			tabMgr.UpdateAll()
		}
		return cmdCtx.Source.SendMessage(FormatText("&a[Nick] Nickname cleared. Your real username is shown again."))
	}

	info := func(cmdCtx *command.Context) error {
		player, ok := cmdCtx.Source.(proxy.Player)
		if !ok {
			return cmdCtx.Source.SendMessage(FormatText("&b[Nick] &7Use &f/" + name + " <name>&7 in game."))
		}
		return sendNickInfo(player, states)
	}

	whois := func(cmdCtx *command.Context) error {
		query := cmdCtx.String("query")
		realName, nick, found := resolveWhois(p, states, query)
		if !found {
			return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&c[Nick] No player found matching &f%s&c.", query)))
		}
		if nick == "" {
			return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&b[Nick] &f%s &7does not have a custom nickname.", realName)))
		}
		return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&b[Nick] &f%s &7has real username &f%s&7.", nick, realName)))
	}

	return brigodier.Literal(name).
		Executes(command.Command(info)).
		Then(brigodier.Literal("info").Executes(command.Command(info))).
		Then(brigodier.Literal("reset").Executes(command.Command(reset))).
		Then(brigodier.Literal("clear").Executes(command.Command(reset))).
		Then(brigodier.Literal("whois").
			Requires(command.Requires(func(c *command.RequiresContext) bool {
				return c.Source.HasPermission("bettertab.whois") ||
					c.Source.HasPermission("bettertab.admin") ||
					c.Source.HasPermission("*")
			})).
			Then(brigodier.Argument("query", brigodier.String).
				Executes(command.Command(whois)))).
		Then(brigodier.Literal("set").
			Then(brigodier.Argument("nickname", brigodier.String).
				Executes(command.Command(apply)))).
		Then(brigodier.Argument("nickname", brigodier.String).
			Executes(command.Command(apply)))
}
