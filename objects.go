package hanabi

import (
	"math"
	"image"
)

type BaseObject struct {
	Id            int16
	X, Y          int16
	Height, Width int16
	scope *image.Rectangle
}

func (o *BaseObject) getWidth() int16 {
	return o.Width
}

func (o *BaseObject) getHeight() int16 {
	return o.Height
}

func (o *BaseObject) getX() int16 {
	return o.X
}

func (o *BaseObject) getY() int16 {
	return o.Y
}

func clamp16(x, min, max int16) int16 {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}

func (o *BaseObject) setX(x int16) {
	if o.scope != nil {
		x = clamp16(x, int16(o.scope.Min.X), int16(o.scope.Max.X) - o.Width)
	}
	o.X = x
}

func (o *BaseObject) setY(y int16) {
	if o.scope != nil {
		y = clamp16(y, int16(o.scope.Min.Y), int16(o.scope.Max.Y) - o.Height)
	}
	o.Y = y
}

func (o *BaseObject) tick() {
}

type HasShape interface {
	tick()
	getX() int16
	getY() int16
	getWidth() int16
	getHeight() int16
}

type Flipper interface {
	flip()
}

type Mover interface {
	setX(y int16)
	setY(y int16)
}

type RotatingObject struct {
	BaseObject
	centerX, centerY, radius int16
}

func (o *RotatingObject) tick() {
	o.Y = o.centerX + int16(float64(o.radius)*math.Cos(math.Pi*2*float64(passedSeconds)/2))
	o.X = o.centerY + int16(float64(o.radius)*math.Sin(math.Pi*2*float64(passedSeconds)/2))
}

type StaticObject struct {
	BaseObject
}

type Card struct {
	BaseObject
	SpiritId int16
}

func newCard(id, x, y, spiritId int16, scope *image.Rectangle) *Card{
	return &Card{BaseObject{Id: id, X: x, Y: y, Width: 100, Height: 140, scope: scope}, spiritId}
}

type HintToken struct {
	BaseObject
	SpiritId int
	Used     bool
}

func (t *HintToken) flip() {
	t.Used = !t.Used

	if t.Used {
		t.SpiritId = 104
	} else {
		t.SpiritId = 103
	}
}

func newHintToken(id, x, y int16) *HintToken {
	return &HintToken{
		BaseObject{id, x, y, 25, 25, &fullScope},
		103,
		false,
	}
}

type MistakeToken struct {
	BaseObject
	SpiritId int
	Used     bool
}

func (t *MistakeToken) flip() {
	t.Used = !t.Used
	if t.Used {
		t.SpiritId = 102
	} else {
		t.SpiritId = 101
	}
}

func newMistakeToken(id, x, y int16) *MistakeToken {
	return &MistakeToken{
		BaseObject{id, x, y, 25, 25, &fullScope},
		101,
		false,
	}
}
