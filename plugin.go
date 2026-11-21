package bettertab

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"go.minekube.com/brigodier"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/util/uuid"

	stonyperms "github.com/andreisugu/gate-stonyperms"
)

// Plugin is the Gate BetterTab tablist & player list customization plugin.
var Plugin = proxy.Plugin{
	Name: "BetterTab",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		configPath := strings.TrimSpace(os.Getenv("BETTERTAB_CONFIG"))
		if configPath == "" {
			configPath = filepath.Join("config", "bettertab.toml")
		}

		store := NewConfigStore(configPath)
		if _, err := store.Load(); err != nil {
			log.Error(err, "Failed to load bettertab configuration")
			return err
		}

		states := NewPlayerStateManager()
		tabMgr := NewTabManager(p, store, states, log)
		tabMgr.Start(ctx)

		// 1. Update on server switches / joins
		event.Subscribe(p.Event(), 0, func(e *proxy.ServerPostConnectEvent) {
			sName := "unknown"
			if e.Player().CurrentServer() != nil {
				sName = e.Player().CurrentServer().Server().ServerInfo().Name()
			}
			tabMgr.HandleServerSwitch(e.Player(), sName)
		})

		// 2. Update on disconnects
		event.Subscribe(p.Event(), 0, func(e *proxy.DisconnectEvent) {
			tabMgr.HandlePlayerDisconnect(e.Player())
		})

		// ---------------------------------------------------------------------
		// 3. Register In-Game Commands: /bettertab, /btab, /nick, /nickname, /gatetab
		// ---------------------------------------------------------------------
		findTarget := func(targetName string) (uuid.UUID, string, bool) {
			if online := p.PlayerByName(targetName); online != nil {
				return online.ID(), online.Username(), true
			}
			if mgr := stonyperms.GetManager(); mgr != nil {
				if u := mgr.GetUserByNameCopy(targetName); u != nil {
					return uuid.UUID(u.UUID), u.Username, true
				}
			}
			return uuid.Nil, "", false
		}

		buildCommand := func(name string) brigodier.LiteralNodeBuilder {
			return brigodier.Literal(name).
				Executes(command.Command(func(cmdCtx *command.Context) error {
					return cmdCtx.Source.SendMessage(FormatText(
						"&b[BetterTab for Gate] &7Commands: " +
							"&f/nick <name>&7, &f/nick reset&7, &f/" + name + " reload&7, &f/" + name + " toggle&7, &f/" + name + " info",
					))
				})).
				Then(brigodier.Literal("reload").
					Requires(command.Requires(func(c *command.RequiresContext) bool {
						return c.Source.HasPermission("bettertab.reload") || c.Source.HasPermission("bettertab.admin") || c.Source.HasPermission("*")
					})).
					Executes(command.Command(func(cmdCtx *command.Context) error {
						if _, err := store.Load(); err != nil {
							return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&c[BetterTab] Reload failed: %v", err)))
						}
						tabMgr.UpdateAll()
						return cmdCtx.Source.SendMessage(FormatText("&a[BetterTab] Configuration reloaded successfully!"))
					})),
				).
				Then(brigodier.Literal("reset").
					Requires(command.Requires(func(c *command.RequiresContext) bool {
						return c.Source.HasPermission("bettertab.reset") || c.Source.HasPermission("bettertab.name") || c.Source.HasPermission("bettertab.admin") || c.Source.HasPermission("*")
					})).
					Executes(command.Command(func(cmdCtx *command.Context) error {
						player, ok := cmdCtx.Source.(proxy.Player)
						if !ok {
							return cmdCtx.Source.SendMessage(FormatText("&cOnly players can reset their own nickname."))
						}
						cleared, err := clearTargetNickname(states, player.ID())
						if err != nil {
							return cmdCtx.Source.SendMessage(FormatText("&c[Nick] " + err.Error() + "."))
						}
						if cleared {
							tabMgr.UpdateAll()
							return cmdCtx.Source.SendMessage(FormatText("&a[BetterTab] Your nickname has been reset."))
						}
						return cmdCtx.Source.SendMessage(FormatText("&e[BetterTab] You do not have a custom nickname set."))
					})),
				).
				Then(brigodier.Literal("toggle").
					Executes(command.Command(func(cmdCtx *command.Context) error {
						player, ok := cmdCtx.Source.(proxy.Player)
						if !ok {
							return cmdCtx.Source.SendMessage(FormatText("&cOnly players can toggle their TAB list."))
						}
						enabled := states.ToggleTab(player.ID())
						tabMgr.UpdateAll()
						if enabled {
							return cmdCtx.Source.SendMessage(FormatText("&a[BetterTab] Custom TAB list enabled."))
						}
						return cmdCtx.Source.SendMessage(FormatText("&e[BetterTab] Custom TAB list disabled."))
					})),
				).
				Then(brigodier.Literal("info").
					Executes(command.Command(func(cmdCtx *command.Context) error {
						player, ok := cmdCtx.Source.(proxy.Player)
						if !ok {
							cfg := store.GetConfig()
							return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf(
								"&b[BetterTab Status] &7Groups: &f%d&7, Online: &f%d", len(cfg.Groups), len(p.Players()),
							)))
						}
						sName := "none"
						if cs := player.CurrentServer(); cs != nil {
							sName = cs.Server().ServerInfo().Name()
						}
						group := store.GetGroupForServer(sName)
						gName := "none"
						if group != nil {
							gName = group.Name
						}
						var serverCtx map[string]string
						cleanServer := strings.ToLower(strings.TrimSpace(sName))
						if cleanServer != "" && cleanServer != "none" {
							serverCtx = map[string]string{"server": cleanServer}
						}
						role, rName, _ := tabMgr.ResolvePlayerRoleAndMeta(player, cleanServer)
						nick := ResolveNickname(player.ID(), serverCtx, states)
						if nick == "" {
							nick = "(none)"
						}

						infoMsg := fmt.Sprintf(
							"<aqua><b>[BetterTab Info]</b></aqua>\n"+
								"<gray>Player: <white>%s</white> (Nick: <white>%s</white>)</gray>\n"+
								"<gray>Server: <white>%s</white> (Group: <white>%s</white>)</gray>\n"+
								"<gray>Role: <white>%s</white> (Weight: <white>%d</white>)</gray>\n"+
								"<gray>Prefix: %s<white>%s</white>%s</gray>\n"+
								"<gray>Ping: <white>%dms</white></gray>",
							player.Username(), nick,
							sName, gName,
							rName, role.Weight,
							role.Prefix, player.Username(), role.Suffix,
							player.Ping().Milliseconds(),
						)
						return cmdCtx.Source.SendMessage(FormatText(infoMsg))
					})),
				).
				Then(brigodier.Literal("name").
					Requires(command.Requires(func(c *command.RequiresContext) bool {
						return c.Source.HasPermission("bettertab.name") || c.Source.HasPermission("bettertab.admin") || c.Source.HasPermission("*")
					})).
					Then(brigodier.Argument("new_name", brigodier.String).
						Executes(command.Command(func(cmdCtx *command.Context) error {
							player, ok := cmdCtx.Source.(proxy.Player)
							if !ok {
								return cmdCtx.Source.SendMessage(FormatText("&cOnly players can set their TAB name."))
							}
							newName := cmdCtx.String("new_name")
							value, err := setTargetNickname(p, states, player.ID(), player.Username(), newName, canUseColorNick(player))
							if err != nil {
								return cmdCtx.Source.SendMessage(FormatText("&c[Nick] " + err.Error() + "."))
							}
							tabMgr.UpdateAll()
							return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&a[Nick] Your network nickname is now &f%s&a.", value)))
						})),
					),
				).
				Then(brigodier.Literal("whois").
					Requires(command.Requires(func(c *command.RequiresContext) bool {
						return c.Source.HasPermission("bettertab.whois") || c.Source.HasPermission("bettertab.admin") || c.Source.HasPermission("*")
					})).
					Then(brigodier.Argument("query", brigodier.String).
						Executes(command.Command(func(cmdCtx *command.Context) error {
							query := cmdCtx.String("query")
							realName, nick, found := resolveWhois(p, states, query)
							if !found {
								return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&c[Nick] No player found matching &f%s&c.", query)))
							}
							if nick == "" {
								return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&b[Nick] &f%s &7does not have a custom nickname.", realName)))
							}
							return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&b[Nick] &f%s &7has real username &f%s&7.", nick, realName)))
						})),
					),
				).
				Then(brigodier.Literal("nick").
					Requires(command.Requires(func(c *command.RequiresContext) bool {
						return c.Source.HasPermission("bettertab.admin") || c.Source.HasPermission("*")
					})).
					Then(brigodier.Argument("player", brigodier.String).
						Then(brigodier.Literal("set").
							Then(brigodier.Argument("nickname", brigodier.String).
								Executes(command.Command(func(cmdCtx *command.Context) error {
									targetName := cmdCtx.String("player")
									targetID, realName, ok := findTarget(targetName)
									if !ok {
										return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&c[Nick] Player &f%s&c not found.", targetName)))
									}
									value, err := setTargetNickname(p, states, targetID, realName, cmdCtx.String("nickname"), true)
									if err != nil {
										return cmdCtx.Source.SendMessage(FormatText("&c[Nick] " + err.Error() + "."))
									}
									tabMgr.UpdateAll()
									return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&a[Nick] Set &f%s&a's nickname to &f%s&a.", realName, value)))
								})),
							),
						).
						Then(brigodier.Literal("reset").
							Executes(command.Command(func(cmdCtx *command.Context) error {
								targetName := cmdCtx.String("player")
								targetID, realName, ok := findTarget(targetName)
								if !ok {
									return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&c[Nick] Player &f%s&c not found.", targetName)))
								}
								cleared, err := clearTargetNickname(states, targetID)
								if err != nil {
									return cmdCtx.Source.SendMessage(FormatText("&c[Nick] " + err.Error() + "."))
								}
								if !cleared {
									return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&e[Nick] &f%s&e does not have a nickname set.", realName)))
								}
								tabMgr.UpdateAll()
								return cmdCtx.Source.SendMessage(FormatText(fmt.Sprintf("&a[Nick] Cleared &f%s&a's nickname.", realName)))
							})),
						),
					),
				)
		}

		p.Command().Register(buildCommand("bettertab"))
		p.Command().Register(buildCommand("btab"))
		p.Command().Register(nickCommand("nick", p, states, tabMgr))
		p.Command().Register(nickCommand("nickname", p, states, tabMgr))
		p.Command().Register(
			brigodier.Literal("gatetab").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					return cmdCtx.Source.SendMessage(FormatText("&bUse &f/bettertab &b(or &f/btab&b) for tab list commands."))
				})),
		)

		log.Info("BetterTab loaded (cross-server tablist, sorting, animations & nicknames active)")
		return nil
	},
}
