package hanabi

import (
	"math"
)

type BaseObject struct {
	Id                       int
	X, Y                     int
	Height, Width            int
	centerX, centerY, radius int
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
func (o *BaseObject) tick(passedSeconds float64) {
	o.X = o.centerX + int(float64(o.radius)*math.Cos(math.Pi*2*float64(passedSeconds)/2))
	o.Y = o.centerY + int(float64(o.radius)*math.Sin(math.Pi*2*float64(passedSeconds)/2))
}

type CardColor int

const ColorCount = 5
const NumberMax = 5
const NumberMin = 1

type Card struct {
	Id           int
	Color        CardColor // Color == 0 means unknown color (to client)
	Number       int       // Number == 0 means unknown number
	ColorHinted  bool
	NumberHinted bool
}

func newCard(id int, color CardColor, number int) *Card {
	return &Card{
		id,
		color,
		number,
		false,
		false,
	}
}
