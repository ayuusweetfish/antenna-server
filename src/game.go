package main

import (
	"sync"
	"time"
)

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

type GameRoom struct {
	Room
	Closed    bool
	Conns     map[int]WebSocketConn
	InChannel chan GameRoomInMessage
	Signal    chan interface{}
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
		Mutex:     &sync.RWMutex{},
	}
	GameRoomMap[room.Id] = r
	GameRoomDataMutex.Unlock()

	timeoutDur := 30 * time.Second
	timeoutTicker := time.NewTicker(timeoutDur)
	defer timeoutTicker.Stop()

	hahaTicker := time.NewTicker(1 * time.Second)
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
			timeoutTicker.Reset(timeoutDur)

		case sig := <-r.Signal:
			if sigNewConn, ok := sig.(GameRoomSignalNewConn); ok {
				r.Mutex.RLock()
				conn := r.Conns[sigNewConn.UserId]
				r.Mutex.RUnlock()
				conn.OutChannel <- OrderedKeysMarshal{
					{"message", "Hello " + conn.User.Nickname},
					{"room", room.Repr()},
				}
			}
			if sigNewConn, ok := sig.(GameRoomSignalLostConn); ok {
				println("connection lost ", sigNewConn.UserId)
			}
			timeoutTicker.Reset(timeoutDur)

		// Debug
		case <-hahaTicker.C:
			r.Mutex.RLock()
			n := len(r.Conns)
			r.Mutex.RUnlock()
			for userId, conn := range r.Conns {
				conn.OutChannel <- OrderedKeysMarshal{
					{"message", "haha"},
					{"user_id", userId},
					{"count", n},
				}
			}

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
