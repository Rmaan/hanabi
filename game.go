package hanabi

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack"
	"log"
	"time"
)

var allObjects = make([]HasShape, 0)
var playerList = make([]*Player, 0)
var newCommands = make(chan playerCommand, 100)

type Player struct {
	ws             *websocket.Conn
	name           string
	isObserver     bool
	isDisconnected bool
}

type playerCommand struct {
	player *Player
	data   []byte
}

var tickNumber int
var passedSeconds float64 // It's tickNumber / ticksPerSecond

const maxWidth = 1000
const maxHeight = 560

type Packet struct {
	AllObjects []HasShape
	TickNumber int
}

func enqueuePlayerCommands(player *Player) {
	// player is shared with game goroutine without lock take care!
	for !player.isDisconnected {
		mt, message, err := player.ws.ReadMessage()
		if err != nil {
			player.isDisconnected = true
			log.Println("error in WS read:", err)

		} else if mt != websocket.TextMessage {
			player.isDisconnected = true
			log.Printf("Binary message received from player %v", player)

		} else {
			newCommands <- playerCommand{player, message}
		}
	}
}

func joinNewPlayers() {
	for len(newClients) > 0 {
		ws := <-newClients
		player := &Player{
			ws:         ws,
			name:       fmt.Sprintf("Player %d", len(playerList)+1),
			isObserver: false,
		}
		playerList = append(playerList, player)
		if !player.isObserver {
			go enqueuePlayerCommands(player)
		}
	}
}

func doTick() {
	joinNewPlayers()

	processCommands()
	for _, obj := range allObjects {
		obj.tick()
	}

	broadcastWorld()
}

func processCommands() {
	command := struct {
		Type   string          `json:"type"`
		Params json.RawMessage `json:"params"`
	}{}

	moveCommand := struct {
		X, Y, Target int16
		TargetId     uint16
	}{}

	for len(newCommands) > 0 {
		c := <-newCommands
		err := json.Unmarshal(c.data, &command)
		if err != nil {
			log.Printf("Invalid msg received from %v", c.player)
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
			obj := allObjects[moveCommand.TargetId-1].(*Card)
			obj.X = moveCommand.X
			obj.Y = moveCommand.Y
		}
	}
}

// Serialize world and send it to all players and remove any player that fails.
func broadcastWorld() {
	serializedWorld, err := msgpack.Marshal(Packet{
		AllObjects: allObjects,
		TickNumber: tickNumber,
	})
	if err != nil {
		panic(err)
	}

	for _, player := range playerList {
		if player.isDisconnected {
			continue
		}
		err := player.ws.WriteMessage(websocket.BinaryMessage, serializedWorld)
		if err != nil {
			log.Printf("Dropping client because of error: %#v", err)
			player.isDisconnected = true
		}
	}
}

// The game run in a single-thread environment. Other goroutines write to channels to interoperate with game engine.
func gameLoop(tickPerSecond int) {
	tickInterval := time.Duration(time.Second.Nanoseconds() / int64(tickPerSecond))
	fmt.Println("Tick per sec", tickPerSecond, "each", tickInterval)

	nextId := func() int16 {
		return int16(len(allObjects) + 1)
	}

	allObjects = append(allObjects, &StaticObject{BaseObject{Id: nextId(), X: 100, Y: 100, Width: 10, Height: 10}})
	allObjects = append(allObjects, &RotatingObject{BaseObject{Id: nextId(), Height: 2, Width: 2}, 100, 100, 50})
	allObjects = append(allObjects, &Card{BaseObject{Id: nextId(), X: 300, Y: 100, Width: 100, Height: 140}, 2})
	allObjects = append(allObjects, &Card{BaseObject{Id: nextId(), X: 300, Y: 300, Width: 100, Height: 140}, 32})

	for tickNumber = 0; ; tickNumber++ {
		tickBegin := time.Now()
		passedSeconds = float64(tickNumber) / float64(tickPerSecond)

		doTick()

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
