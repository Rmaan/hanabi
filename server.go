package hanabi

import (
	"net/http"
	"log"
	"flag"
	"github.com/gorilla/websocket"
	"html/template"
)

var homeTemplate = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html>
<head>
<title>HANABI</title>
<link href="/static/css/main.css" rel="stylesheet">
</head>
<meta charset="utf-8">
<body>
<div id="tick"></div>
<div id="canvas"></div>
<script>
window.args = {
	"ws_url": {{.}},
}
</script>
<script src="/static/js/client.js"></script>
</body>
</html>
`))


var upgrader = websocket.Upgrader{}
var newClients = make(chan *websocket.Conn, 10)

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println("aaaa")
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	homeTemplate.Execute(w, "ws://"+r.Host+"/ws")

	//http.ServeFile(w, r, "home.html")
	//w.Write([]byte("HI HI HI!\n"))
}

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

func RunServerAndGame() {
	var addr = flag.String("addr", "127.0.0.1:8080", "http service address")
	flag.Parse()
	go gameLoop()

	fs := http.FileServer(http.Dir("static"))
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	//http.Handle("/static", fs)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
