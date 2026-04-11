package api

import (
	"errors"
	"testing"
)

func TestRebuildConfigAndRestartStopsOnBuildError(t *testing.T) {
	buildErr := errors.New("build failed")
	restartCalled := false

	err := rebuildConfigAndRestart(func() error {
		return buildErr
	}, func() error {
		restartCalled = true
		return nil
	})

	if !errors.Is(err, buildErr) {
		t.Fatalf("expected build error %v, got %v", buildErr, err)
	}
	if restartCalled {
		t.Fatal("restart should not be called when build fails")
	}
}

func TestRebuildConfigAndRestartReturnsRestartError(t *testing.T) {
	restartErr := errors.New("restart failed")
	buildCalled := false
	restartCalled := false

	err := rebuildConfigAndRestart(func() error {
		buildCalled = true
		return nil
	}, func() error {
		restartCalled = true
		return restartErr
	})

	if !buildCalled {
		t.Fatal("build should be called before restart")
	}
	if !restartCalled {
		t.Fatal("restart should be called after build succeeds")
	}
	if !errors.Is(err, restartErr) {
		t.Fatalf("expected restart error %v, got %v", restartErr, err)
	}
}
