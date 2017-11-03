package hanabi

import (
	"fmt"
	"time"
	"github.com/gorilla/websocket"
	"log"
	"encoding/json"
)


var allObjects = make([]MovingObject, 0)
var clientList = make([]*websocket.Conn, 0)

type MovingObject struct {
	X, Y int16
}


type Packet struct {
	AllObjects []MovingObject
	TickNumber int
}

func joinNewPlayers() {
	for len(newClients) > 0 {
		ws := <-newClients
		_ = ws.WriteJSON("Welcome!")
		clientList = append(clientList, ws)
	}
}

func doTick(tickNumber int) {
	joinNewPlayers()
	//_, err := msgpack.Marshal(Packet{
	//	allObjects: allObjects,
	//	tickNumber: tickNumber,
	//})
	data, err := json.Marshal(Packet{
		AllObjects: allObjects,
		TickNumber: tickNumber,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", data)
	for _, ws := range clientList {
		err = ws.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Printf("Client disconnected")
		}
	}
}


func gameLoop() {
	tickPerSecond := 2
	tickInterval := time.Duration(time.Second.Nanoseconds() / int64(tickPerSecond))
	fmt.Println("Tick per sec", tickPerSecond, "each", tickInterval)

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
