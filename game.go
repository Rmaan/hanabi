package hanabi

import (
	"fmt"
	"time"
	"github.com/gorilla/websocket"
	"log"
	"github.com/vmihailenco/msgpack"
	"encoding/json"
)

var allObjects = make([]HasShape, 0)
var clientList = make([]*websocket.Conn, 0)
var newCommands = make(chan playerCommand, 100)

type playerCommand struct {
	ws   *websocket.Conn
	data []byte
}

const maxWidth = 1000
const maxHeight = 560


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
	command := struct {
		Type   string          `json:"type"`
		Params json.RawMessage `json:"params"`
	}{}

	moveCommand := struct {
		X, Y, Target int16
		TargetId uint16
	}{}

	for len(newCommands) > 0 {
		c := <-newCommands
		err := json.Unmarshal(c.data, &command)
		if err != nil {
			log.Printf("Invalid msg received from %v", c.ws)
			continue
		}
		if command.Type == "move" {
			err = json.Unmarshal(command.Params, &moveCommand)
			if err != nil {
				log.Printf("err in move %v `%s`", err, command.Params)
				continue
			}
			log.Printf("move command %+v", moveCommand)
			if int(moveCommand.TargetId) > len(allObjects) || moveCommand.TargetId == 0 {
				log.Printf("bad obj id to move %v", moveCommand.TargetId)
				continue
			}
			obj := allObjects[moveCommand.TargetId - 1].(*Card)
			obj.X = moveCommand.X
			obj.Y = moveCommand.Y
		}
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
func gameLoop(tickPerSecond int) {
	tickInterval := time.Duration(time.Second.Nanoseconds() / int64(tickPerSecond))
	fmt.Println("Tick per sec", tickPerSecond, "each", tickInterval)

	nextId := func () int16 {
		return int16(len(allObjects) + 1)
	}

	allObjects = append(allObjects, &StaticObject{BaseObject{Id: nextId(), X: 100, Y: 100, Width: 10, Height: 10}})
	allObjects = append(allObjects, &RotatingObject{BaseObject{Id: nextId(), Height: 2, Width: 2}, 100, 100, 50})
	allObjects = append(allObjects, &Card{BaseObject{Id: nextId(), X: 300, Y: 100, Width: 100, Height: 140}, 2})
	allObjects = append(allObjects, &Card{BaseObject{Id: nextId(), X: 300, Y: 300, Width: 100, Height: 140}, 32})

	for tickNumber := 0; ; tickNumber++ {
		tickBegin := time.Now()

		doTick(tickNumber)

		duration := time.Since(tickBegin)
		remaining := time.Duration(tickInterval.Nanoseconds() - duration.Nanoseconds())
		if remaining > 0 {
			fmt.Printf("Tick %6v done in %10v\n", tickNumber, duration)
			time.Sleep(remaining)
		} else {
			fmt.Printf("Tick was too long!! %v\n", remaining)
		}
	}
}
