package scene

import (
	"fmt"
	"sort"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Manager owns a set of registered scenes and drives their lifecycle: it
// initializes them, updates them each frame, and draws them in layer order.
// Scenes are keyed by SceneName.
type Manager struct {
	scenes map[SceneName]Scene
}

// NewManager returns an empty scene Manager.
func NewManager() *Manager {
	return &Manager{scenes: map[SceneName]Scene{}}
}

// AddScene registers a scene. It panics if a scene with the same name is
// already registered.
func (m *Manager) AddScene(s Scene) {
	name := s.SceneName()
	if _, ok := m.scenes[name]; ok {
		panic(fmt.Sprintf("duplicate scene name: %s", name))
	}
	m.scenes[name] = s
}

// Scene returns the registered scene with the given name.
func (m *Manager) Scene(name SceneName) (Scene, bool) {
	s, ok := m.scenes[name]
	return s, ok
}

// Init initializes every registered scene with the screen dimensions.
func (m *Manager) Init(screenWidth, screenHeight int) {
	for _, s := range m.scenes {
		s.Init(screenWidth, screenHeight)
	}
}

// Update advances every registered scene by the given duration.
func (m *Manager) Update(duration time.Duration) error {
	for name, s := range m.scenes {
		if err := s.Update(duration); err != nil {
			return fmt.Errorf("updating scene %q: %w", name, err)
		}
	}
	return nil
}

// Draw renders all registered scenes onto screen in ascending layer order.
func (m *Manager) Draw(screen *ebiten.Image) {
	sceneList := make([]Scene, 0, len(m.scenes))
	for _, s := range m.scenes {
		sceneList = append(sceneList, s)
	}
	sort.Slice(sceneList, func(i, j int) bool {
		return sceneList[i].LayerIndex() < sceneList[j].LayerIndex()
	})
	for _, s := range sceneList {
		s.Draw(screen)
	}
}

// SetVisible sets the visibility of a single registered scene.
func (m *Manager) SetVisible(name SceneName, visible bool) {
	s, ok := m.scenes[name]
	if !ok {
		panic(fmt.Sprintf("scene not found: %s", name))
	}
	s.SetVisible(visible)
}

// ShowOnly makes the named scenes visible and hides all others.
func (m *Manager) ShowOnly(names ...SceneName) {
	show := make(map[SceneName]bool, len(names))
	for _, n := range names {
		show[n] = true
	}
	for name, s := range m.scenes {
		s.SetVisible(show[name])
	}
}

// SetExclusiveFocus gives focus to the named scene and removes focus from all
// others.
func (m *Manager) SetExclusiveFocus(name SceneName) {
	for n, s := range m.scenes {
		s.SetFocus(n == name)
	}
}
