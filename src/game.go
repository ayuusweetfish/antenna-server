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

////// Gameplay //////

type GameplayPhaseStatusAssembly struct {
}
type GameplayPhaseStatusAppointment struct {
	Holder int
}
type GameplayPhaseStatusGamelay struct {
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
		}

	case GameplayPhaseStatusGamelay:
		phaseName = "gameplay"
		statusRepr = OrderedKeysMarshal{}
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

func (s *GameplayState) Start() string {
	s.PhaseStatus = GameplayPhaseStatusAppointment{
		Holder: CloudRandom(len(s.Players)),
	}
	return ""
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
	var myIndex interface{}
	myIndex = nil
	for i, p := range r.Gameplay.Players {
		if p.User.Id == userId {
			myIndex = i
			break
		}
	}
	entries := OrderedKeysMarshal{
		{"type", "room_state"},
		{"room", r.Room.Repr()},
		{"my_index", myIndex},
	}
	entries = append(entries, r.Gameplay.Repr(userId)...)
	return entries
}

func (r *GameRoom) BroadcastRoomState(extraMsg interface{}) {
	r.Mutex.RLock()
	for userId, conn := range r.Conns {
		if extraMsg != nil {
			conn.OutChannel <- extraMsg
		}
		conn.OutChannel <- r.StateMessage(userId)
	}
	r.Mutex.RUnlock()
}

func (r *GameRoom) BroadcastAssemblyUpdate() {
	r.Mutex.RLock()
	playerReprs := r.Gameplay.PlayerReprs()
	message := OrderedKeysMarshal{
		{"type", "assembly_update"},
		{"players", playerReprs},
	}
	for _, conn := range r.Conns {
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
		if err := r.Gameplay.Start(); err != "" {
			panic(err)
		}
		unlock()
		r.BroadcastRoomState(OrderedKeysMarshal{
			{"type", "start"},
		})
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
	timeoutTicker := time.NewTicker(timeoutDur)
	defer timeoutTicker.Stop()

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
					timeoutTicker.Stop()
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
					timeoutTicker.Reset(timeoutDur)
				}
			}

		// Debug
		case <-hahaTicker.C:
			r.Mutex.RLock()
			n := len(r.Conns)
			for userId, conn := range r.Conns {
				conn.OutChannel <- OrderedKeysMarshal{
					{"message", "haha"},
					{"user_id", userId},
					{"count", n},
				}
			}
			r.Mutex.RUnlock()

		case <-timeoutTicker.C:
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
