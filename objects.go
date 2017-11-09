package hanabi

import (
	"image"
	"math"
)

type BaseObject struct {
	Id            int
	X, Y          int
	Height, Width int
	scope         *image.Rectangle
}

func (o *BaseObject) getId() int {
	return o.Id
}

func (o *BaseObject) getWidth() int {
	return o.Width
}

func (o *BaseObject) getHeight() int {
	return o.Height
}

func (o *BaseObject) getX() int {
	return o.X
}

func (o *BaseObject) getY() int {
	return o.Y
}


func clamp(x, min, max int) int {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}

func (o *BaseObject) setX(x int) {
	if o.scope != nil {
		x = clamp(x, o.scope.Min.X, o.scope.Max.X-o.Width)
	}
	o.X = x
}

func (o *BaseObject) setY(y int) {
	if o.scope != nil {
		y = clamp(y, o.scope.Min.Y, o.scope.Max.Y-o.Height)
	}
	o.Y = y
}

func (o *BaseObject) tick() {
}

type HasShape interface {
	tick()
	getX() int
	getY() int
	getId() int
	getWidth() int
	getHeight() int
}

type Flipper interface {
	flip()
}

type Mover interface {
	setX(y int)
	setY(y int)
}

type RotatingObject struct {
	BaseObject
	centerX, centerY, radius int
}

func (o *RotatingObject) tick() {
	o.Y = o.centerX + int(float64(o.radius)*math.Cos(math.Pi*2*float64(passedSeconds)/2))
	o.X = o.centerY + int(float64(o.radius)*math.Sin(math.Pi*2*float64(passedSeconds)/2))
}

type StaticObject struct {
	BaseObject
}

type CardColor int

const ColorCount = 5
const NumberMax = 5
const NumberMin = 1

type Card struct {
	BaseObject
	SpiritId int
	color    CardColor
	number   int
}

func newCard(id, x, y int, color CardColor, number int, scope *image.Rectangle) *Card {
	const scale = 0.7
	return &Card{BaseObject{Id: id, X: x, Y: y, Width: 100 * scale, Height: 140 * scale, scope: scope}, (int(color)+1)*10 + number, color, number}
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

func newHintToken(id, x, y int) *HintToken {
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

func newMistakeToken(id, x, y int) *MistakeToken {
	return &MistakeToken{
		BaseObject{id, x, y, 25, 25, &fullScope},
		101,
		false,
	}
}
