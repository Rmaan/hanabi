package hanabi

import (
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const templatesDirGlob = "templates/*"

var gameTemplate = template.Must(template.New("game.html").ParseGlob(templatesDirGlob))
var gameListTemplate = template.Must(template.New("gameList.html").ParseGlob(templatesDirGlob))

var upgrader = websocket.Upgrader{}
var activeGames = make(map[int]*Game)
var tickPerSecond int

func panicIfNotNil(err error) {
	if err != nil {
		panic(err)
	}
}

func serveGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.Header().Set("Content-Type", "text/html")
	panicIfNotNil(gameTemplate.Execute(w, fmt.Sprintf("/game/%s/socket", vars["gameId"])))
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameId, err := strconv.Atoi(vars["gameId"])
	game, ok := activeGames[gameId]
	if err != nil || !ok {
		http.Error(w, "invalid game", 404)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	game.newClients <- ws
}

func serveGameList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	panicIfNotNil(gameListTemplate.Execute(w, activeGames))
}

func serveNewGame(w http.ResponseWriter, r *http.Request) {
	game := newGame(tickPerSecond)
	newId := 1e5 + rand.Intn(1e6-1e5)
	// TODO fix data race
	activeGames[newId] = game
	go game.gameLoop()
	http.Redirect(w, r, "/game/"+strconv.Itoa(newId), http.StatusFound)
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
	const staticUrl = "/static/"
	const staticRootPath = "static"

	var addr = flag.String("addr", "127.0.0.1:8080", "http service address")
	flag.IntVar(&tickPerSecond, "tick", 20, "How many ticks per seconds will server has.")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	msgpack.RegisterExt(0, new(Card))

	r := mux.NewRouter()

	r.HandleFunc("/", serveGameList).Methods("GET")
	r.HandleFunc("/game/new", serveNewGame).Methods("GET")
	r.HandleFunc("/game/{gameId:[0-9]+}", serveGame).Methods("GET")
	r.HandleFunc("/game/{gameId:[0-9]+}/socket", serveWs).Methods("GET")
	r.PathPrefix(staticUrl).Handler(NoCache(http.StripPrefix(staticUrl, http.FileServer(http.Dir(staticRootPath)))))

	http.Handle("/", r)
	log.Printf("Listening on %v", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
