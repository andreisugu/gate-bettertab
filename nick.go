package bettertab

import (
	"fmt"
	"regexp"
	"strings"

	guuid "github.com/google/uuid"
	"go.minekube.com/brigodier"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"

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

// NicknameFor returns the network nickname stored in StonyPerms user metadata.
func NicknameFor(id guuid.UUID) string {
	mgr := stonyperms.GetManager()
	if mgr == nil {
		return ""
	}
	eff, _, _ := mgr.EffectiveUser(id, nil, "default")
	if eff == nil {
		return ""
	}
	return strings.TrimSpace(eff.Meta[NicknameMetaKey])
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

func nicknameConflict(p *proxy.Proxy, self proxy.Player, visible string) string {
	mgr := stonyperms.GetManager()
	if mgr == nil {
		return "permissions are still loading, try again in a moment"
	}

	if other := mgr.GetUserByNameCopy(visible); other != nil && other.UUID != guuid.UUID(self.ID()) {
		return "that name belongs to another player"
	}

	users, _, _ := mgr.ListUsers("", 0, 100000)
	for _, other := range users {
		if other.UUID == guuid.UUID(self.ID()) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(other.Meta[NicknameMetaKey]), visible) {
			return "that nickname is already in use"
		}
	}

	if p != nil {
		for _, other := range p.Players() {
			if other.ID() == self.ID() {
				continue
			}
			if strings.EqualFold(other.Username(), visible) {
				return "that name belongs to another player"
			}
			if strings.EqualFold(NicknameFor(guuid.UUID(other.ID())), visible) {
				return "that nickname is already in use"
			}
		}
	}
	return ""
}

func setNickname(p *proxy.Proxy, player proxy.Player, raw string) (string, error) {
	allowColor := player.HasPermission("bettertab.name.color") ||
		player.HasPermission("bettertab.admin") ||
		player.HasPermission("*")

	value, err := normalizeNickname(raw, allowColor)
	if err != nil {
		return "", err
	}
	if conflict := nicknameConflict(p, player, stripNickColors(value)); conflict != "" {
		return "", fmt.Errorf("%s", conflict)
	}

	mgr := stonyperms.GetManager()
	if mgr == nil {
		return "", fmt.Errorf("permissions are still loading, try again in a moment")
	}
	mgr.GetOrCreateUser(guuid.UUID(player.ID()), player.Username())
	user := mgr.GetUserCopy(guuid.UUID(player.ID()))
	if user == nil {
		return "", fmt.Errorf("could not load your permission profile")
	}
	user.SetMeta(NicknameMetaKey, value)
	if err := mgr.SaveUser(user); err != nil {
		return "", fmt.Errorf("could not save the nickname: %v", err)
	}
	return value, nil
}

func clearNickname(player proxy.Player) (bool, error) {
	mgr := stonyperms.GetManager()
	if mgr == nil {
		return false, fmt.Errorf("permissions are still loading, try again in a moment")
	}
	user := mgr.GetUserCopy(guuid.UUID(player.ID()))
	if user == nil || !user.UnsetMeta(NicknameMetaKey) {
		return false, nil
	}
	if err := mgr.SaveUser(user); err != nil {
		return false, fmt.Errorf("could not save the nickname: %v", err)
	}
	return true, nil
}

func sendNickInfo(player proxy.Player) error {
	current := NicknameFor(guuid.UUID(player.ID()))
	if current == "" {
		return player.SendMessage(&c.Text{
			Content: "§b[Nick] §7No nickname set. Use §f/nick <name>§7.",
		})
	}
	return player.SendMessage(&c.Text{
		Content: fmt.Sprintf("§b[Nick] §7Current nickname: §f%s§7. Use §f/nick reset§7 to remove it.", current),
	})
}

func requireNickPermission(player proxy.Player) error {
	if player.HasPermission("bettertab.name") ||
		player.HasPermission("bettertab.admin") ||
		player.HasPermission("*") {
		return nil
	}
	return fmt.Errorf("you need the VIP rank or higher to set a nickname")
}

func nickCommand(name string, p *proxy.Proxy) brigodier.LiteralNodeBuilder {
	apply := func(cmdCtx *command.Context) error {
		player, ok := cmdCtx.Source.(proxy.Player)
		if !ok {
			return cmdCtx.Source.SendMessage(&c.Text{Content: "§cOnly players can set a nickname."})
		}
		if err := requireNickPermission(player); err != nil {
			return cmdCtx.Source.SendMessage(&c.Text{Content: "§c[Nick] " + err.Error() + "."})
		}
		value, err := setNickname(p, player, cmdCtx.String("nickname"))
		if err != nil {
			return cmdCtx.Source.SendMessage(&c.Text{Content: "§c[Nick] " + err.Error() + "."})
		}
		return cmdCtx.Source.SendMessage(&c.Text{
			Content: fmt.Sprintf("§a[Nick] Your network nickname is now §f%s§a. It shows in chat and the tab list on every server.", value),
		})
	}

	reset := func(cmdCtx *command.Context) error {
		player, ok := cmdCtx.Source.(proxy.Player)
		if !ok {
			return cmdCtx.Source.SendMessage(&c.Text{Content: "§cOnly players can reset a nickname."})
		}
		cleared, err := clearNickname(player)
		if err != nil {
			return cmdCtx.Source.SendMessage(&c.Text{Content: "§c[Nick] " + err.Error() + "."})
		}
		if !cleared {
			return cmdCtx.Source.SendMessage(&c.Text{Content: "§7[Nick] You do not have a nickname set."})
		}
		return cmdCtx.Source.SendMessage(&c.Text{
			Content: "§a[Nick] Nickname cleared. Your real username is shown again.",
		})
	}

	info := func(cmdCtx *command.Context) error {
		player, ok := cmdCtx.Source.(proxy.Player)
		if !ok {
			return cmdCtx.Source.SendMessage(&c.Text{Content: "§b[Nick] §7Use §f/" + name + " <name>§7 in game."})
		}
		return sendNickInfo(player)
	}

	return brigodier.Literal(name).
		Executes(command.Command(info)).
		Then(brigodier.Literal("info").Executes(command.Command(info))).
		Then(brigodier.Literal("reset").Executes(command.Command(reset))).
		Then(brigodier.Literal("clear").Executes(command.Command(reset))).
		Then(brigodier.Literal("set").
			Then(brigodier.Argument("nickname", brigodier.String).
				Executes(command.Command(apply)))).
		Then(brigodier.Argument("nickname", brigodier.String).
			Executes(command.Command(apply)))
}
