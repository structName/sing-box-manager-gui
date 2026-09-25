package api

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/structName/sing-box-manager-gui/internal/database"
	"github.com/structName/sing-box-manager-gui/internal/database/models"
	"github.com/structName/sing-box-manager-gui/internal/service"
	"gorm.io/gorm"
)

func newSpeedTestEnsureTestStore(t *testing.T) *database.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sbm-speedtest-ensure.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SpeedTestProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database.NewStore(db)
}

func TestEnsureDefaultSpeedTestProfileDoesNotForceEnableExisting(t *testing.T) {
	store := newSpeedTestEnsureTestStore(t)
	disabled := &models.SpeedTestProfile{
		Name:         "user-disabled",
		Enabled:      true,
		AutoTest:     false,
		Mode:         "speed",
		ScheduleType: "cron",
		ScheduleCron: "0 0 */6 * * *",
	}
	if err := store.CreateSpeedTestProfile(disabled); err != nil {
		t.Fatalf("CreateSpeedTestProfile: %v", err)
	}
	if disabled.ID == 0 {
		t.Fatal("CreateSpeedTestProfile did not assign ID")
	}
	disabledID := disabled.ID

	sched := service.NewUnifiedScheduler(store, nil)
	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	server := &Server{
		dbStore:          store,
		unifiedScheduler: sched,
	}
	server.ensureDefaultSpeedTestProfile()

	profiles, err := store.GetSpeedTestProfiles()
	if err != nil {
		t.Fatalf("GetSpeedTestProfiles: %v", err)
	}

	var sawDisabled, sawDelay bool
	for _, p := range profiles {
		switch p.Name {
		case "user-disabled":
			sawDisabled = true
			if p.AutoTest {
				t.Fatalf("existing profile AutoTest forced back on; want left false")
			}
			if !p.Enabled {
				t.Fatalf("existing profile Enabled flipped off")
			}
		case "延迟检测":
			sawDelay = true
			if !p.AutoTest || !p.Enabled {
				t.Fatalf("default delay profile AutoTest/Enabled = %v/%v, want true/true", p.AutoTest, p.Enabled)
			}
		}
	}
	if !sawDisabled {
		t.Fatal("existing user-disabled profile missing after ensure")
	}
	if !sawDelay {
		t.Fatal("expected default delay profile to be created (missing mode)")
	}

	// Existing disabled profile must not be registered for cron.
	if entry := sched.GetEntry("speed_test:" + strconv.FormatUint(uint64(disabledID), 10)); entry != nil {
		t.Fatalf("disabled profile was scheduled: %+v", entry)
	}
}

func TestEnsureDefaultSpeedTestProfileCreatesBothWhenEmpty(t *testing.T) {
	store := newSpeedTestEnsureTestStore(t)
	sched := service.NewUnifiedScheduler(store, nil)
	if err := sched.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	server := &Server{dbStore: store, unifiedScheduler: sched}
	server.ensureDefaultSpeedTestProfile()

	profiles, err := store.GetSpeedTestProfiles()
	if err != nil {
		t.Fatalf("GetSpeedTestProfiles: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("profiles = %d, want 2 defaults", len(profiles))
	}
	for _, p := range profiles {
		if !p.AutoTest || !p.Enabled {
			t.Fatalf("default %q AutoTest/Enabled = %v/%v", p.Name, p.AutoTest, p.Enabled)
		}
	}
}
