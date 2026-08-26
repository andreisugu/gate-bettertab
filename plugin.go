package velocitab

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

// Plugin is the Gate Velocitab tablist & player list customization plugin.
var Plugin = proxy.Plugin{
	Name: "Velocitab",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		configPath := filepath.Join("config", "velocitab.toml")

		store := NewConfigStore(configPath)
		if _, err := store.Load(); err != nil {
			log.Error(err, "Failed to load velocitab configuration")
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
		// 3. Register In-Game Commands: /velocitab and /gatetab
		// ---------------------------------------------------------------------
		buildCommand := func() brigodier.LiteralNodeBuilder {
			return brigodier.Literal("velocitab").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					return cmdCtx.Source.SendMessage(&c.Text{
						Content: "§b[Velocitab for Gate] §7by Andrei\n" +
							"§7Commands: §f/velocitab reload§7, §f/velocitab name <name>§7, §f/velocitab reset§7, §f/velocitab toggle§7, §f/velocitab info",
					})
				})).
				Then(brigodier.Literal("reload").
					Executes(command.Command(func(cmdCtx *command.Context) error {
						if _, err := store.Load(); err != nil {
							return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[Velocitab] Reload failed: %v", err)})
						}
						tabMgr.UpdateAll()
						return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[Velocitab] Configuration reloaded successfully!"})
					})),
				).
				Then(brigodier.Literal("reset").
					Executes(command.Command(func(cmdCtx *command.Context) error {
						player, ok := cmdCtx.Source.(proxy.Player)
						if !ok {
							return cmdCtx.Source.SendMessage(&c.Text{Content: "§cOnly players can reset their TAB name."})
						}
						if states.ResetCustomName(player.ID()) {
							tabMgr.UpdateAll()
							return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[Velocitab] Your TAB display name has been reset."})
						}
						return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[Velocitab] You do not have a custom TAB name set."})
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
							return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[Velocitab] Custom TAB list enabled."})
						}
						return cmdCtx.Source.SendMessage(&c.Text{Content: "§e[Velocitab] Custom TAB list disabled."})
					})),
				).
				Then(brigodier.Literal("info").
					Executes(command.Command(func(cmdCtx *command.Context) error {
						player, ok := cmdCtx.Source.(proxy.Player)
						if !ok {
							cfg := store.GetConfig()
							return cmdCtx.Source.SendMessage(&c.Text{
								Content: fmt.Sprintf("§b[Velocitab Status] §7Groups: %d, Online: %d", len(cfg.Groups), len(p.Players())),
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
							Content: fmt.Sprintf("§b[Velocitab Info]\n§7Server: §f%s\n§7Group: §f%s\n§7Role: §f%s\n§7Ping: §f%dms",
								sName, gName, rName, player.Ping().Milliseconds()),
						})
					})),
				).
				Then(brigodier.Literal("name").
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
							states.SetCustomName(player.ID(), newName)
							tabMgr.UpdateAll()
							return cmdCtx.Source.SendMessage(&c.Text{
								Content: fmt.Sprintf("§a[Velocitab] Your TAB display name is now: §f%s", newName),
							})
						})),
					),
				)
		}

		p.Command().Register(buildCommand())
		p.Command().Register(
			brigodier.Literal("gatetab").
				Executes(command.Command(func(cmdCtx *command.Context) error {
					return cmdCtx.Source.SendMessage(&c.Text{
						Content: "§bUse §f/velocitab §bfor tab list commands.",
					})
				})),
		)

		log.Info("Velocitab loaded (custom tablist formatting, animations & groups active)")
		return nil
	},
}
