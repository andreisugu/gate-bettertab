package bettertab

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	guuid "github.com/google/uuid"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/profile"
	"go.minekube.com/gate/pkg/edition/java/proto/version"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	gateplayer "go.minekube.com/gate/pkg/edition/java/proxy/player"
	"go.minekube.com/gate/pkg/edition/java/proxy/tablist"
	"go.minekube.com/gate/pkg/util/uuid"

	stonyperms "github.com/andreisugu/gate-stonyperms"
)

type playerSnapshot struct {
	player        proxy.Player
	id            uuid.UUID
	realUsername  string
	customName    string
	serverName    string
	group         *Group
	role          Role
	roleName      string
	meta          map[string]string
	ping          time.Duration
	latencyBucket int
	profile       profile.GameProfile
}

type syncedEntryState struct {
	displayNameRaw string
	listOrder      int
	listed         bool
	latencyBucket  int
	synthetic      bool
}

type viewerTabState struct {
	serverName      string
	disabledCleaned bool
	header          string
	footer          string
	entries         map[uuid.UUID]syncedEntryState
}

// TabManager manages the live state, animation ticks, and tablist synchronization for all players.
type TabManager struct {
	proxy        *proxy.Proxy
	store        *ConfigStore
	states       *PlayerStateManager
	log          logr.Logger
	animFrame    atomic.Uint64
	eventsSub    atomic.Bool
	updateMu     sync.Mutex
	viewerStates map[uuid.UUID]*viewerTabState
}

// NewTabManager creates a new TabManager instance.
func NewTabManager(p *proxy.Proxy, store *ConfigStore, states *PlayerStateManager, log logr.Logger) *TabManager {
	return &TabManager{
		proxy:        p,
		store:        store,
		states:       states,
		log:          log,
		viewerStates: make(map[uuid.UUID]*viewerTabState),
	}
}

// Start begins the background update loop.
func (tm *TabManager) Start(ctx context.Context) {
	tm.ensureStonyPermsSubscribed(ctx)
	go func() {
		for {
			tm.ensureStonyPermsSubscribed(ctx)
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

// UpdateAll synchronizes headers, footers, display names, cross-server entries, and list ordering across all active viewers.
func (tm *TabManager) UpdateAll() {
	tm.updateMu.Lock()
	defer tm.updateMu.Unlock()

	defer func() {
		_ = recover()
	}()

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

	// 1. Collect per-player role, metadata, nickname, and server counts in a single O(N) pass.
	serverCounts := make(map[string]int, 8)
	groupCounts := make(map[string]int, 8)
	snapshots := make([]playerSnapshot, 0, totalOnline)
	snapByID := make(map[uuid.UUID]*playerSnapshot, totalOnline)

	for _, p := range allPlayers {
		sName := "connecting"
		if cs := p.CurrentServer(); cs != nil {
			sName = cs.Server().ServerInfo().Name()
		}
		cleanSName := strings.ToLower(strings.TrimSpace(sName))
		if cleanSName == "" {
			cleanSName = "connecting"
		}
		serverCounts[cleanSName]++

		group := tm.store.GetGroupForServer(cleanSName)
		if group != nil {
			groupCounts[strings.ToLower(group.Name)]++
		}

		role, roleName, meta := tm.ResolvePlayerRoleAndMeta(p, cleanSName)
		var ctxMap map[string]string
		if cleanSName != "connecting" && cleanSName != "none" && cleanSName != "unknown" {
			ctxMap = map[string]string{"server": cleanSName}
		}
		customName := ResolveNickname(p.ID(), ctxMap, tm.states)
		ping := p.Ping()
		gp := p.GameProfile()
		if gp.ID == uuid.Nil {
			gp.ID = p.ID()
		}
		if gp.Name == "" {
			gp.Name = p.Username()
		}

		snapshots = append(snapshots, playerSnapshot{
			player:        p,
			id:            p.ID(),
			realUsername:  p.Username(),
			customName:    customName,
			serverName:    cleanSName,
			group:         group,
			role:          role,
			roleName:      roleName,
			meta:          meta,
			ping:          ping,
			latencyBucket: latencyBucketFor(ping),
			profile:       gp,
		})
	}
	for i := range snapshots {
		snapByID[snapshots[i].id] = &snapshots[i]
	}

	// Sort orders lazily cached by group sorting rules per tick.
	sortedCache := make(map[string][]playerSnapshot, 4)
	getSortedForGroup := func(g *Group) []playerSnapshot {
		rules := g.SortingPlaceholders
		if len(rules) == 0 {
			rules = []string{"%role_weight%", "%username_lower%"}
		}
		cacheKey := strings.Join(rules, "\x00")
		if cached, ok := sortedCache[cacheKey]; ok {
			return cached
		}
		ordered := make([]playerSnapshot, len(snapshots))
		copy(ordered, snapshots)
		sortPlayerSnapshots(ordered, rules, serverCounts, groupCounts, totalOnline, maxPlayers)
		sortedCache[cacheKey] = ordered
		return ordered
	}

	frame := tm.animFrame.Load()

	// 2. Sync each active viewer.
	for i := range snapshots {
		viewerSnap := &snapshots[i]
		viewer := viewerSnap.player
		if !viewer.Active() {
			continue
		}

		vs := tm.viewerStates[viewer.ID()]
		if vs == nil {
			vs = &viewerTabState{
				serverName: viewerSnap.serverName,
				entries:    make(map[uuid.UUID]syncedEntryState, totalOnline),
			}
			tm.viewerStates[viewer.ID()] = vs
		} else if vs.serverName != viewerSnap.serverName {
			// Reset cached header/footer/entry state across backend switches so new server transitions resync cleanly.
			vs.serverName = viewerSnap.serverName
			vs.header = ""
			vs.footer = ""
			clear(vs.entries)
		}

		tl := viewer.TabList()
		entries := tl.Entries()

		// Cleanly restore vanilla tab state if viewer disabled BetterTab via /bettertab toggle.
		if tm.states.IsTabDisabled(viewer.ID()) {
			if !vs.disabledCleaned {
				tm.clearDisabledViewerTab(viewer, tl, entries, vs)
			}
			continue
		}
		vs.disabledCleaned = false

		// Remove entries for players who disconnected from the proxy.
		tm.pruneOfflineEntries(viewer, tl, entries, vs, snapByID)

		group := viewerSnap.group
		if group == nil {
			continue
		}

		viewerCtx := buildPlaceholderContext(viewerSnap, group, serverCounts, groupCounts, totalOnline, maxPlayers)

		var headerRaw, footerRaw string
		if len(group.Headers) > 0 {
			headerRaw = group.Headers[int(frame%uint64(len(group.Headers)))]
		}
		if len(group.Footers) > 0 {
			footerRaw = group.Footers[int(frame%uint64(len(group.Footers)))]
		}
		headerFormatted := ReplacePlaceholders(headerRaw, viewerCtx)
		footerFormatted := ReplacePlaceholders(footerRaw, viewerCtx)
		if headerFormatted != vs.header || footerFormatted != vs.footer {
			if err := tl.SetHeaderFooter(FormatText(headerFormatted), FormatText(footerFormatted)); err == nil {
				vs.header = headerFormatted
				vs.footer = footerFormatted
			}
		}

		orderedTargets := getSortedForGroup(group)
		needFlush := false

		for rankIdx := range orderedTargets {
			targetSnap := &orderedTargets[rankIdx]
			sameServer := strings.EqualFold(viewerSnap.serverName, targetSnap.serverName)
			existingEntry, exists := entries[targetSnap.id]
			prevSynced, hasPrev := vs.entries[targetSnap.id]

			// Hide/remove cross-server players when only_list_players_in_same_server is enabled.
			if group.OnlyListPlayersInSameServer && !sameServer {
				if exists {
					if prevSynced.synthetic {
						if err := tl.RemoveAll(targetSnap.id); err == nil {
							needFlush = true
						}
						delete(vs.entries, targetSnap.id)
					} else if !hasPrev || prevSynced.listed {
						_ = existingEntry.SetListed(false)
						vs.entries[targetSnap.id] = syncedEntryState{listed: false}
					}
				} else {
					delete(vs.entries, targetSnap.id)
				}
				continue
			}

			order := 0
			if cfg.SortPlayers {
				order = (len(orderedTargets) - rankIdx) * 10
			}

			targetCtx := buildPlaceholderContext(targetSnap, group, serverCounts, groupCounts, totalOnline, maxPlayers)
			displayNameRaw := ReplacePlaceholders(group.Format, targetCtx)

			// Case 1: Entry not present in viewer's TabList -> inject synthetic cross-server entry.
			if !exists {
				displayComp := FormatText(displayNameRaw)
				lat := targetSnap.ping
				if lat < 0 {
					lat = 0
				}
				synth := newProxyTabEntry(tl, targetSnap.profile, displayComp, lat, order)
				if err := tl.Add(synth); err == nil {
					vs.entries[targetSnap.id] = syncedEntryState{
						displayNameRaw: displayNameRaw,
						listOrder:      order,
						listed:         true,
						latencyBucket:  targetSnap.latencyBucket,
						synthetic:      true,
					}
				}
				continue
			}

			// Case 2: Entry already synthetic in viewer's TabList -> delta-update via TabList.Add only if changed.
			if _, isSynth := existingEntry.(*proxyTabEntry); isSynth {
				changed := !hasPrev || !prevSynced.synthetic || !prevSynced.listed ||
					prevSynced.displayNameRaw != displayNameRaw ||
					prevSynced.listOrder != order ||
					prevSynced.latencyBucket != targetSnap.latencyBucket
				if !changed {
					continue
				}
				displayComp := FormatText(displayNameRaw)
				lat := targetSnap.ping
				if lat < 0 {
					lat = 0
				}
				fresh := newProxyTabEntry(tl, targetSnap.profile, displayComp, lat, order)
				var err error
				if viewer.Protocol().GreaterEqual(version.Minecraft_1_19_3) {
					err = tl.Add(fresh)
				} else {
					_ = tl.RemoveAll(targetSnap.id)
					err = tl.Add(fresh)
				}
				if err == nil {
					vs.entries[targetSnap.id] = syncedEntryState{
						displayNameRaw: displayNameRaw,
						listOrder:      order,
						listed:         true,
						latencyBucket:  targetSnap.latencyBucket,
						synthetic:      true,
					}
				}
				continue
			}

			// Case 3: Real backend-managed entry -> delta-update only changed attributes.
			if !hasPrev || prevSynced.synthetic || !prevSynced.listed {
				if !existingEntry.Listed() {
					_ = existingEntry.SetListed(true)
				}
			}
			if !hasPrev || prevSynced.displayNameRaw != displayNameRaw || existingEntry.DisplayName() == nil {
				_ = existingEntry.SetDisplayName(FormatText(displayNameRaw))
			}
			if !hasPrev || prevSynced.listOrder != order || existingEntry.ListOrder() != order {
				_ = existingEntry.SetListOrder(order)
			}
			vs.entries[targetSnap.id] = syncedEntryState{
				displayNameRaw: displayNameRaw,
				listOrder:      order,
				listed:         true,
				latencyBucket:  targetSnap.latencyBucket,
				synthetic:      false,
			}
		}

		if needFlush {
			_ = viewer.Flush()
		}
	}
}

func (tm *TabManager) clearDisabledViewerTab(
	viewer proxy.Player,
	tl tablist.TabList,
	entries map[uuid.UUID]tablist.Entry,
	vs *viewerTabState,
) {
	_ = tablist.ClearTabListHeaderFooter(tl)

	var removeIDs []uuid.UUID
	for id, synced := range vs.entries {
		entry, exists := entries[id]
		if !exists {
			continue
		}
		if synced.synthetic {
			removeIDs = append(removeIDs, id)
			continue
		}
		if entry.DisplayName() != nil {
			_ = entry.SetDisplayName(nil)
		}
		if entry.ListOrder() != 0 {
			_ = entry.SetListOrder(0)
		}
		if !entry.Listed() {
			_ = entry.SetListed(true)
		}
	}
	if len(removeIDs) > 0 {
		_ = tl.RemoveAll(removeIDs...)
		_ = viewer.Flush()
	}

	vs.header = ""
	vs.footer = ""
	clear(vs.entries)
	vs.disabledCleaned = true
}

func (tm *TabManager) pruneOfflineEntries(
	viewer proxy.Player,
	tl tablist.TabList,
	entries map[uuid.UUID]tablist.Entry,
	vs *viewerTabState,
	online map[uuid.UUID]*playerSnapshot,
) {
	var removeIDs []uuid.UUID
	for id, synced := range vs.entries {
		if _, isOnline := online[id]; isOnline {
			continue
		}
		if synced.synthetic {
			if _, exists := entries[id]; exists {
				removeIDs = append(removeIDs, id)
			}
		}
		delete(vs.entries, id)
	}
	if len(removeIDs) > 0 {
		_ = tl.RemoveAll(removeIDs...)
		_ = viewer.Flush()
	}
}

func buildPlaceholderContext(
	snap *playerSnapshot,
	viewerGroup *Group,
	serverCounts map[string]int,
	groupCounts map[string]int,
	totalOnline int,
	maxPlayers int,
) *PlaceholderContext {
	grpCount := 0
	if viewerGroup != nil {
		grpCount = groupCounts[strings.ToLower(viewerGroup.Name)]
	}
	return &PlaceholderContext{
		Player:           snap.player,
		RealUsername:     snap.realUsername,
		CustomName:       snap.customName,
		ServerName:       snap.serverName,
		ProxyPlayerCount: totalOnline,
		MaxPlayers:       maxPlayers,
		LocalPlayerCount: serverCounts[snap.serverName],
		GroupPlayerCount: grpCount,
		ServerCounts:     serverCounts,
		GroupCounts:      groupCounts,
		Role:             snap.role,
		RoleName:         snap.roleName,
		Group:            viewerGroup,
		Meta:             snap.meta,
	}
}

func latencyBucketFor(d time.Duration) int {
	ms := d.Milliseconds()
	switch {
	case ms < 0:
		return -1
	case ms < 150:
		return 5
	case ms < 300:
		return 4
	case ms < 600:
		return 3
	case ms < 1000:
		return 2
	default:
		return 1
	}
}

type parsedSortRule struct {
	template   string
	descending bool
}

func parseSortRule(raw string) parsedSortRule {
	rule := strings.TrimSpace(raw)
	explicitDir := false
	desc := false

	switch {
	case strings.HasPrefix(rule, "-"):
		rule = strings.TrimSpace(rule[1:])
		desc = true
		explicitDir = true
	case strings.HasPrefix(rule, "+"):
		rule = strings.TrimSpace(rule[1:])
		desc = false
		explicitDir = true
	case len(rule) >= 5 && strings.EqualFold(rule[:5], "desc:"):
		rule = strings.TrimSpace(rule[5:])
		desc = true
		explicitDir = true
	case len(rule) >= 4 && strings.EqualFold(rule[:4], "asc:"):
		rule = strings.TrimSpace(rule[4:])
		desc = false
		explicitDir = true
	}

	if !explicitDir {
		lower := strings.ToLower(rule)
		switch lower {
		case "%role_weight%", "%weight%", "%players_online%", "%local_players_online%",
			"%server_online_players%", "%group_players_online%":
			desc = true
		}
	}
	return parsedSortRule{template: rule, descending: desc}
}

func sortPlayerSnapshots(
	snaps []playerSnapshot,
	rawRules []string,
	serverCounts map[string]int,
	groupCounts map[string]int,
	totalOnline int,
	maxPlayers int,
) {
	if len(snaps) <= 1 {
		return
	}
	if len(rawRules) == 0 {
		rawRules = []string{"%role_weight%", "%username_lower%"}
	}

	rules := make([]parsedSortRule, 0, len(rawRules))
	for _, r := range rawRules {
		if strings.TrimSpace(r) != "" {
			rules = append(rules, parseSortRule(r))
		}
	}

	type keyedSnap struct {
		snap playerSnapshot
		keys []string
	}
	keyed := make([]keyedSnap, len(snaps))
	for i := range snaps {
		ctx := buildPlaceholderContext(&snaps[i], snaps[i].group, serverCounts, groupCounts, totalOnline, maxPlayers)
		vals := make([]string, len(rules))
		for j, r := range rules {
			if strings.EqualFold(r.template, "%role_weight%") || strings.EqualFold(r.template, "%weight%") {
				vals[j] = strconv.Itoa(snaps[i].role.Weight)
			} else {
				vals[j] = strings.TrimSpace(ReplacePlaceholders(r.template, ctx))
			}
		}
		keyed[i] = keyedSnap{snap: snaps[i], keys: vals}
	}

	sort.SliceStable(keyed, func(i, j int) bool {
		a, b := keyed[i], keyed[j]
		for k, rule := range rules {
			ka, kb := a.keys[k], b.keys[k]
			if ka == kb {
				continue
			}
			na, errA := strconv.ParseFloat(ka, 64)
			nb, errB := strconv.ParseFloat(kb, 64)
			if errA == nil && errB == nil {
				if na != nb {
					if rule.descending {
						return na > nb
					}
					return na < nb
				}
				continue
			}
			la, lb := strings.ToLower(ka), strings.ToLower(kb)
			if la != lb {
				if rule.descending {
					return la > lb
				}
				return la < lb
			}
		}
		ua, ub := strings.ToLower(a.snap.realUsername), strings.ToLower(b.snap.realUsername)
		if ua != ub {
			return ua < ub
		}
		return a.snap.id.String() < b.snap.id.String()
	})

	for i := range keyed {
		snaps[i] = keyed[i].snap
	}
}

// HandleServerSwitch immediately updates tablist state upon switching servers.
func (tm *TabManager) HandleServerSwitch(_ proxy.Player, _ string) {
	tm.UpdateAll()
}

// HandlePlayerDisconnect cleans up state and updates remaining players upon logout.
func (tm *TabManager) HandlePlayerDisconnect(player proxy.Player) {
	tm.states.RemovePlayer(player.ID())
	tm.updateMu.Lock()
	delete(tm.viewerStates, player.ID())
	tm.updateMu.Unlock()
	tm.UpdateAll()
}

// ResolvePlayerRole resolves a player's Role and role name dynamically via StonyPerms
// when loaded, falling back to static BetterTab configuration.
func (tm *TabManager) ResolvePlayerRole(p proxy.Player, serverName string) (Role, string) {
	role, roleName, _ := tm.ResolvePlayerRoleAndMeta(p, serverName)
	return role, roleName
}

// ResolvePlayerRoleAndMeta resolves Role, role name, and StonyPerms metadata in one lookup.
func (tm *TabManager) ResolvePlayerRoleAndMeta(p proxy.Player, serverName string) (Role, string, map[string]string) {
	if p == nil {
		return Role{Weight: 0}, "default", nil
	}
	if mgr := stonyperms.GetManager(); mgr != nil {
		var contexts map[string]string
		cleanServer := strings.ToLower(strings.TrimSpace(serverName))
		if cleanServer != "" && cleanServer != "connecting" && cleanServer != "none" && cleanServer != "unknown" {
			contexts = map[string]string{"server": cleanServer}
		}
		eff, _, _ := mgr.EffectiveUser(guuid.UUID(p.ID()), contexts, "default")
		role, roleName := tm.store.ResolveRoleForUser(p.Username(), eff)
		var meta map[string]string
		if eff != nil && len(eff.Meta) > 0 {
			meta = eff.Meta
		}
		return role, roleName, meta
	}
	role, roleName := tm.store.GetRoleWithInfo(p.Username())
	return role, roleName, nil
}

func (tm *TabManager) ensureStonyPermsSubscribed(ctx context.Context) {
	if tm.eventsSub.Load() {
		return
	}
	mgr := stonyperms.GetManager()
	if mgr == nil {
		return
	}
	if !tm.eventsSub.CompareAndSwap(false, true) {
		return
	}
	ch, unsub := mgr.SubscribeEvents(32)
	go func() {
		defer unsub()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
			drain:
				for {
					select {
					case _, more := <-ch:
						if !more {
							tm.UpdateAll()
							return
						}
					default:
						break drain
					}
				}
				tm.UpdateAll()
			}
		}
	}()
}

// proxyTabEntry represents a synthetic cross-server player entry in another player's TabList.
type proxyTabEntry struct {
	tl          tablist.TabList
	profile     profile.GameProfile
	displayName c.Component
	gameMode    int
	latency     time.Duration
	chatSession gateplayer.ChatSession
	listed      bool
	listOrder   int
	showHat     bool
}

var _ tablist.Entry = (*proxyTabEntry)(nil)

func newProxyTabEntry(
	tl tablist.TabList,
	gp profile.GameProfile,
	displayName c.Component,
	latency time.Duration,
	listOrder int,
) *proxyTabEntry {
	return &proxyTabEntry{
		tl:          tl,
		profile:     gp,
		displayName: displayName,
		gameMode:    0,
		latency:     latency,
		listed:      true,
		listOrder:   listOrder,
		showHat:     true,
	}
}

func (e *proxyTabEntry) TabList() tablist.TabList            { return e.tl }
func (e *proxyTabEntry) Profile() profile.GameProfile        { return e.profile }
func (e *proxyTabEntry) DisplayName() c.Component            { return e.displayName }
func (e *proxyTabEntry) SetDisplayName(n c.Component) error  { e.displayName = n; return nil }
func (e *proxyTabEntry) GameMode() int                       { return e.gameMode }
func (e *proxyTabEntry) SetGameMode(gm int) error            { e.gameMode = gm; return nil }
func (e *proxyTabEntry) Latency() time.Duration              { return e.latency }
func (e *proxyTabEntry) SetLatency(d time.Duration) error    { e.latency = d; return nil }
func (e *proxyTabEntry) ChatSession() gateplayer.ChatSession { return e.chatSession }
func (e *proxyTabEntry) Listed() bool                        { return e.listed }
func (e *proxyTabEntry) SetListed(l bool) error              { e.listed = l; return nil }
func (e *proxyTabEntry) ListOrder() int                      { return e.listOrder }
func (e *proxyTabEntry) SetListOrder(o int) error            { e.listOrder = o; return nil }
func (e *proxyTabEntry) ShowHat() bool                       { return e.showHat }
func (e *proxyTabEntry) SetShowHat(h bool) error             { e.showHat = h; return nil }
