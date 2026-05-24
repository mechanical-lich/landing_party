package emotes

import (
	"image"

	"github.com/mechanical-lich/landing_party/internal/components"
	"github.com/mechanical-lich/mlge/ecs"
)

// Emote key constants.
const (
	Alert        = "alert"
	Anger        = "anger"
	Bars         = "bars"
	Cash         = "cash"
	Circle       = "circle"
	Cloud        = "cloud"
	Cross        = "cross"
	Dots1        = "dots1"
	Dots2        = "dots2"
	Dots3        = "dots3"
	Drop         = "drop"
	Drops        = "drops"
	Exclamation  = "exclamation"
	Exclamations = "exclamations"
	FaceAngry    = "faceAngry"
	FaceHappy    = "faceHappy"
	FaceSad      = "faceSad"
	Heart        = "heart"
	HeartBroken  = "heartBroken"
	Hearts       = "hearts"
	Idea         = "idea"
	Laugh        = "laugh"
	Music        = "music"
	Question     = "question"
	Sleep        = "sleep"
	Sleeps       = "sleeps"
	Star         = "star"
	Stars        = "stars"
	Swirl        = "swirl"
)

// SpriteSizePx is the width and height of each emote sprite in emotes.png.
const SpriteSizePx = 16

// Sprite returns the top-left pixel coordinate of key in emotes.png.
func Sprite(key string) (image.Point, bool) {
	p, ok := sprites[key]
	return p, ok
}

var sprites = map[string]image.Point{
	Swirl:        {0, 0},
	Stars:        {0, 16},
	Star:         {0, 32},
	Sleeps:       {0, 48},
	Sleep:        {0, 64},
	Question:     {0, 80},
	Music:        {16, 0},
	Laugh:        {16, 16},
	Idea:         {16, 32},
	Hearts:       {16, 48},
	HeartBroken:  {16, 64},
	Heart:        {16, 80},
	FaceHappy:    {32, 16},
	FaceSad:      {32, 0},
	Exclamations: {32, 48},
	Exclamation:  {32, 64},
	Drops:        {32, 80},
	Drop:         {48, 0},
	Dots3:        {48, 16},
	Dots2:        {48, 32},
	Dots1:        {48, 48},
	Cross:        {48, 64},
	Cloud:        {48, 80},
	Circle:       {64, 0},
	Cash:         {64, 16},
	Bars:         {64, 32},
	Anger:        {64, 48},
	Alert:        {64, 64},
	FaceAngry:    {64, 80},
}

// Set queues an emote on entity. Higher priority replaces a pending lower-priority
// emote. Has no effect if the entity has no Emote component.
func Set(entity *ecs.Entity, key string, durationTurns, priority int) {
	if !entity.HasComponent(components.Emote) {
		return
	}
	ec := entity.GetComponent(components.Emote).(*components.EmoteComponent)
	if ec.Pending == "" || priority >= ec.PendingPriority {
		ec.Pending = key
		ec.PendingDuration = durationTurns
		ec.PendingPriority = priority
	}
}
