package cluster

import (
	"testing"

	"github.com/alexandrmotologa/streamgate/internal/hub"
)

func TestClusterManager_Standalone(t *testing.T) {
	h := hub.NewHub(10, 10, "drop_oldest")
	cm, err := NewManager("", "", "test", h)
	if err != nil {
		t.Fatalf("expected nil error for standalone mode, got %v", err)
	}
	if cm != nil {
		t.Fatalf("expected nil cluster manager when redis addr is empty")
	}

	// PublishToCluster on nil should be safe no-op
	if err := cm.PublishToCluster("test", "ev", []byte("{}")); err != nil {
		t.Fatalf("expected nil error on nil manager call")
	}
}
