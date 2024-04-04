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

func validOrNil(valid bool, val interface{}) interface{} {
	if valid {
		return val
	} else {
		return nil
	}
}

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
type GameplayPhaseStatusGameplayPlayer struct {
	Relationship [][3]int
	ActionPoints int
	Hand         []string
}
type GameplayPhaseStatusGameplay struct {
	ActCount   int
	RoundCount int
	MoveCount  int
	Player     []GameplayPhaseStatusGameplayPlayer
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

func GameplayPhaseStatusGameplayNew(n int, holder int, f func()) GameplayPhaseStatusGameplay {
	players := []GameplayPhaseStatusGameplayPlayer{}
	for i := range n {
		s := fmt.Sprintf("%d", i)
		players = append(players, GameplayPhaseStatusGameplayPlayer{
			Relationship: make([][3]int, n),
			ActionPoints: 1,
			Hand:         []string{"card" + s + "1", "card" + s + "2", "card" + s + "3"},
		})
	}
	arena := []string{"kw1", "kw2", "kw3"}

	return GameplayPhaseStatusGameplay{
		ActCount:   1,
		RoundCount: 1,
		MoveCount:  1,
		Player:     players,
		Arena:      arena,
		Holder:     holder,
		Step:       "selection",

		Timer: NewPeekableTimerFunc(30*time.Second, f),
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
func (ps GameplayPhaseStatusGameplay) Repr(playerIndex int) OrderedKeysMarshal {
	return ps.ReprWithEvent(playerIndex, "none")
}
func (ps GameplayPhaseStatusGameplay) ReprWithEvent(playerIndex int, event string) OrderedKeysMarshal {
	actionTaken := (ps.Step == "storytelling_holder" || ps.Step == "storytelling_target")
	return OrderedKeysMarshal{
		{"event", event},
		{"act_count", ps.ActCount},
		{"round_count", ps.RoundCount},
		{"move_count", ps.MoveCount},
		{"relationship", ps.Player[playerIndex].Relationship},
		{"action_points", ps.Player[playerIndex].ActionPoints},
		{"hand", ps.Player[playerIndex].Hand},
		{"arena", ps.Arena},
		{"holder", ps.Holder},
		{"step", ps.Step},
		{"action", validOrNil(actionTaken, ps.Action)},
		{"keyword", validOrNil(actionTaken, ps.Keyword)},
		{"target", validOrNil(actionTaken, ps.Target)},
		{"holder_difficulty", validOrNil(actionTaken, ps.HolderDifficulty)},
		{"holder_result", validOrNil(actionTaken, ps.HolderResult)},
		{"target_difficulty", validOrNil(actionTaken, ps.TargetDifficulty)},
		{"target_result", validOrNil(actionTaken, ps.TargetResult)},
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

	case GameplayPhaseStatusGameplay:
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
	if _, ok := s.PhaseStatus.(GameplayPhaseStatusAssembly); !ok {
		return "Not in assembly phase"
	}
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
	if _, ok := s.PhaseStatus.(GameplayPhaseStatusAssembly); !ok {
		return "Not in assembly phase"
	}
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
func (s *GameplayState) AppointmentAcceptOrPass(userId int, accept bool, roomSignalChannel chan interface{}) (int, int, bool, string) {
	st, ok := s.PhaseStatus.(GameplayPhaseStatusAppointment)
	if !ok {
		return -1, -1, false, "Not in appointment phase"
	}
	if userId != -1 && s.Players[st.Holder].User.Id != userId {
		return -1, -1, false, "Not move holder"
	}

	f := func() {
		roomSignalChannel <- GameRoomSignalTimer{Type: "gameplay"}
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
			s.PhaseStatus = GameplayPhaseStatusGameplayNew(len(s.Players), luckyDog, f)
			return st.Holder, luckyDog, true, ""
		}
	} else {
		st.Timer.Stop()
		s.PhaseStatus = GameplayPhaseStatusGameplayNew(len(s.Players), st.Holder, f)
		return -1, st.Holder, true, ""
	}
}

func (s *GameplayState) ActionCheck(userId int, handIndex int, arenaIndex int, target int) string {
	st, ok := s.PhaseStatus.(GameplayPhaseStatusGameplay)
	if !ok {
		return "Not in gameplay phase"
	}
	if userId != -1 && s.Players[st.Holder].User.Id != userId {
		return "Not move holder"
	}
	if st.Step != "selection" {
		return "Not in selection step"
	}

	playerIndex := st.Holder

	if handIndex < 0 || handIndex >= len(st.Player[playerIndex].Hand) {
		return "`hand_index` out of range"
	}
	if arenaIndex < 0 || arenaIndex >= len(st.Arena) {
		return "`arena_index` out of range"
	}
	if target < -2 || target >= len(s.Players) {
		return "`target` out of range"
	}
	if target == playerIndex {
		target = -1
	}

	st.Step = "storytelling_holder"
	st.Action = st.Player[playerIndex].Hand[handIndex]
	st.Keyword = arenaIndex
	st.Target = target
	st.HolderDifficulty = CloudRandom(100)
	// TODO
	if st.HolderDifficulty < 50 {
		st.HolderResult = 1
		if st.HolderDifficulty <= 5 {
			st.HolderResult = 2
		}
	} else {
		st.HolderResult = -1
		if st.HolderDifficulty >= 90 {
			st.HolderResult = -2
		}
	}
	st.Timer.Reset(120 * time.Second)

	// Remove card from hand
	st.Player[playerIndex].Hand = append(
		st.Player[playerIndex].Hand[:handIndex],
		st.Player[playerIndex].Hand[handIndex+1:]...,
	)
	s.PhaseStatus = st

	return ""
}

// Returns (isNewMove, error)
func (s *GameplayState) StorytellingEnd(userId int) (bool, string) {
	st, ok := s.PhaseStatus.(GameplayPhaseStatusGameplay)
	if !ok {
		return false, "Not in gameplay phase"
	}

	var storyteller int
	var nextStoryteller int
	if st.Step == "storytelling_holder" {
		storyteller = st.Holder
		nextStoryteller = st.Target // Can be -1
	} else if st.Step == "storytelling_target" {
		storyteller = st.Target
		nextStoryteller = -1
	} else {
		return false, "Not in storytelling step"
	}

	if userId != -1 && s.Players[storyteller].User.Id != userId {
		return false, "Not storyteller"
	}

	var isNewMove bool
	if nextStoryteller != -1 {
		st.Step = "storytelling_target"
		isNewMove = false
		st.Timer.Reset(120 * time.Second)
	} else {
		st.Step = "selection"
		st.MoveCount += 1
		st.Holder = (st.Holder + 1) % len(s.Players)
		// Remove keyword from arena
		st.Arena = append(
			st.Arena[:st.Keyword],
			st.Arena[st.Keyword+1:]...,
		)
		st.Timer.Reset(30 * time.Second)
	}

	s.PhaseStatus = st
	return isNewMove, ""
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
			st := r.Gameplay.PhaseStatus.(GameplayPhaseStatusGameplay)
			message = OrderedKeysMarshal{
				{"type", "appointment_accept"},
				{"prev_holder", prevVal},
				{"gameplay_status", st.Repr(r.Gameplay.PlayerIndex(userId))},
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

func (r *GameRoom) BroadcastGameProgress(event string) {
	r.Mutex.RLock()
	st := r.Gameplay.PhaseStatus.(GameplayPhaseStatusGameplay)
	for userId, conn := range r.Conns {
		conn.OutChannel <- OrderedKeysMarshal{
			{"type", "gameplay_progress"},
			{"gameplay_status", st.ReprWithEvent(r.Gameplay.PlayerIndex(userId), event)},
		}
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
			r.Gameplay.AppointmentAcceptOrPass(msg.UserId, message["type"] == "appointment_accept", r.Signal)
		if err != "" {
			panic(err)
		}
		unlock()
		r.BroadcastAppointmentUpdate(prevHolder, nextHolder, isStarting)
	} else if message["type"] == "action" {
		handIndex, ok := message["hand_index"].(float64)
		if !ok {
			panic("Incorrect `hand_index`")
		}
		arenaIndex, ok := message["arena_index"].(float64)
		if !ok {
			panic("Incorrect `arena_index`")
		}
		target, ok := message["target"].(float64)
		if !ok {
			target = -1
		}
		if err := r.Gameplay.ActionCheck(msg.UserId, int(handIndex), int(arenaIndex), int(target)); err != "" {
			panic(err)
		}
		unlock()
		r.BroadcastGameProgress("action_check")
	} else if message["type"] == "storytelling_end" {
		isNewMove, err := r.Gameplay.StorytellingEnd(msg.UserId)
		if err != "" {
			panic(err)
		}
		unlock()
		var event string
		if isNewMove {
			event = "storytelling_end_new_move"
		} else {
			event = "storytelling_end_next_storyteller"
		}
		r.BroadcastGameProgress(event)
	} else if message["type"] == "queue" {
	} else if message["type"] == "comment" {
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
						r.Gameplay.AppointmentAcceptOrPass(-1, false, r.Signal)
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
