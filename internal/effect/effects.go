package effect

import "github.com/hajimehoshi/ebiten/v2"

type Effect interface {
	Update()
	Draw(screen *ebiten.Image, cameraX, cameraY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int)
}

type EffectManager struct {
	effects []Effect
}

var singleton *EffectManager

func GetEffectManager() *EffectManager {
	if singleton == nil {
		singleton = &EffectManager{effects: make([]Effect, 0)}
	}
	return singleton
}

func (em *EffectManager) AddEffect(e Effect) {
	em.effects = append(em.effects, e)
}

func (em *EffectManager) RemoveEffect(e Effect) {
	for i, v := range em.effects {
		if v == e {
			em.effects = append(em.effects[:i], em.effects[i+1:]...)
			return
		}
	}
}

func (em *EffectManager) Update() {
	for _, e := range em.effects {
		e.Update()
	}
}

func (em *EffectManager) Draw(screen *ebiten.Image, cameraX, cameraY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH int) {
	for _, e := range em.effects {
		e.Draw(screen, cameraX, cameraY, cameraZ, tileSizeW, tileSizeH, spriteSizeW, spriteSizeH)
	}
}
