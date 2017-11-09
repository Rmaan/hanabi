package hanabi

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack"
	"image"
	"log"
	"math/rand"
	"reflect"
	"time"
)

var deskObjects = make([]HasShape, 0)
var playerList = make([]*Player, 0)
var newCommands = make(chan playerCommand, 100)

// Positioning
const maxWidth = 1000
const maxHeight = 560
const playerMargin = 0.20

var cardsScope = image.Rect(maxWidth*playerMargin, 0, maxWidth*(1-playerMargin), maxHeight*(1-playerMargin))
var playerScope = image.Rect(maxWidth*playerMargin, maxHeight*(1-playerMargin), maxWidth*(1-playerMargin), maxHeight)
var fullScope = image.Rect(0, 0, maxWidth, maxHeight)

var deck []*Card

var tickNumber int
var passedSeconds float64 // It's tickNumber / ticksPerSecond

type Player struct {
	ws            *websocket.Conn
	name          string
	isObserver    bool
	readCancelled chan struct{} // Websocket reader goroutine is the sender
	disconnected  chan struct{} // Game goroutine is the sender
	// TODO change disconnected to boolean and change related goroutines.
	Cards         []*Card
}

type playerCommand struct {
	player *Player
	data   []byte
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

		cardX := 300
		for x := 0; x < 5; x++ {
			c := getCardFromDeck()
			c.scope = &playerScope
			randPart := rand.Intn(40) - 10
			c.X = cardX + randPart
			cardX += randPart + c.Width
			c.Y = 450
			player.Cards = append(player.Cards, c)
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

	for _, obj := range deskObjects {
		obj.tick()
	}

	broadcastWorld()
}

func processCommands() {
	command := struct {
		Type   string          `json:"type"`
		Params json.RawMessage `json:"params"`
	}{}

	findObjById := func(id int) (HasShape, error) {
		for _, x := range deskObjects {
			if x.getId() == id {
				return x, nil
			}
		}

		for _, p := range playerList {
			for _, x := range p.Cards {
				if x.getId() == id {
					return x, nil
				}
			}
		}
		return nil, fmt.Errorf("Invalid id")
	}

	for len(newCommands) > 0 {
		c := <-newCommands
		err := json.Unmarshal(c.data, &command)
		if err != nil {
			log.Printf("Invalid msg received from %v", c.player)
			continue
		}
		if command.Type == "move" {
			moveCommand := struct {
				X, Y, TargetId int
			}{}

			err = json.Unmarshal(command.Params, &moveCommand)
			if err != nil {
				log.Printf("err in move params %v `%s`", err, command.Params)
				continue
			}
			log.Printf("move command %+v", moveCommand)

			obj, err := findObjById(moveCommand.TargetId)
			if err != nil {
				continue
			}
			if obj, ok := obj.(Mover); ok {
				obj.setX(moveCommand.X)
				obj.setY(moveCommand.Y)
			}
		} else if command.Type == "flip" {
			flipCommand := struct {
				TargetId int
			}{}

			err = json.Unmarshal(command.Params, &flipCommand)
			if err != nil {
				log.Printf("err in flip params %v `%s`", err, command.Params)
				continue
			}
			log.Printf("flip command %+v", flipCommand)
			obj, err := findObjById(flipCommand.TargetId)
			if err != nil {
				continue
			}
			if obj, ok := obj.(Flipper); ok {
				obj.flip()
			}
		} else {
			log.Printf("unknown command type %v", command.Type)
		}
	}
}

func serializeWorld(player *Player) []byte {
	packet := struct {
		DeskObjects  []HasShape
		TickNumber   int
		Players      []*Player
		YourPlayerId int
	}{
		deskObjects,
		tickNumber,
		nil,
		-1,
	}

	// Don't serialize disconnected players. Find user's ID.
	for _, p := range playerList {
		select {
		case <-p.disconnected:
			continue
		default:
		}
		if p == player {
			packet.YourPlayerId = len(packet.Players)
		}
		packet.Players = append(packet.Players, p)
	}

	serializedWorld, err := msgpack.Marshal(packet)
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
			serializedWorld = serializeWorld(player)
			err := player.ws.WriteMessage(websocket.BinaryMessage, serializedWorld)
			if err != nil {
				log.Printf("Dropping client because of error: %#v", err)
				close(player.disconnected)
			}
		}
	}
}

func Shuffle(slice interface{}) {
	rv := reflect.ValueOf(slice)
	swap := reflect.Swapper(slice)
	length := rv.Len()
	for i := length - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		swap(i, j)
	}
}

func getCardFromDeck() *Card {
	card := deck[0]
	deck = deck[1:]
	return card
}

func initObjects() {
	lastId := 0
	nextId := func() int {
	lastId++
	return lastId
}

	deskObjects = append(deskObjects, &StaticObject{BaseObject{Id: nextId(), X: 100, Y: 100, Width: 10, Height: 10}})
	deskObjects = append(deskObjects, &RotatingObject{BaseObject{Id: nextId(), Height: 2, Width: 2}, 100, 100, 50})

	var color CardColor
	const deckX = 300
	const deckY = 100
	for color = 0; color < ColorCount; color++ {
		deck = append(deck,
			newCard(nextId(), deckX, deckY, color, 1, &cardsScope),
			newCard(nextId(), deckX, deckY, color, 1, &cardsScope),
			newCard(nextId(), deckX, deckY, color, 1, &cardsScope),
			newCard(nextId(), deckX, deckY, color, 2, &cardsScope),
			newCard(nextId(), deckX, deckY, color, 2, &cardsScope),
			newCard(nextId(), deckX, deckY, color, 3, &cardsScope),
			newCard(nextId(), deckX, deckY, color, 3, &cardsScope),
			newCard(nextId(), deckX, deckY, color, 4, &cardsScope),
			newCard(nextId(), deckX, deckY, color, 4, &cardsScope),
			newCard(nextId(), deckX, deckY, color, 5, &cardsScope),
		)
	}
	Shuffle(deck)

	for x := 0; x < 4; x++ {
		deskObjects = append(deskObjects, getCardFromDeck())
	}

	for x := 0; x < 4; x++ {
		deskObjects = append(deskObjects, newHintToken(nextId(), 300+40*x, 300))
	}
	for x := 0; x < 4; x++ {
		deskObjects = append(deskObjects, newHintToken(nextId(), 300+40*x, 325))
	}
	for x := 0; x < 3; x++ {
		deskObjects = append(deskObjects, newMistakeToken(nextId(), 320+40*x, 360))
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
		if tickNumber%100 == 0 {
			fmt.Printf("Tick %6v avg tick duration %10v\n", tickNumber, time.Duration(durationTotal/100))
			durationTotal = 0
		}
		if remaining > 0 {
			time.Sleep(remaining)
		} else {
			fmt.Printf("Tick was too long!! %v\n", remaining)
		}
	}
}
