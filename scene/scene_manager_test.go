package scene

import (
	"errors"
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// fakeScene is a minimal Scene for exercising the Manager.
type fakeScene struct {
	BaseScene
	name      SceneName
	layer     int
	updates   int
	draws     int
	updateErr error
}

func (f *fakeScene) SceneName() SceneName { return f.name }
func (f *fakeScene) Init(w, h int)        {}
func (f *fakeScene) LayerIndex() int      { return f.layer }
func (f *fakeScene) Update(d time.Duration) error {
	f.updates++
	return f.updateErr
}
func (f *fakeScene) Draw(screen *ebiten.Image) { f.draws++ }

func TestManagerAddSceneDuplicatePanics(t *testing.T) {
	m := NewManager()
	m.AddScene(&fakeScene{name: "a"})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate scene name")
		}
	}()
	m.AddScene(&fakeScene{name: "a"})
}

func TestManagerUpdateAllAndPropagatesError(t *testing.T) {
	m := NewManager()
	good := &fakeScene{name: "good"}
	m.AddScene(good)
	if err := m.Update(time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if good.updates != 1 {
		t.Fatalf("expected 1 update, got %d", good.updates)
	}
}

func TestManagerUpdatePropagatesSceneError(t *testing.T) {
	m := NewManager()
	boom := errors.New("boom")
	m.AddScene(&fakeScene{name: "bad", updateErr: boom})
	err := m.Update(time.Second)
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("expected wrapped boom error, got %v", err)
	}
}

func TestManagerSetExclusiveFocus(t *testing.T) {
	m := NewManager()
	a := &fakeScene{name: "a"}
	b := &fakeScene{name: "b"}
	m.AddScene(a)
	m.AddScene(b)
	m.SetExclusiveFocus("a")
	if !a.HasFocus() {
		t.Fatal("scene a should have focus")
	}
	if b.HasFocus() {
		t.Fatal("scene b should not have focus")
	}
}

func TestManagerShowOnly(t *testing.T) {
	m := NewManager()
	a := &fakeScene{name: "a"}
	b := &fakeScene{name: "b"}
	m.AddScene(a)
	m.AddScene(b)
	m.ShowOnly("a")
	if !a.IsVisible() {
		t.Fatal("scene a should be visible")
	}
	if b.IsVisible() {
		t.Fatal("scene b should be hidden")
	}
}
