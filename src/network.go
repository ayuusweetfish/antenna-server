package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

////// Authentication //////

func createAuthToken(userId int) string {
	key := make([]byte, 64)
retry:
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	keyStr := base64.StdEncoding.EncodeToString(key)
	val, err := rcli.SetNX(context.Background(),
		"auth-key:"+keyStr, userId, 7*24*time.Hour).Result()
	if !val {
		// Collision is almost impossible
		// but handle anyway to stay theoretically sound
		goto retry
	}
	if err != nil {
		panic(err)
	}
	return keyStr
}
func validateAuthToken(token string) int {
	if Config.Debug && token[0] == '!' {
		userId, err := strconv.Atoi(token[1:])
		if err == nil {
			return userId
		}
	}
	val, err := rcli.GetEx(context.Background(),
		"auth-key:"+token, 7*24*time.Hour).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0
		}
		panic(err)
	}
	userId, err := strconv.Atoi(val)
	if err != nil {
		panic(err)
	}
	return userId
}
func auth(w http.ResponseWriter, r *http.Request) User {
	cookies := r.Cookies()
	var cookieValue string
	for _, cookie := range cookies {
		if cookie.Name == "auth" {
			cookieValue = cookie.Value
			break
		}
	}
	if cookieValue == "" {
		// Try Authorization header
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) >= 7 && authHeader[0:7] == "Bearer " {
			cookieValue = authHeader[7:]
		}
	}
	if cookieValue == "" {
		panic("401 Authentication required")
	}
	userId := validateAuthToken(cookieValue)
	if userId == 0 {
		panic("401 Authentication required")
	}
	user := User{Id: userId}
	if !user.LoadById() {
		panic("500 Inconsistent databases")
	}
	return user
}

////// Miscellaneous communication //////

func parseIntFromPathValue(r *http.Request, key string) int {
	n, err := strconv.Atoi(r.PathValue(key))
	if err != nil {
		panic("400 Incorrect `" + key + "`")
	}
	return n
}

func postFormValue(r *http.Request, key string, mandatory bool) (string, bool) {
	value, ok := r.PostForm[key]
	if mandatory && !ok {
		panic("400 Missing `" + key + "`")
	}
	if ok {
		return value[0], true
	} else {
		return "", false
	}
}

func parseIntFromPostFormValue(r *http.Request, key string) int {
	value, _ := postFormValue(r, key, true)
	n, err := strconv.Atoi(value)
	if err != nil {
		panic("400 Incorrect `" + key + "`")
	}
	return n
}

type JsonMessage map[string]interface{}

func (obj JsonMessage) String() string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return string(b)
}
func write(w http.ResponseWriter, status int, p interface{}) {
	bytes, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(bytes)
}

////// Handlers //////

func signUpHandler(w http.ResponseWriter, r *http.Request) {
	nickname := r.PostFormValue("nickname")
	password := r.PostFormValue("password")
	if nickname == "" {
		panic("400 Missing nickname")
	}
	if password == "" {
		panic("400 Missing password")
	}
	user := User{
		Nickname: nickname,
		Password: password,
	}
	user.Save()
	write(w, 200, user.Repr())
}

func logInHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PostFormValue("id")
	password := r.PostFormValue("password")
	if id == "" {
		panic("400 Missing id")
	}
	idN, err := strconv.Atoi(id)
	if err != nil {
		panic("400 Incorrect id format")
	}
	if password == "" {
		panic("400 Missing password")
	}
	user := User{Id: idN}
	if !user.LoadById() {
		panic("401 No such user")
	}
	if !user.VerifyPassword(password) {
		panic("401 Incorrect password")
	}

	token := createAuthToken(idN)

	w.Header().Add("Set-Cookie",
		"auth="+token+"; SameSite=Strict; Path=/; Secure; Max-Age=604800")

	write(w, 200, user.Repr())
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	user := auth(w, r)
	write(w, 200, user.Repr())
}

func profileCUHandler(w http.ResponseWriter, r *http.Request, createNew bool) {
	user := auth(w, r)
	if err := r.ParseForm(); err != nil {
		panic("400 Incorrect form format")
	}

	profile := Profile{Id: 0}
	if createNew {
		profile.Creator = user.Id
	} else {
		profile.Id = parseIntFromPathValue(r, "profile_id")
		if !profile.Load() {
			panic("404 No such profile")
		}
		if profile.Creator != user.Id {
			panic("403 Not creator")
		}
	}

	if details, has := postFormValue(r, "details", createNew); has {
		if !json.Valid([]byte(details)) {
			panic("400 `details` is not a valid JSON encoding")
		}
		profile.Details = details
	}
	if stats, has := postFormValue(r, "stats", createNew); has {
		var err error
		profile.Stats, err = parseProfileStats(stats)
		if err != nil {
			panic("400 " + err.Error())
		}
	}
	if traits, has := postFormValue(r, "traits", createNew); has {
		profile.Traits = parseProfileTraits(traits)
	}

	profile.Save()
	write(w, 200, profile.Repr())
}
func profileCreateHandler(w http.ResponseWriter, r *http.Request) {
	profileCUHandler(w, r, true)
}
func profileUpdateHandler(w http.ResponseWriter, r *http.Request) {
	profileCUHandler(w, r, false)
}

func profileDeleteHandler(w http.ResponseWriter, r *http.Request) {
	user := auth(w, r)

	profileId := parseIntFromPathValue(r, "profile_id")
	profile := Profile{Id: profileId}
	if !profile.Load() {
		panic("404 No such profile")
	}
	if profile.Creator != user.Id {
		panic("403 Not creator")
	}

	profile.Delete()
	write(w, 200, JsonMessage{})
}

func profileGetHandler(w http.ResponseWriter, r *http.Request) {
	_ = auth(w, r)

	profileId := parseIntFromPathValue(r, "profile_id")
	profile := Profile{Id: profileId}
	if !profile.Load() {
		panic("404 No such profile")
	}
	/* if profile.Creator != user.Id {
		panic("403 Not creator")
	} */

	write(w, 200, profile.Repr())
}
func avatarHandler(w http.ResponseWriter, r *http.Request) {
	handle := r.PathValue("profile_id")
	fmt.Fprintln(w, "avatar "+handle)
}

func profileListMyHandler(w http.ResponseWriter, r *http.Request) {
	user := auth(w, r)
	write(w, 200, ProfileListByCreatorRepr(user.Id))
}

func roomCUHandler(w http.ResponseWriter, r *http.Request, createNew bool) {
	user := auth(w, r)
	if err := r.ParseForm(); err != nil {
		panic("400 Incorrect form format")
	}

	room := Room{Id: 0}
	if createNew {
		room.Creator = user.Id
		room.CreatedAt = time.Now().Unix()
	} else {
		room.Id = parseIntFromPathValue(r, "room_id")
		if !room.Load() {
			panic("404 No such room")
		}
		if room.Creator != user.Id {
			panic("403 Not creator")
		}
	}

	if title, has := postFormValue(r, "title", createNew); has {
		room.Title = title
	}
	if tags, has := postFormValue(r, "tags", createNew); has {
		room.Tags = tags
	}
	if description, has := postFormValue(r, "description", createNew); has {
		room.Description = description
	}

	room.Save()

	if createNew {
		go GameRoomRun(room, nil)
		if Config.Debug {
			log.Printf("Visit http://localhost:%d/test/%d/%d for testing\n", Config.Port, room.Id, user.Id)
		}
	}

	write(w, 200, room.Repr())
}
func roomCreateHandler(w http.ResponseWriter, r *http.Request) {
	roomCUHandler(w, r, true)
}
func roomUpdateHandler(w http.ResponseWriter, r *http.Request) {
	roomCUHandler(w, r, false)
}

func roomGetHandler(w http.ResponseWriter, r *http.Request) {
	room := Room{
		Id: parseIntFromPathValue(r, "room_id"),
	}
	if !room.Load() {
		panic("404 No such room")
	}
	write(w, 200, room.Repr())
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func roomChannelHandler(w http.ResponseWriter, r *http.Request) {
	user := auth(w, r)

	room := Room{Id: parseIntFromPathValue(r, "room_id")}
	if !room.Load() {
		panic("404 No such room")
	}

	gameRoom := GameRoomFind(room.Id)
	if gameRoom == nil {
		if room.Creator == user.Id {
			// Reopen room
			createdSignal := make(chan *GameRoom)
			go GameRoomRun(room, createdSignal)
			gameRoom = <-createdSignal
		} else {
			panic("404 Room closed")
		}
	}

	// Establish the WebSocket connection
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}

	// Set read limit and timeouts
	c.SetReadLimit(4096)
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	c.SetPongHandler(func(string) error {
		c.SetReadDeadline(time.Now().Add(10 * time.Second))
		return nil
	})

	// `outChannel`: messages to be sent to the client
	outChannel := make(chan interface{}, 4)

	// Add to the room
	gameRoom.Join(user, outChannel)

	// Goroutine that keeps reading JSON from the WebSocket connection
	// and pushes them to `inChannel`
	go func(c *websocket.Conn) {
		var object map[string]interface{}
		for {
			if err := c.ReadJSON(&object); err != nil {
				if !websocket.IsCloseError(err,
					websocket.CloseNormalClosure,
					websocket.CloseGoingAway,
					websocket.CloseNoStatusReceived,
				) && !errors.Is(err, net.ErrClosed) {
					log.Printf("%T %v\n", err, err)
				}
				if _, ok := err.(*json.SyntaxError); ok {
					continue
				}
				break
			}
			gameRoom.InChannel <- GameRoomInMessage{
				UserId:  user.Id,
				Message: object,
			}
		}
	}(c)

	go func(c *websocket.Conn, outChannel chan interface{}) {
		pingTicker := time.NewTicker(5 * time.Second)
		defer pingTicker.Stop()

	messageLoop:
		for {
			select {
			case object := <-outChannel:
				if object == nil {
					// Signal to close connection
					break messageLoop
				}
				if err := c.WriteJSON(object); err != nil {
					log.Println(err)
					break messageLoop
				}

			case <-pingTicker.C:
				if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Println(err)
					break messageLoop
				}
			}
		}

		gameRoom.Lost(user.Id, outChannel)

		c.Close()
		close(outChannel)
	}(c, outChannel)
}

func versionInfoHandler(w http.ResponseWriter, r *http.Request) {
	var vcsRev string
	var vcsTime string
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				vcsRev = s.Value
			} else if s.Key == "vcs.time" {
				vcsTime = s.Value
			}
		}
	}
	fmt.Fprintf(w, "Time %s\nHash %s\n", vcsTime, vcsRev)
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	content, err := os.ReadFile("test.html")
	if err != nil {
		panic("404 Cannot read page content")
	}
	s := string(content)
	s = strings.Replace(s, "~ room ~", r.PathValue("room_id"), 1)
	s = strings.Replace(s, "~ id ~", r.PathValue("player_id"), 1)
	userId, _ := strconv.Atoi(r.PathValue("player_id"))
	s = strings.Replace(s, "~ profile ~",
		strconv.Itoa(ProfileAnyByCreator(userId)), 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(s))
}

func dataInspectionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ReadEverything(w)
}

// A handler that captures panics and return the error message as 500
type errCaptureHandler struct {
	Handler http.Handler
}

func (h *errCaptureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if obj := recover(); obj != nil {
			if err, ok := obj.(error); ok {
				http.Error(w, err.Error(), 500)
			} else if str, ok := obj.(string); ok {
				status := 500
				message := str
				// Try parsing the string `str` into status + message
				index := strings.Index(str, " ")
				if index != -1 {
					if n, err := strconv.Atoi(str[:index]); err == nil {
						status = n
						message = str[(index + 1):]
					}
				}
				http.Error(w, message, status)
			} else {
				message := fmt.Sprintf("%v", obj)
				http.Error(w, message, 500)
			}
		}
	}()
	h.Handler.ServeHTTP(w, r)
}

// A handler that allows all cross-origin requests
type corsAllowAllHandler struct {
	Handler      http.Handler
	StaticServer http.Handler
}

func (h *corsAllowAllHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Upgrade")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	if r.Method == "OPTIONS" {
		// Intercept OPTIONS requests
		w.Write([]byte{})
	} else if strings.HasPrefix(r.URL.Path, "/play/") {
		h.StaticServer.ServeHTTP(w, r)
	} else {
		h.Handler.ServeHTTP(w, r)
	}
}

func ServerListen() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", versionInfoHandler)

	mux.HandleFunc("POST /sign-up", signUpHandler)
	mux.HandleFunc("POST /log-in", logInHandler)
	mux.HandleFunc("GET /me", meHandler)

	mux.HandleFunc("POST /profile/create", profileCreateHandler)
	mux.HandleFunc("POST /profile/{profile_id}/update", profileUpdateHandler)
	mux.HandleFunc("POST /profile/{profile_id}/delete", profileDeleteHandler)
	mux.HandleFunc("GET /profile/{profile_id}", profileGetHandler)
	mux.HandleFunc("GET /profile/{profile_id}/avatar", avatarHandler)
	mux.HandleFunc("GET /profile/my", profileListMyHandler)

	mux.HandleFunc("POST /room/create", roomCreateHandler)
	mux.HandleFunc("POST /room/{room_id}/update", roomUpdateHandler)
	mux.HandleFunc("GET /room/{room_id}", roomGetHandler)
	mux.HandleFunc("GET /room/{room_id}/channel", roomChannelHandler)

	var handler http.Handler
	handler = &errCaptureHandler{Handler: mux}

	if Config.Debug {
		mux.HandleFunc("GET /test", testHandler)
		mux.HandleFunc("GET /test/{room_id}", testHandler)
		mux.HandleFunc("GET /test/{room_id}/{player_id}", testHandler)
		mux.HandleFunc("GET /debug/pprof/", pprof.Index)
		mux.HandleFunc("GET /data", dataInspectionHandler)
		staticHandler := http.StripPrefix("/play",
			http.FileServer(http.Dir("../client")))
		handler = &corsAllowAllHandler{
			Handler:      handler,
			StaticServer: staticHandler,
		}
	}

	port := Config.Port
	log.Printf("Listening on http://localhost:%d/\n", port)
	if Config.Debug {
		log.Printf("Visit http://localhost:%d/play to play")
		log.Printf("Visit http://localhost:%d/debug/pprof/ for profiling stats\n", port)
	}
	server := &http.Server{
		Handler:      handler,
		Addr:         fmt.Sprintf("localhost:%d", port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Print(err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Print(err)
	}
	log.Print("Shutting down")
}
