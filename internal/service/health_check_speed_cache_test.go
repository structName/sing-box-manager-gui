package service

import (
	"testing"
	"time"

	"github.com/xiaobei/singbox-manager/internal/storage"
)

func TestCacheSpeedResult_StoreAndRetrieve(t *testing.T) {
	t.Parallel()

	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	svc := NewHealthCheckService(store)

	result := &storage.ChainSpeedResult{
		ChainID:    "chain-1",
		TestTime:   time.Now(),
		SpeedMbps:  42.5,
		BytesTotal: 5000000,
		Duration:   1200,
	}

	svc.cacheSpeedResult("chain-1", result)

	got := svc.GetCachedSpeedResult("chain-1")
	if got == nil {
		t.Fatal("GetCachedSpeedResult() returned nil for cached chain")
	}
	if got.ChainID != "chain-1" {
		t.Fatalf("ChainID = %q, want %q", got.ChainID, "chain-1")
	}
	if got.SpeedMbps != 42.5 {
		t.Fatalf("SpeedMbps = %f, want 42.5", got.SpeedMbps)
	}
	if got.BytesTotal != 5000000 {
		t.Fatalf("BytesTotal = %d, want 5000000", got.BytesTotal)
	}
	if got.Duration != 1200 {
		t.Fatalf("Duration = %d, want 1200", got.Duration)
	}
}

func TestGetCachedSpeedResult_ReturnsNilForMissing(t *testing.T) {
	t.Parallel()

	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	svc := NewHealthCheckService(store)

	got := svc.GetCachedSpeedResult("nonexistent")
	if got != nil {
		t.Fatalf("GetCachedSpeedResult() = %v, want nil for missing chain", got)
	}
}

func TestCacheSpeedResult_OverwritesPrevious(t *testing.T) {
	t.Parallel()

	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	svc := NewHealthCheckService(store)

	first := &storage.ChainSpeedResult{
		ChainID:   "chain-1",
		SpeedMbps: 10.0,
	}
	svc.cacheSpeedResult("chain-1", first)

	second := &storage.ChainSpeedResult{
		ChainID:   "chain-1",
		SpeedMbps: 99.9,
	}
	svc.cacheSpeedResult("chain-1", second)

	got := svc.GetCachedSpeedResult("chain-1")
	if got == nil {
		t.Fatal("GetCachedSpeedResult() returned nil")
	}
	if got.SpeedMbps != 99.9 {
		t.Fatalf("SpeedMbps = %f, want 99.9 (should be overwritten)", got.SpeedMbps)
	}
}

func TestGetAllCachedSpeedResults_Empty(t *testing.T) {
	t.Parallel()

	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	svc := NewHealthCheckService(store)

	results := svc.GetAllCachedSpeedResults()
	if results == nil {
		t.Fatal("GetAllCachedSpeedResults() returned nil, want empty map")
	}
	if len(results) != 0 {
		t.Fatalf("len(results) = %d, want 0", len(results))
	}
}

func TestGetAllCachedSpeedResults_MultipleChains(t *testing.T) {
	t.Parallel()

	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	svc := NewHealthCheckService(store)

	chains := map[string]float64{
		"chain-a": 10.0,
		"chain-b": 20.0,
		"chain-c": 30.0,
	}

	for id, speed := range chains {
		svc.cacheSpeedResult(id, &storage.ChainSpeedResult{
			ChainID:   id,
			SpeedMbps: speed,
		})
	}

	results := svc.GetAllCachedSpeedResults()
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}

	for id, wantSpeed := range chains {
		got, ok := results[id]
		if !ok {
			t.Fatalf("results missing key %q", id)
		}
		if got.SpeedMbps != wantSpeed {
			t.Fatalf("results[%q].SpeedMbps = %f, want %f", id, got.SpeedMbps, wantSpeed)
		}
	}
}

func TestGetAllCachedSpeedResults_ReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()

	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	svc := NewHealthCheckService(store)

	svc.cacheSpeedResult("chain-1", &storage.ChainSpeedResult{
		ChainID:   "chain-1",
		SpeedMbps: 50.0,
	})

	// Get the map and mutate it
	results := svc.GetAllCachedSpeedResults()
	delete(results, "chain-1")

	// The original cache should be unaffected
	got := svc.GetCachedSpeedResult("chain-1")
	if got == nil {
		t.Fatal("deleting from returned map should not affect internal cache")
	}
}

func TestSpeedCacheInitialisedByConstructor(t *testing.T) {
	t.Parallel()

	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	svc := NewHealthCheckService(store)

	// Immediately calling Get methods should not panic (maps must be initialised)
	_ = svc.GetCachedSpeedResult("any")
	_ = svc.GetAllCachedSpeedResults()
}
