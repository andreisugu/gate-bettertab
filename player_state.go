package velocitab

import (
	"strings"
	"sync"

	"go.minekube.com/gate/pkg/util/uuid"
)

// PlayerStateManager tracks in-memory custom names and preferences for players.
type PlayerStateManager struct {
	mu           sync.RWMutex
	customNames  map[uuid.UUID]string
	disabledTabs map[uuid.UUID]bool
}

// NewPlayerStateManager creates a new PlayerStateManager.
func NewPlayerStateManager() *PlayerStateManager {
	return &PlayerStateManager{
		customNames:  make(map[uuid.UUID]string),
		disabledTabs: make(map[uuid.UUID]bool),
	}
}

// SetCustomName sets a custom display nickname for a player.
func (sm *PlayerStateManager) SetCustomName(id uuid.UUID, name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	clean := strings.TrimSpace(name)
	if clean == "" {
		delete(sm.customNames, id)
	} else {
		sm.customNames[id] = clean
	}
}

// GetCustomName returns the custom nickname if present.
func (sm *PlayerStateManager) GetCustomName(id uuid.UUID) (string, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	name, ok := sm.customNames[id]
	return name, ok
}

// ResetCustomName clears any custom nickname for a player.
func (sm *PlayerStateManager) ResetCustomName(id uuid.UUID) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.customNames[id]; ok {
		delete(sm.customNames, id)
		return true
	}
	return false
}

// ToggleTab toggles custom tab formatting for a player and returns the new state (true = enabled, false = disabled).
func (sm *PlayerStateManager) ToggleTab(id uuid.UUID) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	disabled := sm.disabledTabs[id]
	if disabled {
		delete(sm.disabledTabs, id)
		return true
	}
	sm.disabledTabs[id] = true
	return false
}

// IsTabDisabled checks if custom tab formatting is disabled for a player.
func (sm *PlayerStateManager) IsTabDisabled(id uuid.UUID) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.disabledTabs[id]
}

// RemovePlayer cleans up player state on disconnect.
func (sm *PlayerStateManager) RemovePlayer(id uuid.UUID) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.customNames, id)
	delete(sm.disabledTabs, id)
}
