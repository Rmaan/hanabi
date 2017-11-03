package main

import (
	"fmt"
	"time"
	//"github.com/vmihailenco/msgpack"
	"github.com/gorilla/websocket"
	"flag"
	"log"
	"net/http"
	"html/template"
	"encoding/json"
)

var homeTemplate = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>
window.addEventListener("load", function(evt) {
	var ws = new WebSocket("{{.}}");
	window.ws = ws;
	ws.onopen = function(evt) {
		console.log("OPEN");
	}
	ws.onclose = function(evt) {
		console.log("CLOSE");
	}
	ws.onmessage = function(evt) {
		console.log("RESPONSE: " + evt.data);
	}
	ws.onerror = function(evt) {
		console.log("ERROR: " + evt.data);
	}
	//ws.close();
});
</script>
</head>
<body>
</body>
</html>
`))

type MovingObject struct {
	X, Y int16
}

var allObjects = make([]MovingObject, 0)

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
		ws.WriteMessage(websocket.TextMessage, data)
	}
}

var addr = flag.String("addr", "127.0.0.1:8080", "http service address")

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

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	//w.Header().Set("Content-Type", "text/html")
	homeTemplate.Execute(w, "ws://"+r.Host+"/ws")

	//http.ServeFile(w, r, "home.html")
	//w.Write([]byte("HI HI HI!\n"))
}

var upgrader = websocket.Upgrader{}
var clientList = make([]*websocket.Conn, 0)
var newClients = make(chan *websocket.Conn, 10)

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	//defer ws.Close()

	newClients <- ws
	//for {
	//	mt, message, err := ws.ReadMessage()
	//	if err != nil {
	//		log.Println("read:", err)
	//		break
	//	}
	//	log.Printf("recv: %s", message)
	//	err = ws.WriteMessage(mt, message)
	//	if err != nil {
	//		log.Println("write:", err)
	//		break
	//	}
	//}
}

func main() {
	flag.Parse()
	go gameLoop()

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
