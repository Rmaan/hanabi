package hanabi

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack"
	"image"
	"log"
	"time"
)

var allObjects = make([]HasShape, 0)
var playerList = make([]*Player, 0)
var newCommands = make(chan playerCommand, 100)

type Player struct {
	ws            *websocket.Conn
	name          string
	isObserver    bool
	readCancelled chan struct{} // Websocket reader goroutine is the sender
	disconnected  chan struct{} // Game goroutine is the sender
}

type playerCommand struct {
	player *Player
	data   []byte
}

var tickNumber int
var passedSeconds float64 // It's tickNumber / ticksPerSecond

type Packet struct {
	AllObjects []HasShape
	TickNumber int
}

func enqueuePlayerCommands(player *Player) {
	defer close(player.readCancelled)
	// player is shared with game. Don't access non-threadsafe fields.
	// gorilla websocket supports one concurrent writer and one concurrent reader.
	for {
		select {
		case <-player.disconnected:
			return
		default:
			mt, message, err := player.ws.ReadMessage()
			if err != nil {
				log.Println("error in WS read:", err)
				return

			} else if mt != websocket.TextMessage {
				log.Printf("Binary message received from player %v", player)
				return

			} else {
				newCommands <- playerCommand{player, message}
			}
		}
	}
}

func joinNewPlayers() {
	for len(newClients) > 0 {
		ws := <-newClients
		player := &Player{
			ws:            ws,
			name:          fmt.Sprintf("Player %d", len(playerList)+1),
			isObserver:    false,
			disconnected:  make(chan struct{}),
			readCancelled: make(chan struct{}),
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

	for len(newCommands) > 0 {
		c := <-newCommands
		err := json.Unmarshal(c.data, &command)
		if err != nil {
			log.Printf("Invalid msg received from %v", c.player)
			continue
		}
		if command.Type == "move" {
			moveCommand := struct {
				X, Y, Target int
				TargetId     uint16
			}{}

			err = json.Unmarshal(command.Params, &moveCommand)
			if err != nil {
				log.Printf("err in move params %v `%s`", err, command.Params)
				continue
			}
			log.Printf("move command %+v", moveCommand)
			if int(moveCommand.TargetId) > len(allObjects) || moveCommand.TargetId == 0 {
				log.Printf("bad obj id to move %v", moveCommand.TargetId)
				continue
			}
			if obj, ok := allObjects[moveCommand.TargetId-1].(Mover); ok {
				obj.setX(moveCommand.X)
				obj.setY(moveCommand.Y)
			} else {
				log.Printf("Moved the unmovable!")
			}
		} else if command.Type == "flip" {
			flipCommand := struct {
				TargetId uint16
			}{}

			err = json.Unmarshal(command.Params, &flipCommand)
			if err != nil {
				log.Printf("err in flip params %v `%s`", err, command.Params)
				continue
			}
			log.Printf("flip command %+v", flipCommand)
			if int(flipCommand.TargetId) > len(allObjects) || flipCommand.TargetId == 0 {
				log.Printf("bad obj id to flip %v", flipCommand.TargetId)
				continue
			}
			if obj, ok := allObjects[flipCommand.TargetId-1].(Flipper); ok {
				obj.flip()
			}
		} else {
			log.Printf("unknown command type %v", command.Type)
		}
	}
}

func serializeWorld() []byte{
	serializedWorld, err := msgpack.Marshal(Packet{
		AllObjects: allObjects,
		TickNumber: tickNumber,
	})
	if err != nil {
		panic(err)
	}
	return serializedWorld
}

func broadcastWorld() {
	var serializedWorld []byte

	for _, player := range playerList {
		select {
		case <-player.disconnected:
			continue
		default:
		}
		// If we merge selects, we may close `disconnected` for a second time.
		select {
		case <-player.readCancelled:
			close(player.disconnected) // Disconnect player when read goroutine stops
		default:
			// Lazy serialization cause it's reduce server load a lot when nobody is connected
			if serializedWorld == nil {
				serializedWorld = serializeWorld()
			}
			err := player.ws.WriteMessage(websocket.BinaryMessage, serializedWorld)
			if err != nil {
				log.Printf("Dropping client because of error: %#v", err)
				close(player.disconnected)
			}
		}
	}
}

const maxWidth = 1000
const maxHeight = 560
var cardsScope = image.Rect(int(maxWidth*0.15), int(maxHeight*0.15), int(maxWidth*0.85), int(maxHeight*0.85))
var fullScope = image.Rect(0, 0, maxWidth, maxHeight)

func initObjects() {
	lastId := 0
	nextId := func() int {
		lastId++
		return lastId
	}

	allObjects = append(allObjects, &StaticObject{BaseObject{Id: nextId(), X: 100, Y: 100, Width: 10, Height: 10}})
	allObjects = append(allObjects, &RotatingObject{BaseObject{Id: nextId(), Height: 2, Width: 2}, 100, 100, 50})

	var allCards []*Card
	var color CardColor
	for color = 0; color < ColorCount; color++ {
		allCards = append(allCards,
			newCard(nextId(), 300, 100, color, 1, &cardsScope),
			newCard(nextId(), 300, 100, color, 1, &cardsScope),
			newCard(nextId(), 300, 100, color, 1, &cardsScope),
			newCard(nextId(), 300, 100, color, 2, &cardsScope),
			newCard(nextId(), 300, 100, color, 2, &cardsScope),
			newCard(nextId(), 300, 100, color, 3, &cardsScope),
			newCard(nextId(), 300, 100, color, 3, &cardsScope),
			newCard(nextId(), 300, 100, color, 4, &cardsScope),
			newCard(nextId(), 300, 100, color, 4, &cardsScope),
			newCard(nextId(), 300, 100, color, 5, &cardsScope),
		)
	}
	for _, c := range allCards {
		allObjects = append(allObjects, c)
	}

	for x := 0; x < 4; x++ {
		allObjects = append(allObjects, newHintToken(nextId(), 300+40*x, 300))
	}
	for x := 0; x < 4; x++ {
		allObjects = append(allObjects, newHintToken(nextId(), 300+40*x, 325))
	}
	for x := 0; x < 3; x++ {
		allObjects = append(allObjects, newMistakeToken(nextId(), 320+40*x, 360))
	}
}

// The game run in a single-thread environment. Other goroutines write to channels to interoperate with game engine.
func gameLoop(tickPerSecond int) {
	tickInterval := time.Duration(time.Second.Nanoseconds() / int64(tickPerSecond))
	fmt.Println("Tick per sec", tickPerSecond, "each", tickInterval)

	initObjects()

	var durationTotal int64 = 0
	for tickNumber = 0; ; tickNumber++ {
		tickBegin := time.Now()
		passedSeconds = float64(tickNumber) / float64(tickPerSecond)

		doTick()

		duration := time.Since(tickBegin)
		durationTotal += duration.Nanoseconds()
		remaining := time.Duration(tickInterval.Nanoseconds() - duration.Nanoseconds())
		if tickNumber % 100 == 0 {
			fmt.Printf("Tick %6v avg tick duration %10v\n", tickNumber, time.Duration(durationTotal / 100))
			durationTotal = 0
		}
		if remaining > 0 {
			time.Sleep(remaining)
		} else {
			fmt.Printf("Tick was too long!! %v\n", remaining)
		}
	}
}
