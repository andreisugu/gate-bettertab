package velocitab

import (
	"context"
	"fmt"
	"path/filepath"

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

		tabMgr := NewTabManager(p, store, log)
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

		// 3. Register commands: /velocitab reload and /gatetab reload
		reloadCommand := command.Command(func(cmdCtx *command.Context) error {
			if _, err := store.Load(); err != nil {
				return cmdCtx.Source.SendMessage(&c.Text{Content: fmt.Sprintf("§c[Velocitab] Reload failed: %v", err)})
			}
			tabMgr.UpdateAll()
			return cmdCtx.Source.SendMessage(&c.Text{Content: "§a[Velocitab] Configuration reloaded successfully!"})
		})

		p.Command().Register(
			brigodier.Literal("velocitab").
				Then(brigodier.Literal("reload").Executes(reloadCommand)),
		)

		p.Command().Register(
			brigodier.Literal("gatetab").
				Then(brigodier.Literal("reload").Executes(reloadCommand)),
		)

		log.Info("Velocitab loaded (custom tablist formatting, animations & groups active)")
		return nil
	},
}
