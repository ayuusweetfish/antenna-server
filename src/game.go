package main

import (
	"fmt"
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
	UserId int
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
	if !t.Timer.Stop() {
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
	t.Timer.Stop()
}

////// Gameplay //////

type GameplayPhaseStatusAssembly struct {
}
type GameplayPhaseStatusAppointment struct {
	Holder int
	Count  int
	Timer  PeekableTimer
}
type GameplayPhaseStatusGamelay struct {
	Holder int
}
type GameplayPlayer struct {
	User
	Profile
}
type GameplayState struct {
	Players     []GameplayPlayer
	PhaseStatus interface{}
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

	// Phase
	var phaseName string
	var statusRepr OrderedKeysMarshal
	switch ps := s.PhaseStatus.(type) {
	case GameplayPhaseStatusAssembly:
		phaseName = "assembly"
		statusRepr = nil

	case GameplayPhaseStatusAppointment:
		phaseName = "appointment"
		statusRepr = OrderedKeysMarshal{
			{"holder", ps.Holder},
			{"timer", ps.Timer.Remaining().Seconds()},
		}

	case GameplayPhaseStatusGamelay:
		phaseName = "gameplay"
		statusRepr = OrderedKeysMarshal{
			{"holder", ps.Holder},
		}
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
			s.PhaseStatus = GameplayPhaseStatusGamelay{
				Holder: luckyDog,
			}
			return st.Holder, luckyDog, true, ""
		}
	} else {
		st.Timer.Stop()
		s.PhaseStatus = GameplayPhaseStatusGamelay{
			Holder: st.Holder,
		}
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

func (r *GameRoom) Lost(userId int) {
	r.Mutex.Lock()
	delete(r.Conns, userId)
	closed := r.Closed
	r.Mutex.Unlock()
	if !closed {
		r.Signal <- GameRoomSignalLostConn{
			UserId: userId,
		}
	}
}

// Assumes the mutex is held (RLock'ed)
func (r *GameRoom) StateMessage(userId int) OrderedKeysMarshal {
	entries := OrderedKeysMarshal{
		{"type", "room_state"},
		{"room", r.Room.Repr()},
		{"players", r.Gameplay.PlayerReprs()},
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
				// TODO
				{"gameplay_status", OrderedKeysMarshal{
					{"user_id", userId},
					{"holder", nextHolder},
				}},
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

	conn = r.Conns[msg.UserId]
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

	createdSignal <- r

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
				println("connection lost", sigLostConn.UserId)
				r.Mutex.Lock()
				delete(r.Conns, sigLostConn.UserId)
				r.Mutex.Unlock()
				if sigLostConn.UserId == room.Creator {
					timeoutTimer.Reset(timeoutDur)
				}
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
