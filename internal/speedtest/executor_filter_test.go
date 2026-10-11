package speedtest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/structName/sing-box-manager-gui/internal/database"
	"github.com/structName/sing-box-manager-gui/internal/database/models"
	"gorm.io/gorm"
)

func TestFilterNodesByProfileHonorsTagNames(t *testing.T) {
	streaming := models.Node{Tag: "stream", Source: "sub", Country: "JP", Tags: []models.Tag{{Name: "streaming"}}}
	other := models.Node{Tag: "other", Source: "sub", Country: "JP", Tags: []models.Tag{{Name: "backup"}}}
	untagged := models.Node{Tag: "plain", Source: "sub", Country: "JP"}
	nodes := []models.Node{streaming, other, untagged}

	profile := &models.SpeedTestProfile{TagFilter: []string{" streaming ", ""}}
	got := filterNodesByProfile(nodes, profile)
	if len(got) != 1 || got[0].Tag != "stream" {
		t.Fatalf("tag filter = %#v", tagsOf(got))
	}

	combined := &models.SpeedTestProfile{
		SourceFilter: []string{"manual"},
		TagFilter:    []string{"streaming"},
	}
	if got := filterNodesByProfile(nodes, combined); len(got) != 0 {
		t.Fatalf("source+tag mismatch = %#v", tagsOf(got))
	}

	if got := filterNodesByProfile(nodes, &models.SpeedTestProfile{TagFilter: []string{"  "}}); len(got) != len(nodes) {
		t.Fatalf("blank tag filter = %d, want %d", len(got), len(nodes))
	}
}

func TestGetNodesByProfileTagMismatchDoesNotTestAllNodes(t *testing.T) {
	store := newFilterTestStore(t)
	enabled := &models.Node{Tag: "enabled-node", Type: "socks", Server: "192.0.2.10", ServerPort: 1, Source: "manual", Enabled: true}
	if err := store.CreateNode(enabled); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	disabled := &models.Node{Tag: "disabled-node", Type: "socks", Server: "192.0.2.11", ServerPort: 1, Source: "manual", Enabled: false}
	if err := store.CreateNode(disabled); err != nil {
		t.Fatalf("CreateNode disabled: %v", err)
	}
	tag := &models.Tag{Name: "streaming"}
	if err := store.CreateTag(tag); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if err := store.SetNodeTags(enabled.ID, []uint{tag.ID}); err != nil {
		t.Fatalf("SetNodeTags: %v", err)
	}

	executor := NewExecutor(store)
	matched, err := executor.getNodesByProfile(&models.SpeedTestProfile{TagFilter: []string{"streaming"}})
	if err != nil {
		t.Fatalf("matching tag filter: %v", err)
	}
	if len(matched) != 1 || matched[0].Tag != "enabled-node" {
		t.Fatalf("matched = %#v", tagsOf(matched))
	}
	if len(matched[0].Tags) != 1 || matched[0].Tags[0].Name != "streaming" {
		t.Fatalf("tags not preloaded: %#v", matched[0].Tags)
	}

	miss, err := executor.getNodesByProfile(&models.SpeedTestProfile{TagFilter: []string{"missing"}})
	if err != nil {
		t.Fatalf("missing tag filter: %v", err)
	}
	if len(miss) != 0 {
		t.Fatalf("tag mismatch returned %d nodes", len(miss))
	}

	executor.SetTaskManager(failIfCalledTaskManager{})
	_, err = executor.runWithProfile(&models.SpeedTestProfile{Name: "tags", TagFilter: []string{"missing"}}, nil, TriggerTypeManual)
	if err == nil || err.Error() != "没有符合条件的节点" {
		t.Fatalf("empty set error = %v", err)
	}
}

type failIfCalledTaskManager struct{}

func (failIfCalledTaskManager) CreateTask(string, string, string, int) (*models.Task, context.Context, error) {
	return nil, nil, context.Canceled
}
func (failIfCalledTaskManager) StartTask(string) error { return nil }
func (failIfCalledTaskManager) UpdateProgress(string, int, string, string) error {
	return nil
}
func (failIfCalledTaskManager) CompleteTask(string, string, map[string]any) error { return nil }
func (failIfCalledTaskManager) FailTask(string, string) error                     { return nil }
func (failIfCalledTaskManager) CancelTask(string) error                           { return nil }
func (failIfCalledTaskManager) GetTaskContext(string) (context.Context, bool) {
	return nil, false
}

func newFilterTestStore(t *testing.T) *database.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tag-filter.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Node{}, &models.Tag{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database.NewStore(db)
}

func tagsOf(nodes []models.Node) []string {
	out := make([]string, len(nodes))
	for i, node := range nodes {
		out[i] = node.Tag
	}
	return out
}
