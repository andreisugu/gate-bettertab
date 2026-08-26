package bettertab

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"go.minekube.com/brigodier"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// Plugin is the Gate BetterTab tablist & player list customization plugin.
var Plugin = proxy.Plugin{
	Name: "BetterTab",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		configPath := filepath.Join("config", "bettertab.toml")

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
		// 3. Register In-Game Commands: /bettertab, /btab, /tab, /gatetab
		// ---------------------------------------------------------------------
		buildCommand := func(name string) brigodier.LiteralNodeBuilder {
			return brigodier.Literal(name).
				Executes(command.Command(func(cmdCtx *command.Context) error {
					return cmdCtx.Source.SendMessage(&c.Text{
						Content: "§b[BetterTab for Gate] §7by Andrei\n" +
							"§7Commands: §f/" + name + " reload§7, §f/" + name + " name <name>§7, §f/" + name + " reset§7, §f/" + name + " toggle§7, §f/" + name + " info",
					})
				})).
				Then(brigodier.Literal("reload").
					Requires(command.Requires(func(c *command.RequiresContext) bool {
						return c.Source.HasPermission("bettertab.admin")
					})).
					Executes(command.Command(func(cmdCtx *command.Context) error {
						if _, err := store.Load(); err != nil {
							return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[BetterTab] Reload failed: %v", err)})
						}
						tabMgr.UpdateAll()
						return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[BetterTab] Configuration reloaded successfully!"})
					})),
				).
				Then(brigodier.Literal("reset").
					Requires(command.Requires(func(c *command.RequiresContext) bool {
						return c.Source.HasPermission("bettertab.name")
					})).
					Executes(command.Command(func(cmdCtx *command.Context) error {
						player, ok := cmdCtx.Source.(proxy.Player)
						if !ok {
							return cmdCtx.Source.SendMessage(&c.Text{Content: "§cOnly players can reset their TAB name."})
						}
						if states.ResetCustomName(player.ID()) {
							tabMgr.UpdateAll()
							return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[BetterTab] Your TAB display name has been reset."})
						}
						return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[BetterTab] You do not have a custom TAB name set."})
					})),
				).
				Then(brigodier.Literal("toggle").
					Executes(command.Command(func(cmdCtx *command.Context) error {
						player, ok := cmdCtx.Source.(proxy.Player)
						if !ok {
							return cmdCtx.Source.SendMessage(&c.Text{Content: "§cOnly players can toggle their TAB list."})
						}
						enabled := states.ToggleTab(player.ID())
						tabMgr.UpdateAll()
						if enabled {
							return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[BetterTab] Custom TAB list enabled."})
						}
						return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[BetterTab] Custom TAB list disabled."})
					})),
				).
				Then(brigodier.Literal("info").
					Executes(command.Command(func(cmdCtx *command.Context) error {
						player, ok := cmdCtx.Source.(proxy.Player)
						if !ok {
							cfg := store.GetConfig()
							return cmdCtx.Source.SendMessage(&c.Text{
								Content: fmt.Sprintf("§b[BetterTab Status] §7Groups: %d, Online: %d", len(cfg.Groups), len(p.Players())),
							})
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
						_, rName := store.GetRoleWithInfo(player.Username())

						return cmdCtx.Source.SendMessage(&c.Text{
							Content: fmt.Sprintf("§b[BetterTab Info]\n§7Server: §f%s\n§7Group: §f%s\n§7Role: §f%s\n§7Ping: §f%dms",
								sName, gName, rName, player.Ping().Milliseconds()),
						})
					})),
				).
				Then(brigodier.Literal("name").
					Requires(command.Requires(func(c *command.RequiresContext) bool {
						return c.Source.HasPermission("bettertab.name")
					})).
					Then(brigodier.Argument("new_name", brigodier.String).
						Executes(command.Command(func(cmdCtx *command.Context) error {
							player, ok := cmdCtx.Source.(proxy.Player)
							if !ok {
								return cmdCtx.Source.SendMessage(&c.Text{Content: "§cOnly players can set their TAB name."})
							}
							newName := cmdCtx.String("new_name")
							if strings.TrimSpace(newName) == "" {
								return cmdCtx.Source.SendMessage(&c.Text{Content: "§cName cannot be empty."})
							}
							if (strings.Contains(newName, "&") || strings.Contains(newName, "§") || strings.Contains(newName, "<")) && !player.HasPermission("bettertab.name.color") {
								return cmdCtx.Source.SendMessage(&c.Text{Content: "§cYou do not have permission to use color or formatting tags in your TAB name (bettertab.name.color)."})
							}
							states.SetCustomName(player.ID(), newName)
							tabMgr.UpdateAll()
							return cmdCtx.Source.SendMessage(&c.Text{
								Content: fmt.Sprintf("§a[BetterTab] Your TAB display name is now: §f%s", newName),
							})
						})),
					),
				)
		}

		p.Command().Register(buildCommand("bettertab"))
		p.Command().Register(buildCommand("btab"))
		p.Command().Register(
			brigodier.Literal("gatetab").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					return cmdCtx.Source.SendMessage(&c.Text{
						Content: "§bUse §f/bettertab §b(or §f/btab§b) for tab list commands.",
					})
				})),
		)

		log.Info("BetterTab loaded (custom tablist formatting, animations & groups active)")
		return nil
	},
}
