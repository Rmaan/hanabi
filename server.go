package hanabi

import (
	"flag"
	"github.com/gorilla/websocket"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"time"
)

// TODO move HTML to a file!
var homeTemplate = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html>
<head>
<title>HANABI</title>
<link href="/static/css/main.css" rel="stylesheet">
</head>
<meta charset="utf-8">
<body>
<div id="top-bar">
	<div id="status"></div>
	<div id="debug"></div>
	<button id='btn-dc'>DC</button>
</div>
<div id="canvas">
	<div class="desk"></div>
	<div class="hanabis"></div>
	<div class="player-0 player-self"></div>
	<div class="player-1 player-others"></div>
	<div class="player-2 player-others"></div>
	<div class="player-3 player-others"></div>
	<div class="player-4 player-others"></div>
	<div class="player-5 player-others"></div>
</div>
<script>
window.args = {
	"ws_url": {{.}},
}
</script>
<script src="/static/js/pkg/msgpack-lite.min.js"></script>
<script src="/static/js/pkg/underscore-min.js"></script>
<script src="/static/js/client.js"></script>
</body>
</html>
`))

var upgrader = websocket.Upgrader{}
var newClients = make(chan *websocket.Conn, 10)

func serveHome(w http.ResponseWriter, r *http.Request) {
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
}

func NoCache(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		//w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate") // HTTP 1.1.
		w.Header().Set("Cache-Control", "no-cache") // HTTP 1.1.
		w.Header().Set("Pragma", "no-cache")        // HTTP 1.0.
		w.Header().Set("Expires", "0")              // Proxies.

		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func RunServerAndGame() {
	var addr = flag.String("addr", "127.0.0.1:8080", "http service address")
	var tickCount = flag.Int("tick", 20, "How many ticks per seconds will server has.")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	go gameLoop(*tickCount)

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	http.Handle("/static/", NoCache(http.StripPrefix("/static/", http.FileServer(http.Dir("static")))))

	//http.Handle("/static", fs)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
