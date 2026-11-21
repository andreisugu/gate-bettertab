package bettertab

import (
	"testing"
	"time"

	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/profile"
	"go.minekube.com/gate/pkg/edition/java/proxy/tablist"
	"go.minekube.com/gate/pkg/util/uuid"
)

func TestSortPlayerSnapshotsByWeightAndName(t *testing.T) {
	snaps := []playerSnapshot{
		{id: uuid.New(), realUsername: "Zed", role: Role{Weight: 0}, serverName: "lobby"},
		{id: uuid.New(), realUsername: "Bob", role: Role{Weight: 500}, serverName: "survival"},
		{id: uuid.New(), realUsername: "Amy", role: Role{Weight: 500}, serverName: "lobby"},
		{id: uuid.New(), realUsername: "Alex", customName: "&bAaron", role: Role{Weight: 100}, serverName: "lobby"},
		{id: uuid.New(), realUsername: "Beth", role: Role{Weight: 100}, serverName: "lobby"},
	}

	sortPlayerSnapshots(
		snaps,
		[]string{"%role_weight%", "%username_lower%"},
		map[string]int{"lobby": 4, "survival": 1},
		map[string]int{"default": 5},
		5, 100,
	)

	want := []string{"Amy", "Bob", "Alex", "Beth", "Zed"}
	for i, w := range want {
		if snaps[i].realUsername != w {
			t.Fatalf("rank %d = %q, want %q (full order: %+v)", i, snaps[i].realUsername, w, snaps)
		}
	}

	// Verify positive descending ListOrder formula (1.21.2+ -getTabListOrder() sorts highest first).
	prevOrder := int(^uint(0) >> 1)
	for rankIdx := range snaps {
		order := (len(snaps) - rankIdx) * 10
		if order <= 0 || order >= prevOrder {
			t.Fatalf("invalid descending order at rank %d: order=%d prev=%d", rankIdx, order, prevOrder)
		}
		prevOrder = order
	}
}

func TestProxyTabEntryAndLatencyBuckets(t *testing.T) {
	var tl tablist.TabList
	id := uuid.New()
	gp := profile.GameProfile{ID: id, Name: "CrossPlayer"}
	e := newProxyTabEntry(tl, gp, &c.Text{Content: "[VIP] CrossPlayer"}, 42*time.Millisecond, 50)

	if e.Profile().ID != id || e.ListOrder() != 50 || !e.Listed() || !e.ShowHat() {
		t.Fatalf("unexpected proxyTabEntry fields: %+v", e)
	}
	_ = e.SetListOrder(80)
	_ = e.SetListed(false)
	if e.ListOrder() != 80 || e.Listed() {
		t.Fatalf("expected updated proxyTabEntry state")
	}

	if latencyBucketFor(-1*time.Millisecond) != -1 ||
		latencyBucketFor(20*time.Millisecond) != 5 ||
		latencyBucketFor(200*time.Millisecond) != 4 ||
		latencyBucketFor(400*time.Millisecond) != 3 ||
		latencyBucketFor(800*time.Millisecond) != 2 ||
		latencyBucketFor(1500*time.Millisecond) != 1 {
		t.Fatalf("unexpected latency bucket thresholds")
	}
}
