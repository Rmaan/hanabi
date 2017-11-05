package hanabi

import "math"

type BaseObject struct {
	Id int16
	X, Y          int16
	Height, Width int16
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

type HasShape interface {
	tick(tickNumber int)
	getX() int16
	getY() int16
	getWidth() int16
	getHeight() int16
}

type RotatingObject struct {
	BaseObject
	centerX, centerY, radius int16
}

func (o *RotatingObject) tick(tickNumber int) {
	o.Y = o.centerX + int16(float64(o.radius)*math.Cos(math.Pi*2*float64(tickNumber)/50))
	o.X = o.centerY + int16(float64(o.radius)*math.Sin(math.Pi*2*float64(tickNumber)/50))
}

type StaticObject struct {
	BaseObject
}

func (o *StaticObject) tick(int) {
}

type Card struct {
	BaseObject
	SpiritId int
}

func (c *Card) tick(tickNumber int) {
}

