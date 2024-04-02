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
	PlayerIndex int
	Message     map[string]interface{}
}

type GameRoomSignalNewConn struct {
	PlayerIndex int
}
type GameRoomSignalLostConn struct {
	PlayerIndex int
}
type GameRoomSignalReEstConn struct {
	PlayerIndex int
}

type GameRoom struct {
	Room
	Closed    bool
	Conns     []WebSocketConn
	InChannel chan GameRoomInMessage
	Signal    chan interface{}
	Mutex     *sync.Mutex
}

var GameRoomDataMutex = &sync.Mutex{}
var GameRoomMap = make(map[string]*GameRoom)

func GameRoomFind(roomId string) *GameRoom {
	return GameRoomMap[roomId]
}

func (r *GameRoom) Join(user User, channel chan interface{}) int {
	r.Mutex.Lock()
	playerIndex := len(r.Conns)
	r.Conns = append(r.Conns, WebSocketConn{User: user, OutChannel: channel})
	r.Mutex.Unlock()
	r.Signal <- GameRoomSignalNewConn{
		PlayerIndex: playerIndex,
	}
	return playerIndex
}

func (r *GameRoom) Lost(playerIndex int) {
	r.Mutex.Lock()
	r.Conns[playerIndex].OutChannel = nil
	closed := r.Closed
	r.Mutex.Unlock()
	if !closed {
		r.Signal <- GameRoomSignalLostConn{
			PlayerIndex: playerIndex,
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
		Conns:     []WebSocketConn{},
		InChannel: make(chan GameRoomInMessage, 4),
		Signal:    make(chan interface{}, 2),
		Mutex:     &sync.Mutex{},
	}
	GameRoomMap[room.Id] = r
	GameRoomDataMutex.Unlock()

	timeoutTicker := time.NewTicker(5 * time.Second)
	defer timeoutTicker.Stop()

	hahaTicker := time.NewTicker(1 * time.Second)
	defer hahaTicker.Stop()

loop:
	for {
		select {
		case msg := <-r.InChannel:
			conn := r.Conns[msg.PlayerIndex]
			conn.OutChannel <- OrderedKeysMarshal{
				{"message", "received"},
				{"object", msg.Message},
			}
			timeoutTicker.Reset(5 * time.Second)

		case sig := <-r.Signal:
			if sigNewConn, ok := sig.(GameRoomSignalNewConn); ok {
				conn := r.Conns[sigNewConn.PlayerIndex]
				conn.OutChannel <- OrderedKeysMarshal{
					{"message", "Hello " + conn.User.Nickname},
					{"player_index", sigNewConn.PlayerIndex},
					{"room", room.Repr()},
				}
			}
			if sigNewConn, ok := sig.(GameRoomSignalLostConn); ok {
				conn := &r.Conns[sigNewConn.PlayerIndex]
				conn.OutChannel = nil
			}
			timeoutTicker.Reset(5 * time.Second)

		// Debug
		case <-hahaTicker.C:
			for i, conn := range r.Conns {
				if conn.OutChannel != nil {
					conn.OutChannel <- OrderedKeysMarshal{
						{"message", "haha"},
						{"player_index", i},
					}
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
		if conn.OutChannel != nil {
			conn.OutChannel <- nil
		}
	}
}
