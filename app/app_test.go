package app

import (
	"errors"
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/trancecode/vantage/scene"
)

func TestNewAppHasManager(t *testing.T) {
	a := New(Config{WindowTitle: "test"})
	if a.Manager() == nil {
		t.Fatal("expected non-nil Manager")
	}
}

func TestRequestExitMakesUpdateReturnErrExit(t *testing.T) {
	a := New(Config{})
	a.RequestExit()
	err := a.Update()
	if err != ErrExit {
		t.Fatalf("expected ErrExit, got %v", err)
	}
}

func TestOnUpdateErrorStopsLoopBeforeScenes(t *testing.T) {
	a := New(Config{})
	notUpdated := &countingScene{name: "s"}
	a.Manager().AddScene(notUpdated)
	wantErr := errors.New("stop")
	a.OnUpdate = func(d time.Duration) error { return wantErr }
	if err := a.Update(); err != wantErr {
		t.Fatalf("expected propagated OnUpdate error, got %v", err)
	}
	if notUpdated.updates != 0 {
		t.Fatalf("scenes must not update after OnUpdate error, got %d", notUpdated.updates)
	}
}

// countingScene is a minimal Scene that counts Update calls.
type countingScene struct {
	scene.BaseScene
	name    scene.SceneName
	updates int
}

func (c *countingScene) SceneName() scene.SceneName { return c.name }
func (c *countingScene) Init(w, h int)              {}
func (c *countingScene) LayerIndex() int            { return 0 }
func (c *countingScene) Update(time.Duration) error { c.updates++; return nil }
func (c *countingScene) Draw(*ebiten.Image)         {}
