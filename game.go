package hanabi

import (
	"fmt"
	"time"
	"github.com/gorilla/websocket"
	"log"
	"math"
	"github.com/vmihailenco/msgpack"
	"encoding/json"
)

var allObjects = make([]HasShape, 0)
var clientList = make([]*websocket.Conn, 0)
var newCommands = make(chan playerCommand, 100)

type playerCommand struct {
	ws *websocket.Conn
	data []byte
}

const maxWidth = 500
const maxHeight = 280

type BaseObject struct {
	X, Y int16
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

type Packet struct {
	AllObjects []HasShape
	TickNumber int
}

func enqueuePlayerCommands(ws *websocket.Conn) {
	for {
		mt, message, err := ws.ReadMessage()
		if err != nil {
			log.Println("error in WS read:", err)
			break
		}
		if mt != websocket.TextMessage {
			log.Printf("Binary message received from ws %v", ws)
			break
		}
		newCommands <- playerCommand{ws, message}
	}
}

func joinNewPlayers() {
	for len(newClients) > 0 {
		ws := <-newClients
		//_ = ws.WriteJSON("Welcome!")
		clientList = append(clientList, ws)
		go enqueuePlayerCommands(ws)
	}
}

func doTick(tickNumber int) {
	joinNewPlayers()

	processCommands(tickNumber)
	for _, obj := range allObjects {
		obj.tick(tickNumber)
	}

	broadcastWorld(tickNumber)
}

func processCommands(tickNumber int) {
	for len(newCommands) > 0 {
		c := <- newCommands
		res := make(map[string]interface{})
		err := json.Unmarshal(c.data, &res)
		if err != nil {
			log.Printf("Invalid msg received from %v", c.ws)
			continue
		}
		log.Println(res)
	}
}

// Serialize world and send it to all players and remove any player that fails.
func broadcastWorld(tickNumber int) {
	serializedWorld, err := msgpack.Marshal(Packet{
		AllObjects: allObjects,
		TickNumber: tickNumber,
	})
	if err != nil {
		panic(err)
	}

	newClientList := make([]*websocket.Conn, 0, len(clientList))
	for _, ws := range clientList {
		err := ws.WriteMessage(websocket.BinaryMessage, serializedWorld)
		if err != nil {
			log.Printf("Dropping client because of error: %#v", err)
		} else {
			newClientList = append(newClientList, ws)
		}
	}
	clientList = newClientList
}

// The game run in a single-thread environment. Other goroutines write to channels to interoperate with game engine.
func gameLoop() {
	tickPerSecond := 2
	tickInterval := time.Duration(time.Second.Nanoseconds() / int64(tickPerSecond))
	fmt.Println("Tick per sec", tickPerSecond, "each", tickInterval)
	allObjects = append(allObjects, &StaticObject{BaseObject{X: 100, Y: 100, Width: 10, Height:10}})
	allObjects = append(allObjects, &RotatingObject{BaseObject{Height: 2, Width: 2}, 100, 100, 50})

	for tickNumber := 0; ; tickNumber++ {
		tickBegin := time.Now()

		doTick(tickNumber)

		duration := time.Since(tickBegin)
		remaining := time.Duration(tickInterval.Nanoseconds() - duration.Nanoseconds())
		if remaining > 0 {
			fmt.Printf("Tick %v len %v sleeping for %v\n", tickNumber, duration, remaining)
			time.Sleep(remaining)
		} else {
			fmt.Errorf("Tick was too long!! %v\n", remaining)
		}
	}
}
