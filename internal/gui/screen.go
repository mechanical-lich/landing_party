package gui

import "github.com/hajimehoshi/ebiten/v2"

type Screen interface {
	OnEnter()
	OnExit()
	Update()
	Draw(screen *ebiten.Image)
	IsOpaque() bool
}

type ScreenManager struct {
	stack []Screen
}

func (sm *ScreenManager) Push(s Screen) {
	s.OnEnter()
	sm.stack = append(sm.stack, s)
}

func (sm *ScreenManager) Pop() {
	if len(sm.stack) == 0 {
		return
	}
	top := sm.stack[len(sm.stack)-1]
	top.OnExit()
	sm.stack = sm.stack[:len(sm.stack)-1]
}

func (sm *ScreenManager) Update() {
	first := sm.firstOpaqueIdx()
	for i := first; i < len(sm.stack); i++ {
		sm.stack[i].Update()
	}
}

func (sm *ScreenManager) Draw(screen *ebiten.Image) {
	first := sm.firstOpaqueIdx()
	for i := first; i < len(sm.stack); i++ {
		sm.stack[i].Draw(screen)
	}
}

func (sm *ScreenManager) firstOpaqueIdx() int {
	for i := len(sm.stack) - 1; i >= 0; i-- {
		if sm.stack[i].IsOpaque() {
			return i
		}
	}
	return 0
}
