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
		{"type", "room_state"},
		{"user", user.Repr()}, // Debug usage
		{"room", r.Room.Repr()},
	}
	entries = append(entries, r.Gameplay.Repr(userId)...)
	return entries
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

	timeoutDur := 180 * time.Second
	timeoutTicker := time.NewTicker(timeoutDur)
	defer timeoutTicker.Stop()

	hahaTicker := time.NewTicker(10 * time.Second)
	defer hahaTicker.Stop()

loop:
	for {
	selection:
		select {
		case msg := <-r.InChannel:
			message := msg.Message
			if message["type"] == "seat" {
				r.Mutex.Lock()
				conn := r.Conns[msg.UserId]
				user := conn.User
				profileId, ok := message["profile_id"].(float64)
				if !ok {
					r.Mutex.Unlock()
					conn.OutChannel <- OrderedKeysMarshal{{"error", "Incorrect `profile_id`"}}
					break selection
				}
				profile := Profile{Id: int(profileId)}
				if !profile.Load() {
					r.Mutex.Unlock()
					conn.OutChannel <- OrderedKeysMarshal{{"error", "No such profile"}}
					break selection
				}
				if profile.Creator != user.Id {
					r.Mutex.Unlock()
					conn.OutChannel <- OrderedKeysMarshal{{"error", "Not creator"}}
					break selection
				}
				if err := r.Gameplay.Seat(user, profile); err != "" {
					r.Mutex.Unlock()
					conn.OutChannel <- OrderedKeysMarshal{{"error", err}}
					break selection
				}
				r.Mutex.Unlock()
				r.BroadcastAssemblyUpdate()
			} else if message["type"] == "withdraw" {
				r.Mutex.Lock()
				conn := r.Conns[msg.UserId]
				if err := r.Gameplay.WithdrawSeat(msg.UserId); err != "" {
					r.Mutex.Unlock()
					conn.OutChannel <- OrderedKeysMarshal{{"error", err}}
					break selection
				}
				r.Mutex.Unlock()
				r.BroadcastAssemblyUpdate()
			} else {
				r.Mutex.RLock()
				conn := r.Conns[msg.UserId]
				r.Mutex.RUnlock()
				conn.OutChannel <- OrderedKeysMarshal{{"error", "Unknown type"}}
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
