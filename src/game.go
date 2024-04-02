package main

import (
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
	holder int
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

func (s GameplayState) Repr(userId int) OrderedKeysMarshal {
	// Players
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
			{"holder", ps.holder},
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

var GameRoomDataMutex = &sync.Mutex{}
var GameRoomMap = make(map[string]*GameRoom)

func GameRoomFind(roomId string) *GameRoom {
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
	user := r.Conns[userId].User
	entries := OrderedKeysMarshal{
		{"user", user.Repr()}, // Debug usage
		{"room", r.Room.Repr()},
	}
	entries = append(entries, r.Gameplay.Repr(userId)...)
	return entries
}

// Should be run in a goroutine
func GameRoomRun(room Room) {
	GameRoomDataMutex.Lock()
	if _, ok := GameRoomMap[room.Id]; ok {
		GameRoomDataMutex.Unlock()
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
	GameRoomDataMutex.Unlock()

	timeoutDur := 30 * time.Second
	timeoutTicker := time.NewTicker(timeoutDur)
	defer timeoutTicker.Stop()

	hahaTicker := time.NewTicker(10 * time.Second)
	defer hahaTicker.Stop()

loop:
	for {
		select {
		case msg := <-r.InChannel:
			r.Mutex.RLock()
			conn := r.Conns[msg.UserId]
			r.Mutex.RUnlock()
			conn.OutChannel <- OrderedKeysMarshal{
				{"message", "received"},
				{"object", msg.Message},
			}

		case sig := <-r.Signal:
			if sigNewConn, ok := sig.(GameRoomSignalNewConn); ok {
				timeoutTicker.Stop()
				r.Mutex.RLock()
				conn := r.Conns[sigNewConn.UserId]
				message := r.StateMessage(sigNewConn.UserId)
				r.Mutex.RUnlock()
				conn.OutChannel <- message
			}
			if sigNewConn, ok := sig.(GameRoomSignalLostConn); ok {
				println("connection lost ", sigNewConn.UserId)
				r.Mutex.RLock()
				n := len(r.Conns)
				r.Mutex.RUnlock()
				if n == 0 {
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
			GameRoomDataMutex.Lock()
			delete(GameRoomMap, room.Id)
			GameRoomDataMutex.Unlock()
			break loop
		}
	}

	// Close all remaining channels
	for _, conn := range r.Conns {
		conn.OutChannel <- nil
	}
}
