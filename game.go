package hanabi

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack"
	"log"
	"math/rand"
	"reflect"
	"sync"
	"time"
)

const inactivityTickCount = 200 // After this many ticks without commands/join, server will stop broadcasting until another command arrives.

type Game struct {
	tickNumber       int
	tickPerSecond    int
	lastActivityTick int

	playerList  []*Player
	newCommands chan playerCommandRaw

	deskObjects           []*BaseObject
	deck                  []*Card
	successfulPlayedCount [ColorCount]int
	discardedCount        int
	hintTokenCount        int
	mistakeTokenCount     int
	newChats []struct{*Player; string}
}

func newGame(tickPerSecond int) *Game {
	g := &Game{
		newCommands: make(chan playerCommandRaw, 100),
		hintTokenCount:    8,
		mistakeTokenCount: 3,
		tickPerSecond:     tickPerSecond,
	}

	lastId := 0
	nextId := func() int {
		lastId++
		return lastId
	}

	g.deskObjects = append(g.deskObjects, &BaseObject{Id: nextId(), Height: 2, Width: 2, centerX: 200, centerY: 100, radius: 40})

	var color CardColor
	for color = 1; color <= ColorCount; color++ {
		g.deck = append(g.deck,
			newCard(nextId(), color, 1),
			newCard(nextId(), color, 1),
			newCard(nextId(), color, 1),
			newCard(nextId(), color, 2),
			newCard(nextId(), color, 2),
			newCard(nextId(), color, 3),
			newCard(nextId(), color, 3),
			newCard(nextId(), color, 4),
			newCard(nextId(), color, 4),
			newCard(nextId(), color, 5),
		)
	}
	Shuffle(g.deck)
	return g
}

func (g *Game) passedSeconds() float64 {
	return float64(g.tickNumber) / float64(g.tickPerSecond)
}

type Player struct {
	ws         *websocket.Conn // ws == nil means player is disconnected
	wsMutex    sync.Mutex      // Hold this if you want to read/modify ws (the pointer itself)
	Name       string
	isObserver bool
	Cards      []*Card
}

func (p *Player) disconnect() {
	p.wsMutex.Lock()
	p.ws = nil
	p.wsMutex.Unlock()
}

type playerCommandRaw struct {
	player *Player
	data   []byte
}

func (g *Game) enqueuePlayerCommands(player *Player) {
	// player is shared with game. Don't access non-threadsafe fields.
	// gorilla websocket supports one concurrent writer and one concurrent reader.
	for {
		player.wsMutex.Lock()
		ws := player.ws
		player.wsMutex.Unlock()
		if ws == nil {
			return
		}

		mt, message, err := ws.ReadMessage()
		if err != nil {
			log.Println("error in WS read:", err)
			player.disconnect()
			return
		} else if mt != websocket.TextMessage {
			log.Printf("Binary message received from player %v", player)
			player.disconnect()
			return
		} else {
			g.newCommands <- playerCommandRaw{player, message}
		}
	}
}

func (g *Game) getPlayerForSocket(ws *websocket.Conn) *Player {
	// Reconnect socket to a disconnected player or create a new player if no disconnected player is available.

	// Reconnect player to first disconnected player
	for _, p := range g.playerList {
		p.wsMutex.Lock()
		if p.ws == nil {
			p.ws = ws
			p.wsMutex.Unlock()
			return p
		}
		p.wsMutex.Unlock()
	}

	player := &Player{
		ws:         ws,
		Name:       fmt.Sprintf("Player %d", len(g.playerList)+1),
		isObserver: false,
	}

	for x := 0; x < 5; x++ {
		c := g.getCardFromDeck()
		player.Cards = append(player.Cards, c)
	}

	g.playerList = append(g.playerList, player)
	return player
}

func (g *Game) joinNewClients() {
	for len(newClients) > 0 {
		ws := <-newClients
		g.lastActivityTick = g.tickNumber
		player := g.getPlayerForSocket(ws)

		if !player.isObserver {
			go g.enqueuePlayerCommands(player)
		}
	}
}

func (g *Game) doTick() {
	g.joinNewClients()

	g.processAllCommands()

	for _, obj := range g.deskObjects {
		obj.tick(g.passedSeconds())
	}

	g.broadcastWorld()
}

func (g *Game) doCommand(player *Player, commandType string, params json.RawMessage) error {
	if commandType == "hint" {
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
		hintCommand.PlayerId = clamp(hintCommand.PlayerId, 1, len(g.playerList)-1) // Can't hint himself!

		thisPlayerIndex := -1
		for idx, p := range g.playerList {
			if p == player {
				thisPlayerIndex = idx
			}
		}
		// Translate to absolute ID space
		hintCommand.PlayerId = (hintCommand.PlayerId + thisPlayerIndex) % len(g.playerList)
		targetPlayer := g.playerList[hintCommand.PlayerId]
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
		g.hintTokenCount--
	} else if commandType == "discard" {
		discardCommand := struct {
			CardIndex int
		}{}
		err := json.Unmarshal(params, &discardCommand)
		if err != nil {
			return fmt.Errorf("err in params %v `%s`", err, params)
		}
		discardCommand.CardIndex = clamp(discardCommand.CardIndex, 0, len(player.Cards)-1)
		// Put the new card at the end to be consistent with UI
		player.Cards = append(append(player.Cards[0:discardCommand.CardIndex], player.Cards[discardCommand.CardIndex+1:]...), g.getCardFromDeck())
		g.discardedCount++
		g.hintTokenCount++
	} else if commandType == "play" {
		playCommand := struct {
			CardIndex int
		}{}
		err := json.Unmarshal(params, &playCommand)
		if err != nil {
			return fmt.Errorf("err in params %v `%s`", err, params)
		}
		playCommand.CardIndex = clamp(playCommand.CardIndex, 0, len(player.Cards)-1)
		card := player.Cards[playCommand.CardIndex]
		// Put the new card at the end to be consistent with UI
		player.Cards = append(append(player.Cards[0:playCommand.CardIndex], player.Cards[playCommand.CardIndex+1:]...), g.getCardFromDeck())

		if g.successfulPlayedCount[card.Color-1] == card.Number-1 {
			g.successfulPlayedCount[card.Color-1]++
			if card.Number == NumberMax {
				g.hintTokenCount++
			}
		} else {
			g.mistakeTokenCount--
		}
	} else if commandType == "rename" {
		renameCommand := struct {
			NewName string
		}{}
		err := json.Unmarshal(params, &renameCommand)
		if err != nil {
			return fmt.Errorf("err in params %v `%s`", err, params)
		}
		l := len(renameCommand.NewName)
		if l == 0 || l > 100 {
			return fmt.Errorf("invalid name length")
		}
		player.Name = renameCommand.NewName
	} else {
		return fmt.Errorf("unknown command type %v", commandType)
	}
	return nil
}

func (g *Game) processAllCommands() {
	for len(g.newCommands) > 0 {
		c := <-g.newCommands
		g.lastActivityTick = g.tickNumber

		command := struct {
			Type   string          `json:"type"`
			Params json.RawMessage `json:"params"`
		}{}

		err := json.Unmarshal(c.data, &command)
		if err != nil {
			log.Printf("Invalid msg received from %+v", c.player)
			continue
		}

		err = g.doCommand(c.player, command.Type, command.Params)
		if err != nil {
			log.Printf("error in doing command: %s", err.Error())
		}
	}
}

func (p *Card) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.Encode([]interface{}{p.Id, p.Color, p.ColorHinted, p.Number, p.NumberHinted})
}

func (g *Game) serializeWorld(player *Player) []byte {
	packet := struct {
		DeskObjects           []*BaseObject
		TickNumber            int
		Players               []*Player
		SuccessfulPlayedCount [5]int
		DiscardedCount        int
		HintTokenCount        int
		MistakeTokenCount     int
		RemainingDeckCount    int
	}{
		g.deskObjects,
		g.tickNumber,
		nil,
		g.successfulPlayedCount,
		g.discardedCount,
		g.hintTokenCount,
		g.mistakeTokenCount,
		len(g.deck),
	}

	playerId := -1

	for idx, p := range g.playerList {
		if p == player {
			playerId = idx
		}
	}

	// Order players array so each player thinks he is the first player.
	players := append([]*Player{}, g.playerList[playerId:]...)
	players = append(players, g.playerList[:playerId]...)
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

func (g *Game) broadcastWorld() {
	if g.tickNumber-g.lastActivityTick > inactivityTickCount {
		return
	}
	var serializedWorld []byte

	for _, player := range g.playerList {
		player.wsMutex.Lock()
		ws := player.ws
		player.wsMutex.Unlock()

		if ws == nil {
			continue
		}

		serializedWorld = g.serializeWorld(player)
		err := ws.WriteMessage(websocket.BinaryMessage, serializedWorld)
		if err != nil {
			log.Printf("Dropping client because of error: %#v", err)
			player.disconnect()
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

func (g *Game) getCardFromDeck() *Card {
	card := g.deck[0]
	g.deck = g.deck[1:]
	return card
}

// The game run in a single-thread environment. Other goroutines write to channels to interoperate with game engine.
func (g *Game) gameLoop() {
	tickInterval := time.Duration(time.Second.Nanoseconds() / int64(g.tickPerSecond))
	fmt.Println("Tick per sec", g.tickPerSecond, "each", tickInterval)

	msgpack.RegisterExt(0, new(Card))

	var durationTotal int64 = 0
	for g.tickNumber = 0; ; g.tickNumber++ {
		tickBegin := time.Now()

		g.doTick()

		duration := time.Since(tickBegin)
		durationTotal += duration.Nanoseconds()
		remaining := time.Duration(tickInterval.Nanoseconds() - duration.Nanoseconds())
		if g.tickNumber%100 == 0 {
			fmt.Printf("Tick %6v avg tick duration %10v\n", g.tickNumber, time.Duration(durationTotal/100))
			durationTotal = 0
		}
		if remaining > 0 {
			time.Sleep(remaining)
		} else {
			fmt.Printf("Tick was too long!! %v\n", remaining)
		}
	}
}
