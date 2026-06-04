package hubcore

import (
	"testing"
	"time"
)

func TestAppRegisterAndDisconnect(t *testing.T) {
	h := NewHub()
	defer h.Stop()

	h.registerApp(AppInfo{Token: "t1", AppName: "app1"})
	h.registerApp(AppInfo{Token: "t2", AppName: "app2"})
	if got := len(h.Apps()); got != 2 {
		t.Fatalf("apps = %d, want 2", got)
	}

	h.DisconnectApp("t1")
	if got := len(h.Apps()); got != 1 {
		t.Fatalf("after disconnect apps = %d, want 1", got)
	}
	if h.Apps()[0].Token != "t2" {
		t.Errorf("remaining app = %q, want t2", h.Apps()[0].Token)
	}
}

func TestSweepIdleApps(t *testing.T) {
	h := NewHub()
	defer h.Stop()

	h.registerApp(AppInfo{Token: "fresh"})
	h.registerApp(AppInfo{Token: "stale"})

	// stale の最終通信時刻を TTL より過去にする。
	h.mu.Lock()
	h.apps["stale"].lastSeen = time.Now().Add(-2 * appIdleTTL)
	h.mu.Unlock()

	h.sweepIdleApps()

	apps := h.Apps()
	if len(apps) != 1 || apps[0].Token != "fresh" {
		t.Errorf("after sweep apps = %v, want only fresh", apps)
	}
}

func TestTouchAppKeepsAlive(t *testing.T) {
	h := NewHub()
	defer h.Stop()

	h.registerApp(AppInfo{Token: "t"})
	h.mu.Lock()
	h.apps["t"].lastSeen = time.Now().Add(-2 * appIdleTTL)
	h.mu.Unlock()

	h.touchApp("t") // 通信があったので生存更新
	h.sweepIdleApps()

	if got := len(h.Apps()); got != 1 {
		t.Errorf("touched app was swept; apps = %d, want 1", got)
	}
}
