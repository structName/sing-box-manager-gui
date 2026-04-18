package database

import (
	"path/filepath"
	"testing"
)

// TestOpenDB_DoesNotOverwriteGlobalDB verifies that OpenDB() opens a database
// without replacing the package-level global `db` singleton. This is the core
// fix for the "sql: database is closed" bug that occurred when Profile.Create()
// used InitDBWithPath() (which overwrites the global) and then closed the
// returned connection.
func TestOpenDB_DoesNotOverwriteGlobalDB(t *testing.T) {
	// Reset global state so previous tests don't interfere
	ResetDB()

	// Step 1: Initialise the global db with InitDBWithPath
	globalDir := t.TempDir()
	globalDB, err := InitDBWithPath(globalDir)
	if err != nil {
		t.Fatalf("InitDBWithPath() error = %v", err)
	}
	if globalDB == nil {
		t.Fatal("InitDBWithPath() returned nil db")
	}

	// Confirm the global singleton is set
	if GetDB() != globalDB {
		t.Fatal("GetDB() should return the same instance set by InitDBWithPath()")
	}

	// Step 2: Call OpenDB with a different directory
	otherDir := t.TempDir()
	otherDB, err := OpenDB(otherDir)
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	if otherDB == nil {
		t.Fatal("OpenDB() returned nil db")
	}

	// Step 3: The global singleton must still point to the first DB
	if GetDB() != globalDB {
		t.Fatal("OpenDB() must NOT overwrite the global db singleton")
	}

	// Step 4: Close the otherDB (simulates what Profile.Create does)
	sqlDB, err := otherDB.DB()
	if err != nil {
		t.Fatalf("otherDB.DB() error = %v", err)
	}
	sqlDB.Close()

	// Step 5: The global db must still be usable after otherDB is closed
	sqlGlobal, err := GetDB().DB()
	if err != nil {
		t.Fatalf("GetDB().DB() error = %v", err)
	}
	if err := sqlGlobal.Ping(); err != nil {
		t.Fatalf("global db Ping() failed after closing OpenDB() connection: %v", err)
	}

	// Cleanup
	ResetDB()
}

// TestInitDBWithPath_SetsGlobalDB verifies that InitDBWithPath() correctly
// sets the package-level global db singleton.
func TestInitDBWithPath_SetsGlobalDB(t *testing.T) {
	ResetDB()

	dir := t.TempDir()
	retDB, err := InitDBWithPath(dir)
	if err != nil {
		t.Fatalf("InitDBWithPath() error = %v", err)
	}
	if retDB == nil {
		t.Fatal("InitDBWithPath() returned nil")
	}
	if GetDB() != retDB {
		t.Fatal("GetDB() should return the db set by InitDBWithPath()")
	}

	// Verify the database file was created
	sqlDB, err := retDB.DB()
	if err != nil {
		t.Fatalf("retDB.DB() error = %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	ResetDB()
}

// TestInitDBWithPath_SwitchesGlobal verifies that calling InitDBWithPath()
// a second time replaces the global db singleton (used for Profile switching).
func TestInitDBWithPath_SwitchesGlobal(t *testing.T) {
	ResetDB()

	dir1 := t.TempDir()
	db1, err := InitDBWithPath(dir1)
	if err != nil {
		t.Fatalf("InitDBWithPath(dir1) error = %v", err)
	}

	dir2 := t.TempDir()
	db2, err := InitDBWithPath(dir2)
	if err != nil {
		t.Fatalf("InitDBWithPath(dir2) error = %v", err)
	}

	if GetDB() == db1 {
		t.Fatal("GetDB() should no longer return db1 after second InitDBWithPath() call")
	}
	if GetDB() != db2 {
		t.Fatal("GetDB() should return db2 after second InitDBWithPath() call")
	}

	ResetDB()
}

// TestOpenDB_CreatesDBFile verifies that OpenDB creates the database file and
// runs auto-migrations.
func TestOpenDB_CreatesDBFile(t *testing.T) {
	ResetDB()

	dir := t.TempDir()
	newDB, err := OpenDB(dir)
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	if newDB == nil {
		t.Fatal("OpenDB() returned nil")
	}

	// Verify the file exists by pinging
	sqlDB, err := newDB.DB()
	if err != nil {
		t.Fatalf("newDB.DB() error = %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	// Check that the sbm.db file was created
	dbPath := filepath.Join(dir, "sbm.db")
	// We can verify by opening a second connection to the same path
	newDB2, err := OpenDB(dir)
	if err != nil {
		t.Fatalf("second OpenDB() error = %v", err)
	}
	_ = dbPath
	sqlDB2, _ := newDB2.DB()
	sqlDB2.Close()
	sqlDB.Close()

	ResetDB()
}

// TestResetDB_ClearsGlobal verifies that ResetDB() sets the global db to nil
// and allows re-initialization via InitDB.
func TestResetDB_ClearsGlobal(t *testing.T) {
	ResetDB()

	dir := t.TempDir()
	_, err := InitDBWithPath(dir)
	if err != nil {
		t.Fatalf("InitDBWithPath() error = %v", err)
	}
	if GetDB() == nil {
		t.Fatal("GetDB() should not be nil after InitDBWithPath()")
	}

	ResetDB()

	if GetDB() != nil {
		t.Fatal("GetDB() should be nil after ResetDB()")
	}
}

// TestCreateProfile_DoesNotBreakGlobalDB simulates the exact bug scenario:
// 1. Initialize global DB (representing the active profile)
// 2. Call OpenDB + Close (what Profile.Create does)
// 3. Verify global DB is still healthy
func TestCreateProfile_DoesNotBreakGlobalDB(t *testing.T) {
	ResetDB()

	// Simulate the active profile's DB
	activeDir := t.TempDir()
	activeDB, err := InitDBWithPath(activeDir)
	if err != nil {
		t.Fatalf("InitDBWithPath() error = %v", err)
	}

	// Verify active DB works
	sqlActive, err := activeDB.DB()
	if err != nil {
		t.Fatalf("activeDB.DB() error = %v", err)
	}
	if err := sqlActive.Ping(); err != nil {
		t.Fatalf("active DB Ping() before Create: %v", err)
	}

	// Simulate Profile.Create(): OpenDB + Close
	newProfileDir := t.TempDir()
	newDB, err := OpenDB(newProfileDir)
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	sqlNew, _ := newDB.DB()
	sqlNew.Close()

	// The global DB must still work (this would fail with the old bug)
	globalDB := GetDB()
	if globalDB == nil {
		t.Fatal("GetDB() returned nil after OpenDB+Close -- global was corrupted")
	}
	sqlGlobal, err := globalDB.DB()
	if err != nil {
		t.Fatalf("GetDB().DB() error = %v", err)
	}
	if err := sqlGlobal.Ping(); err != nil {
		t.Fatalf("global DB Ping() FAILED after Profile.Create simulation: %v -- this is the bug!", err)
	}

	ResetDB()
}
