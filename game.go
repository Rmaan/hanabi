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
var newCommands = make(chan playerCommandRaw, 100)

// Positioning
const maxWidth = 1000
const maxHeight = 560
const playerMargin = 0.20

var cardsScope = image.Rect(maxWidth*playerMargin, 0, maxWidth*(1-playerMargin), maxHeight*(1-playerMargin))
var playerScope = image.Rect(maxWidth*playerMargin, maxHeight*(1-playerMargin), maxWidth*(1-playerMargin), maxHeight)
var fullScope = image.Rect(0, 0, maxWidth, maxHeight)

var deck []*Card
var successfulPlayedCount [ColorCount]int
var unsuccessfulPlayedCount int
var discardedCount int

var tickNumber int
var passedSeconds float64 // It's tickNumber / ticksPerSecond
var lastActivityTick = 0

const inactivityTickCount = 200 // After this many ticks without commands/join, server will stop broadcasting until another command arrives.

type Player struct {
	ws            *websocket.Conn
	name          string
	isObserver    bool
	readCancelled chan struct{} // Websocket reader goroutine is the sender
	disconnected  chan struct{} // Game goroutine is the sender
	// TODO change disconnected to boolean and change related goroutines.
	Cards []*Card
}

type playerCommandRaw struct {
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
				newCommands <- playerCommandRaw{player, message}
			}
		}
	}
}

func joinNewPlayers() {
	for len(newClients) > 0 {
		ws := <-newClients
		lastActivityTick = tickNumber
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

	processAllCommands()

	for _, obj := range deskObjects {
		obj.tick()
	}

	broadcastWorld()
}

func findObjById(id int) (HasShape, error) {
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
	return nil, fmt.Errorf("Invalid object id")
}

func doCommand(player *Player, commandType string, params json.RawMessage) error {
	if commandType == "move" {
		moveCommand := struct {
			X, Y, TargetId int
		}{}

		err := json.Unmarshal(params, &moveCommand)
		if err != nil {
			return fmt.Errorf("err in params %v `%s`", err, params)
		}
		log.Printf("move command %+v", moveCommand)

		obj, err := findObjById(moveCommand.TargetId)
		if err != nil {
			return err
		}
		if obj, ok := obj.(Mover); ok {
			obj.setX(moveCommand.X)
			obj.setY(moveCommand.Y)
		}
	} else if commandType == "flip" {
		flipCommand := struct {
			TargetId int
		}{}

		err := json.Unmarshal(params, &flipCommand)
		if err != nil {
			return fmt.Errorf("err in params %v `%s`", err, params)
		}
		log.Printf("flip command %+v", flipCommand)
		obj, err := findObjById(flipCommand.TargetId)
		if err != nil {
			return err
		}
		if obj, ok := obj.(Flipper); ok {
			obj.flip()
		}
	} else if commandType == "hint" {
		hintCommand := struct {
			PlayerId int
			IsColor  bool
			Value    int
		}{}

		err := json.Unmarshal(params, &hintCommand)
		if err != nil {
			return fmt.Errorf("err in params %v `%s`", err, params)
		}
		hintCommand.Value = clamp(hintCommand.Value, NumberMin, NumberMax)
		hintCommand.PlayerId = clamp(hintCommand.PlayerId, 1, len(playerList)-1) // Can't hint himself!

		thisPlayerIndex := -1
		for idx, p := range playerList {
			if p == player {
				thisPlayerIndex = idx
			}
		}
		// Translate to absolute ID space
		hintCommand.PlayerId = (hintCommand.PlayerId + thisPlayerIndex) % len(playerList)
		targetPlayer := playerList[hintCommand.PlayerId]
		for _, c := range targetPlayer.Cards {
			if hintCommand.IsColor {
				if int(c.Color) == hintCommand.Value {
					c.ColorHinted = true
				}
			} else {
				if c.Number == hintCommand.Value {
					c.NumberHinted = true
				}
			}
		}
	} else if commandType == "discard" {
		discardCommand := struct {
			CardIndex    int
		}{}
		err := json.Unmarshal(params, &discardCommand)
		if err != nil {
			return fmt.Errorf("err in params %v `%s`", err, params)
		}
		discardCommand.CardIndex = clamp(discardCommand.CardIndex, 0, len(player.Cards) - 1)
		// Put the new card at the end to be consistent with UI
		player.Cards = append(append(player.Cards[0:discardCommand.CardIndex], player.Cards[discardCommand.CardIndex + 1:]...), getCardFromDeck())
		discardedCount++
	} else if commandType == "play" {
		playCommand := struct {
			CardIndex    int
		}{}
		err := json.Unmarshal(params, &playCommand)
		if err != nil {
			return fmt.Errorf("err in params %v `%s`", err, params)
		}
		playCommand.CardIndex = clamp(playCommand.CardIndex, 0, len(player.Cards) - 1)
		card := player.Cards[playCommand.CardIndex]
		// Put the new card at the end to be consistent with UI
		player.Cards = append(append(player.Cards[0:playCommand.CardIndex], player.Cards[playCommand.CardIndex + 1:]...), getCardFromDeck())

		if successfulPlayedCount[card.Color - 1] == card.Number - 1 {
			successfulPlayedCount[card.Color - 1]++
		} else {
			unsuccessfulPlayedCount++
		}
	} else {
		return fmt.Errorf("unknown command type %v", commandType)
	}
	return nil
}

func processAllCommands() {
	for len(newCommands) > 0 {
		c := <-newCommands
		lastActivityTick = tickNumber

		command := struct {
			Type   string          `json:"type"`
			Params json.RawMessage `json:"params"`
		}{}

		err := json.Unmarshal(c.data, &command)
		if err != nil {
			log.Printf("Invalid msg received from %+v", c.player)
			continue
		}

		err = doCommand(c.player, command.Type, command.Params)
		if err != nil {
			log.Printf("error in doing command: %s", err.Error())
		}
	}
}

func (p *Card) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.Encode([]interface{}{p.Id, p.X, p.Y, p.Width, p.Height, p.Color, p.ColorHinted, p.Number, p.NumberHinted})
}

func serializeWorld(player *Player) []byte {
	packet := struct {
		DeskObjects []HasShape
		TickNumber  int
		Players     []*Player
		SuccessfulPlayedCount [5]int
		UnsuccessfulPlayedCount int
		DiscardedCount int
	}{
		deskObjects,
		tickNumber,
		nil,
		successfulPlayedCount,
		unsuccessfulPlayedCount,
		discardedCount,
	}

	playerId := -1
	var players []*Player

	// Don't serialize disconnected players. Find user's ID.
	for _, p := range playerList {
		select {
		case <-p.disconnected:
			continue
		default:
		}
		if p == player {
			playerId = len(players)
		}
		players = append(players, p)
	}
	// Order players array so each player thinks he is the first player.
	players = append(players[playerId:], players[:playerId]...)
	packet.Players = players

	// Conceal player cards' number/color
	hiddenCards := []*Card(nil)
	for _, c := range player.Cards {
		copy := *c // Shallow copy card
		if !c.ColorHinted {
			copy.Color = 0
		}
		if !c.NumberHinted {
			copy.Number = 0
		}
		hiddenCards = append(hiddenCards, &copy)
	}
	tmp := player.Cards
	player.Cards = hiddenCards

	serializedWorld, err := msgpack.Marshal(packet)
	player.Cards = tmp

	if err != nil {
		panic(err)
	}
	return serializedWorld
}

func broadcastWorld() {
	if tickNumber-lastActivityTick > inactivityTickCount {
		return
	}
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
	for color = 1; color <= ColorCount; color++ {
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

	msgpack.RegisterExt(0, new(Card))

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
