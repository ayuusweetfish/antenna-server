package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

////// Connections and signals //////

type WebSocketConn struct {
	User
	OutChannel chan interface{}
}

type GameRoomInMessage struct {
	UserId  int
	Message map[string]interface{}
}

type GameRoomSignalNewConn struct {
	UserId int
}
type GameRoomSignalLostConn struct {
	UserId  int
	Channel chan interface{}
}
type GameRoomSignalReEstConn struct {
	UserId int
}
type GameRoomSignalTimer struct {
	Type string
}

////// Miscellaneous utilities //////

type PeekableTimer struct {
	Timer   *time.Timer
	Expires time.Time
	Func    func()
}

func NewPeekableTimer(d time.Duration) PeekableTimer {
	return PeekableTimer{
		Timer:   time.NewTimer(d),
		Expires: time.Now().Add(d),
		Func:    nil,
	}
}

func NewPeekableTimerFunc(d time.Duration, f func()) PeekableTimer {
	return PeekableTimer{
		Timer:   time.AfterFunc(d, f),
		Expires: time.Now().Add(d),
		Func:    f,
	}
}

func (t PeekableTimer) Remaining() time.Duration {
	return t.Expires.Sub(time.Now())
}

func (t *PeekableTimer) Reset(d time.Duration) {
	if t.Timer == nil || !t.Timer.Stop() {
		if t.Func == nil {
			t.Timer = time.NewTimer(d)
		} else {
			t.Timer = time.AfterFunc(d, t.Func)
		}
	} else {
		t.Timer.Reset(d)
	}
	t.Expires = time.Now().Add(d)
}

func (t *PeekableTimer) Stop() {
	if t.Timer != nil {
		t.Timer.Stop()
	}
}

////// Gameplay //////

type GameplayPhaseStatusAssembly struct {
}
type GameplayPhaseStatusAppointment struct {
	Holder int
	Count  int
	Timer  PeekableTimer
}
type GameplayPhaseStatusGamelayPlayer struct {
	Relationship [][3]int
	ActionPoints int
	Hand         []string
}
type GameplayPhaseStatusGamelay struct {
	ActCount   int
	RoundCount int
	MoveCount  int
	Player     []GameplayPhaseStatusGamelayPlayer
	Arena      []string
	Holder     int
	Step       string

	Timer PeekableTimer
	Queue []int

	Action           string
	Keyword          int
	Target           int
	HolderDifficulty int
	HolderResult     int
	TargetDifficulty int
	TargetResult     int
}

func GameplayPhaseStatusGamelayNew(n int, holder int) GameplayPhaseStatusGamelay {
	players := []GameplayPhaseStatusGamelayPlayer{}
	for i := range n {
		s := fmt.Sprintf("%d", i)
		players = append(players, GameplayPhaseStatusGamelayPlayer{
			Relationship: make([][3]int, n),
			ActionPoints: 1,
			Hand:         []string{"card" + s + "1", "card" + s + "2", "card" + s + "3"},
		})
	}
	arena := []string{"kw1", "kw2", "kw3"}

	return GameplayPhaseStatusGamelay{
		ActCount:   1,
		RoundCount: 1,
		MoveCount:  1,
		Player:     players,
		Arena:      arena,
		Holder:     holder,
		Step:       "selection",

		Timer: NewPeekableTimerFunc(30 * time.Second, func() {
			roomSignalChannel <- GameRoomSignalTimer{Type: "gameplay"}
		}),
		Queue: []int{},

		// Current action irrelevant
	}
}

type GameplayPlayer struct {
	User
	Profile
}
type GameplayState struct {
	Players     []GameplayPlayer
	PhaseStatus interface {
		Repr(userId int) OrderedKeysMarshal
	}
}

func (ps GameplayPhaseStatusAssembly) Repr(playerIndex int) OrderedKeysMarshal {
	return nil
}
func (ps GameplayPhaseStatusAppointment) Repr(playerIndex int) OrderedKeysMarshal {
	return OrderedKeysMarshal{
		{"holder", ps.Holder},
		{"timer", json.Number(fmt.Sprintf("%.1f", ps.Timer.Remaining().Seconds()))},
	}
}
func (ps GameplayPhaseStatusGamelay) Repr(playerIndex int) OrderedKeysMarshal {
	return OrderedKeysMarshal{
		{"act_count", ps.ActCount},
		{"round_count", ps.RoundCount},
		{"move_count", ps.MoveCount},
		{"relationship", ps.Player[playerIndex].Relationship},
		{"action_points", ps.Player[playerIndex].ActionPoints},
		{"hand", ps.Player[playerIndex].Hand},
		{"arena", ps.Arena},
		{"holder", ps.Holder},
		{"step", ps.Step},
		{"timer", json.Number(fmt.Sprintf("%.1f", ps.Timer.Remaining().Seconds()))},
		{"queue", ps.Queue},
	}
}

func (s GameplayState) PlayerReprs() []OrderedKeysMarshal {
	playerReprs := []OrderedKeysMarshal{}
	for _, p := range s.Players {
		if p.Profile.Id > 0 {
			playerReprs = append(playerReprs, p.Profile.Repr())
		} else {
			playerReprs = append(playerReprs, OrderedKeysMarshal{
				{"id", nil},
				{"creator", p.User.Repr()},
			})
		}
	}
	return playerReprs
}
func (s GameplayState) Repr(userId int) OrderedKeysMarshal {
	// Players
	playerReprs := s.PlayerReprs()
	playerIndex := s.PlayerIndex(userId)

	// Phase
	var phaseName string
	var statusRepr OrderedKeysMarshal
	switch ps := s.PhaseStatus.(type) {
	case GameplayPhaseStatusAssembly:
		phaseName = "assembly"
		statusRepr = ps.Repr(playerIndex)

	case GameplayPhaseStatusAppointment:
		phaseName = "appointment"
		statusRepr = ps.Repr(playerIndex)

	case GameplayPhaseStatusGamelay:
		phaseName = "gameplay"
		statusRepr = ps.Repr(playerIndex)
	}

	entries := OrderedKeysMarshal{
		{"players", playerReprs},
		{"phase", phaseName},
	}
	if statusRepr != nil {
		entries = append(entries, OrderedKeysEntry{phaseName + "_status", statusRepr})
	}
	return entries
}

func (s *GameplayState) Seat(user User, profile Profile) string {
	// Check duplicate
	for _, p := range s.Players {
		if p.User.Id == user.Id {
			return "Already seated"
		}
	}
	s.Players = append(s.Players, GameplayPlayer{User: user, Profile: profile})
	return ""
}

func (s *GameplayState) WithdrawSeat(userId int) string {
	for i, p := range s.Players {
		if p.User.Id == userId {
			s.Players = append(s.Players[:i], s.Players[i+1:]...)
			return ""
		}
	}
	return "Not seated"
}

func (s *GameplayState) Start(roomSignalChannel chan interface{}) string {
	if _, ok := s.PhaseStatus.(GameplayPhaseStatusAssembly); !ok {
		return "Not in assembly phase"
	}
	s.PhaseStatus = GameplayPhaseStatusAppointment{
		Holder: CloudRandom(len(s.Players)),
		Count:  0,
		Timer: NewPeekableTimerFunc(30*time.Second, func() {
			roomSignalChannel <- GameRoomSignalTimer{Type: "appointment"}
		}),
	}
	return ""
}

func (s GameplayState) PlayerIndex(userId int) int {
	for i, p := range s.Players {
		if p.User.Id == userId {
			return i
		}
	}
	return -1
}
func (s GameplayState) PlayerIndexNullable(userId int) interface{} {
	i := s.PlayerIndex(userId)
	if i == -1 {
		return nil
	} else {
		return i
	}
}

// A `userId` of -1 means on behealf of the current holder (i.e., skip the holder check)
// Returns:
// - the holder who has selected to skip (-1 denotes null)
// - the next player to hold (if the next return value is `false`)
// - the player to take the first move (if the next return value is `true`)
// - the error message
func (s *GameplayState) AppointmentAcceptOrPass(userId int, accept bool) (int, int, bool, string) {
	var st GameplayPhaseStatusAppointment
	var ok bool
	if st, ok = s.PhaseStatus.(GameplayPhaseStatusAppointment); !ok {
		return -1, -1, false, "Not in appointment phase"
	}
	if userId != -1 && s.Players[st.Holder].User.Id != userId {
		return -1, -1, false, "Not move holder"
	}

	if !accept {
		st.Count++
		if st.Count < 2*len(s.Players) {
			// Continue
			prev := st.Holder
			st.Holder = (st.Holder + 1) % len(s.Players)
			st.Timer.Reset(30 * time.Second)
			s.PhaseStatus = st
			return prev, st.Holder, false, ""
		} else {
			// Random appointment
			st.Timer.Stop()
			luckyDog := CloudRandom(len(s.Players))
			s.PhaseStatus = GameplayPhaseStatusGamelayNew(len(s.Players), luckyDog)
			return st.Holder, luckyDog, true, ""
		}
	} else {
		st.Timer.Stop()
		s.PhaseStatus = GameplayPhaseStatusGamelayNew(len(s.Players), st.Holder)
		return -1, st.Holder, true, ""
	}
}

////// Room //////

type GameRoom struct {
	Room
	Closed    bool
	Conns     map[int]WebSocketConn
	InChannel chan GameRoomInMessage
	Signal    chan interface{}
	Gameplay  GameplayState
	Mutex     *sync.RWMutex
}

var GameRoomMapMutex = &sync.Mutex{}
var GameRoomMap = make(map[int]*GameRoom)

func GameRoomFind(roomId int) *GameRoom {
	return GameRoomMap[roomId]
}

func (r *GameRoom) Join(user User, channel chan interface{}) int {
	r.Mutex.Lock()
	playerIndex := len(r.Conns)
	r.Conns[user.Id] = WebSocketConn{User: user, OutChannel: channel}
	r.Mutex.Unlock()
	r.Signal <- GameRoomSignalNewConn{
		UserId: user.Id,
	}
	return playerIndex
}

func (r *GameRoom) Lost(userId int, channel chan interface{}) {
	r.Mutex.Lock()
	closed := r.Closed
	r.Mutex.Unlock()
	if !closed {
		r.Signal <- GameRoomSignalLostConn{
			UserId:  userId,
			Channel: channel,
		}
	}
}

// Assumes the mutex is held (RLock'ed)
func (r *GameRoom) StateMessage(userId int) OrderedKeysMarshal {
	entries := OrderedKeysMarshal{
		{"type", "room_state"},
		{"room", r.Room.Repr()},
		{"my_index", r.Gameplay.PlayerIndexNullable(userId)},
	}
	entries = append(entries, r.Gameplay.Repr(userId)...)
	return entries
}

func (r *GameRoom) BroadcastStart() {
	r.Mutex.RLock()
	for userId, conn := range r.Conns {
		conn.OutChannel <- OrderedKeysMarshal{
			{"type", "start"},
			{"holder", r.Gameplay.PhaseStatus.(GameplayPhaseStatusAppointment).Holder},
			{"my_index", r.Gameplay.PlayerIndexNullable(userId)},
		}
	}
	r.Mutex.RUnlock()
}

func (r *GameRoom) BroadcastRoomState() {
	r.Mutex.RLock()
	for userId, conn := range r.Conns {
		conn.OutChannel <- r.StateMessage(userId)
	}
	r.Mutex.RUnlock()
}

func (r *GameRoom) BroadcastAssemblyUpdate() {
	r.Mutex.RLock()
	message := OrderedKeysMarshal{
		{"type", "assembly_update"},
		{"players", r.Gameplay.PlayerReprs()},
	}
	for _, conn := range r.Conns {
		conn.OutChannel <- message
	}
	r.Mutex.RUnlock()
}

func (r *GameRoom) BroadcastAppointmentUpdate(prevHolder int, nextHolder int, isStarting bool) {
	r.Mutex.RLock()
	for userId, conn := range r.Conns {
		var message OrderedKeysMarshal
		if isStarting {
			var prevVal interface{}
			if prevHolder == -1 {
				prevVal = nil
			} else {
				prevVal = prevHolder
			}
			message = OrderedKeysMarshal{
				{"type", "appointment_accept"},
				{"prev_holder", prevVal},
				{"gameplay_status",
					r.Gameplay.PhaseStatus.(GameplayPhaseStatusGamelay).Repr(
						r.Gameplay.PlayerIndex(userId))},
			}
		} else {
			message = OrderedKeysMarshal{
				{"type", "appointment_pass"},
				{"prev_holder", prevHolder},
				{"next_holder", nextHolder},
			}
		}
		conn.OutChannel <- message
	}
	r.Mutex.RUnlock()
}

func (r *GameRoom) ProcessMessage(msg GameRoomInMessage) {
	var conn WebSocketConn

	defer func() {
		if obj := recover(); obj != nil {
			var errorMsg string
			if err, ok := obj.(error); ok {
				errorMsg = err.Error()
			} else if str, ok := obj.(string); ok {
				errorMsg = str
			} else {
				errorMsg = fmt.Sprintf("%v", obj)
			}
			conn.OutChannel <- OrderedKeysMarshal{{"error", errorMsg}}
		}
	}()

	message := msg.Message

	r.Mutex.Lock()
	unlock := sync.OnceFunc(r.Mutex.Unlock)
	defer unlock()

	var ok bool
	conn, ok = r.Conns[msg.UserId]
	if !ok {
		log.Printf("Connection handling goes wrong\n")
	}
	if message["type"] == "seat" {
		user := conn.User
		profileId, ok := message["profile_id"].(float64)
		if !ok {
			panic("Incorrect `profile_id`")
		}
		profile := Profile{Id: int(profileId)}
		if !profile.Load() {
			panic("No such profile")
		}
		if profile.Creator != user.Id {
			panic("Not creator")
		}
		if err := r.Gameplay.Seat(user, profile); err != "" {
			panic(err)
		}
		unlock()
		r.BroadcastAssemblyUpdate()
	} else if message["type"] == "withdraw" {
		if err := r.Gameplay.WithdrawSeat(msg.UserId); err != "" {
			panic(err)
		}
		unlock()
		r.BroadcastAssemblyUpdate()
	} else if message["type"] == "start" {
		if msg.UserId != r.Room.Creator {
			panic("Not room creator")
		}
		// Ensure that all present players have seated
		for userId, _ := range r.Conns {
			seated := false
			for _, p := range r.Gameplay.Players {
				if p.User.Id == userId {
					seated = true
					break
				}
			}
			if !seated {
				panic(fmt.Sprintf("Player (ID %d) is not seated", userId))
			}
		}
		if err := r.Gameplay.Start(r.Signal); err != "" {
			panic(err)
		}
		unlock()
		r.BroadcastStart()
	} else if message["type"] == "appointment_accept" || message["type"] == "appointment_pass" {
		prevHolder, nextHolder, isStarting, err :=
			r.Gameplay.AppointmentAcceptOrPass(msg.UserId, message["type"] == "appointment_accept")
		if err != "" {
			panic(err)
		}
		unlock()
		r.BroadcastAppointmentUpdate(prevHolder, nextHolder, isStarting)
	} else {
		panic("Unknown type")
	}
}

// Should be run in a goroutine
func GameRoomRun(room Room, createdSignal chan *GameRoom) {
	GameRoomMapMutex.Lock()
	if _, ok := GameRoomMap[room.Id]; ok {
		GameRoomMapMutex.Unlock()
		return
	}
	r := &GameRoom{
		Room:      room,
		Closed:    false,
		Conns:     map[int]WebSocketConn{},
		InChannel: make(chan GameRoomInMessage, 4),
		Signal:    make(chan interface{}, 2),
		Gameplay: GameplayState{
			Players:     []GameplayPlayer{},
			PhaseStatus: GameplayPhaseStatusAssembly{},
		},
		Mutex: &sync.RWMutex{},
	}
	GameRoomMap[room.Id] = r
	GameRoomMapMutex.Unlock()

	timeoutDur := 180 * time.Second
	timeoutTimer := time.NewTimer(timeoutDur)
	defer timeoutTimer.Stop()

	hahaTicker := time.NewTicker(10 * time.Second)
	defer hahaTicker.Stop()

	if createdSignal != nil {
		createdSignal <- r
	}

loop:
	for {
		select {
		case msg := <-r.InChannel:
			r.ProcessMessage(msg)

		case sig := <-r.Signal:
			if sigNewConn, ok := sig.(GameRoomSignalNewConn); ok {
				if sigNewConn.UserId == room.Creator {
					timeoutTimer.Stop()
				}
				r.Mutex.RLock()
				conn := r.Conns[sigNewConn.UserId]
				message := r.StateMessage(sigNewConn.UserId)
				r.Mutex.RUnlock()
				conn.OutChannel <- message
			}
			if sigLostConn, ok := sig.(GameRoomSignalLostConn); ok {
				r.Mutex.Lock()
				if r.Conns[sigLostConn.UserId].OutChannel == sigLostConn.Channel {
					println("connection lost", sigLostConn.UserId)
					delete(r.Conns, sigLostConn.UserId)
					if sigLostConn.UserId == room.Creator {
						timeoutTimer.Reset(timeoutDur)
					}
				}
				r.Mutex.Unlock()
			}
			if sigTimer, ok := sig.(GameRoomSignalTimer); ok {
				println("timer", sigTimer.Type)
				switch sigTimer.Type {
				case "appointment":
					r.Mutex.Lock()
					prevHolder, nextHolder, isStarting, err :=
						r.Gameplay.AppointmentAcceptOrPass(-1, false)
					r.Mutex.Unlock()
					if err == "" {
						r.BroadcastAppointmentUpdate(prevHolder, nextHolder, isStarting)
					}

				case "gameplay":
					r.Mutex.Lock()
					r.Mutex.Unlock()
				}
			}

		// Debug
		case <-hahaTicker.C:
			r.Mutex.RLock()
			/* n := len(r.Conns)
			for userId, conn := range r.Conns {
				conn.OutChannel <- OrderedKeysMarshal{
					{"message", "haha"},
					{"user_id", userId},
					{"count", n},
				}
			} */
			r.Mutex.RUnlock()

		case <-timeoutTimer.C:
			r.Closed = true
			GameRoomMapMutex.Lock()
			delete(GameRoomMap, room.Id)
			GameRoomMapMutex.Unlock()
			break loop
		}
	}

	// Close all remaining channels
	for _, conn := range r.Conns {
		conn.OutChannel <- nil
	}
}
