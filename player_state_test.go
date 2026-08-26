package velocitab

import (
	"testing"

	"go.minekube.com/gate/pkg/util/uuid"
)

func TestPlayerStateManager(t *testing.T) {
	sm := NewPlayerStateManager()
	id := uuid.New()

	// 1. Custom Name
	sm.SetCustomName(id, "CoolPlayer")
	name, ok := sm.GetCustomName(id)
	if !ok || name != "CoolPlayer" {
		t.Fatalf("Expected 'CoolPlayer', got %q, %v", name, ok)
	}

	// 2. Toggle Tab
	enabled := sm.ToggleTab(id)
	if enabled {
		t.Fatalf("First toggle should disable tab (false), got %v", enabled)
	}
	if !sm.IsTabDisabled(id) {
		t.Fatalf("Expected tab to be disabled")
	}

	enabledSecond := sm.ToggleTab(id)
	if !enabledSecond {
		t.Fatalf("Second toggle should re-enable tab (true), got %v", enabledSecond)
	}

	// 3. Reset
	if !sm.ResetCustomName(id) {
		t.Fatalf("Expected ResetCustomName to return true")
	}
	if _, ok := sm.GetCustomName(id); ok {
		t.Fatalf("Expected custom name to be cleared")
	}
}
