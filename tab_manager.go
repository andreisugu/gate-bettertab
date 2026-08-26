package bettertab

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"go.minekube.com/gate/pkg/util/uuid"
)

// TabManager manages the live state, animation ticks, and tablist synchronization for all players.
type TabManager struct {
	proxy     *proxy.Proxy
	store     *ConfigStore
	states    *PlayerStateManager
	log       logr.Logger
	animFrame atomic.Uint64
}

// NewTabManager creates a new TabManager instance.
func NewTabManager(p *proxy.Proxy, store *ConfigStore, states *PlayerStateManager, log logr.Logger) *TabManager {
	return &TabManager{
		proxy:  p,
		store:  store,
		states: states,
		log:    log,
	}
}

// Start begins the background update loop.
func (tm *TabManager) Start(ctx context.Context) {
	go func() {
		for {
			interval := 1000 * time.Millisecond
			if cfg := tm.store.GetConfig(); cfg != nil && cfg.UpdateIntervalMs > 0 {
				interval = time.Duration(cfg.UpdateIntervalMs) * time.Millisecond
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				tm.animFrame.Add(1)
				tm.UpdateAll()
			}
		}
	}()
}

// UpdateAll synchronizes headers, footers, display names, and list ordering across all active connections.
func (tm *TabManager) UpdateAll() {
	allPlayers := tm.proxy.Players()
	totalOnline := len(allPlayers)
	if totalOnline == 0 {
		return
	}

	maxPlayers := tm.proxy.Config().Status.ShowMaxPlayers
	cfg := tm.store.GetConfig()
	if cfg == nil {
		return
	}

	// 1. Calculate per-server and per-group counts
	serverCounts := make(map[string]int)
	groupCounts := make(map[string]int)
	playerServerMap := make(map[uuid.UUID]string)

	for _, p := range allPlayers {
		sName := "connecting"
		if cs := p.CurrentServer(); cs != nil {
			sName = cs.Server().ServerInfo().Name()
		}
		cleanSName := strings.ToLower(sName)
		serverCounts[cleanSName]++
		playerServerMap[p.ID()] = cleanSName

		group := tm.store.GetGroupForServer(cleanSName)
		if group != nil {
			groupCounts[strings.ToLower(group.Name)]++
		}
	}

	frame := tm.animFrame.Load()

	// 2. Update each viewer
	for _, viewer := range allPlayers {
		// Check if viewer has disabled custom tab list
		if tm.states.IsTabDisabled(viewer.ID()) {
			continue
		}

		viewerServer := playerServerMap[viewer.ID()]
		group := tm.store.GetGroupForServer(viewerServer)
		if group == nil {
			continue
		}

		viewerCustomName, _ := tm.states.GetCustomName(viewer.ID())
		viewerRole, viewerRoleName := tm.store.GetRoleWithInfo(viewer.Username())

		viewerCtx := &PlaceholderContext{
			Player:           viewer,
			CustomName:       viewerCustomName,
			ServerName:       viewerServer,
			ProxyPlayerCount: totalOnline,
			MaxPlayers:       maxPlayers,
			LocalPlayerCount: serverCounts[viewerServer],
			GroupPlayerCount: groupCounts[strings.ToLower(group.Name)],
			ServerCounts:     serverCounts,
			GroupCounts:      groupCounts,
			Role:             viewerRole,
			RoleName:         viewerRoleName,
			Group:            group,
		}

		// --- Update Animated Header & Footer ---
		var headerRaw, footerRaw string
		if len(group.Headers) > 0 {
			hIdx := int(frame % uint64(len(group.Headers)))
			headerRaw = group.Headers[hIdx]
		}
		if len(group.Footers) > 0 {
			fIdx := int(frame % uint64(len(group.Footers)))
			footerRaw = group.Footers[fIdx]
		}

		headerFormatted := ReplacePlaceholders(headerRaw, viewerCtx)
		footerFormatted := ReplacePlaceholders(footerRaw, viewerCtx)

		_ = viewer.TabList().SetHeaderFooter(
			FormatText(headerFormatted),
			FormatText(footerFormatted),
		)

		// --- Update Player Entries in Tab List ---
		entries := viewer.TabList().Entries()

		for _, target := range allPlayers {
			targetServer := playerServerMap[target.ID()]

			// Check server isolation mode
			if group.OnlyListPlayersInSameServer && !strings.EqualFold(viewerServer, targetServer) {
				if entry, exists := entries[target.ID()]; exists {
					_ = entry.SetListed(false)
				}
				continue
			}

			entry, exists := entries[target.ID()]
			if !exists {
				continue
			}

			_ = entry.SetListed(true)

			targetCustomName, _ := tm.states.GetCustomName(target.ID())
			targetRole, targetRoleName := tm.store.GetRoleWithInfo(target.Username())

			targetCtx := &PlaceholderContext{
				Player:           target,
				CustomName:       targetCustomName,
				ServerName:       targetServer,
				ProxyPlayerCount: totalOnline,
				MaxPlayers:       maxPlayers,
				LocalPlayerCount: serverCounts[targetServer],
				GroupPlayerCount: groupCounts[strings.ToLower(group.Name)],
				ServerCounts:     serverCounts,
				GroupCounts:      groupCounts,
				Role:             targetRole,
				RoleName:         targetRoleName,
				Group:            group,
			}

			// Format display name with placeholders & colors
			displayNameRaw := ReplacePlaceholders(group.Format, targetCtx)
			_ = entry.SetDisplayName(FormatText(displayNameRaw))

			// Sort order (higher role weight = lower list order number = appears higher in list)
			if cfg.SortPlayers {
				order := 10000 - targetRole.Weight
				if order < 0 {
					order = 0
				}
				_ = entry.SetListOrder(order)
			}
		}
	}
}

// HandleServerSwitch immediately updates tablist state upon switching servers.
func (tm *TabManager) HandleServerSwitch(player proxy.Player, targetServer string) {
	tm.UpdateAll()
}

// HandlePlayerDisconnect cleans up state and updates remaining players upon logout.
func (tm *TabManager) HandlePlayerDisconnect(player proxy.Player) {
	tm.states.RemovePlayer(player.ID())
	tm.UpdateAll()
}
