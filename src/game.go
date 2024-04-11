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

////// Card set settings //////

type Card struct {
	Condition          []int
	Growth             int
	RelationshipChange [3]int
}

var CardSet map[string]Card = func() map[string]Card {
	const (
		Se = iota
		Si
		Ne
		Ni
		Te
		Ti
		Fe
		Fi
	)
	return map[string]Card{
		"吸引关注": {[]int{0, 4, 3, 2}, 1, [3]int{2, 0, 0}},
		"散发性感": {[]int{0, 7}, 1, [3]int{3, 0, 0}},
		"凝视":   {[]int{7, 0, 2}, 1, [3]int{2, 1, 0}},
		"微笑":   {[]int{6}, 1, [3]int{0, 2, 0}},
		"触碰":   {[]int{0, 1}, 1, [3]int{3, 2, 1}},
		"牵手":   {[]int{2, 0, 6}, 1, [3]int{3, 3, 1}},
		"共舞":   {[]int{0, 6, 2, 3}, 1, [3]int{5, 5, 0}},
		"拥抱":   {[]int{6, 0, 7, 3}, 1, [3]int{2, 5, 1}},
		"分享":   {[]int{6, 2, 4}, 1, [3]int{2, 3, 0}},
		"倾诉":   {[]int{6, 4, 7, 1, 2}, 1, [3]int{1, 3, 5}},
		"倾听":   {[]int{6, 5, 3, 1, 7}, 1, [3]int{0, 8, 5}},
		"同情":   {[]int{6, 7}, 1, [3]int{0, 1, 1}},
		"安慰":   {[]int{6, 7, 4, 5, 2, 3}, 1, [3]int{0, 2, 2}},
		"共情":   {[]int{6, 7, 2, 3, 1, 0}, 1, [3]int{0, 5, 0}},
		"理解":   {[]int{5, 4, 3, 2, 1, 0}, 1, [3]int{1, 2, 2}},
		"指责":   {[]int{4, 7, 6, 3, 1}, 1, [3]int{5, -2, -2}},
		"分手":   {[]int{5, 4, 2, 3, 1, 0, 7}, 1, [3]int{-9, -9, -9}},
		"共鸣":   {[]int{7, 6, 4, 5, 1, 0, 2, 3}, 1, [3]int{2, 9, 2}},
		"邀约":   {[]int{7, 2, 4, 6, 3, 0}, 1, [3]int{3, 2, 1}},
		"赠礼":   {[]int{6, 4, 2, 3}, 1, [3]int{1, 1, 2}},
		"投食":   {[]int{6, 1}, 1, [3]int{2, 5, 1}},
		"照料":   {[]int{6, 0, 1}, 1, [3]int{-1, 3, 9}},
		"告白":   {[]int{7, 6, 4, 2}, 1, [3]int{5, 5, 3}},
		"亲吻":   {[]int{0, 1, 7, 6}, 1, [3]int{5, 8, 0}},
		"性爱":   {[]int{0, 7}, 2, [3]int{9, 8, 0}},
		"约定终身": {[]int{7, 1, 6, 4, 3}, 3, [3]int{3, 5, 9}},
		"刺杀":   {[]int{0, 3, 4}, 3, [3]int{5, -9, -9}},
		"做饭":   {[]int{0, 1, 7, 6, 2}, 1, [3]int{1, 3, 3}},
		"吃喝":   {[]int{0, 1, 7, 6, 2}, 1, [3]int{2, 3, 3}},
		"睡觉":   {[]int{0, 1}, 1, [3]int{1, 1, 1}},
		"上厕所":  {[]int{0, 1}, 1, [3]int{1, 0, 3}},
		"外出":   {[]int{0, 2}, 1, [3]int{0, 3, 1}},
		"窥视":   {[]int{2, 7}, 2, [3]int{5, 5, -9}},
		"违法":   {[]int{2, 5, 3, 7}, 3, [3]int{5, -2, -9}},
		"努力工作": {[]int{4, 3, 7, 6}, 1, [3]int{2, 1, 9}},
		"开枪":   {[]int{0, 3}, 3, [3]int{5, -8, 1}},
		"自残":   {[]int{7, 6, 5}, 3, [3]int{3, -3, -9}},
		"治疗":   {[]int{6, 1, 5, 2, 3, 4}, 2, [3]int{1, 1, 8}},
		"求医":   {[]int{1, 6, 3, 0}, 2, [3]int{0, 3, 3}},
		"作弊":   {[]int{0, 5, 4}, 1, [3]int{3, -3, -8}},
		"背叛":   {[]int{0, 3, 5, 2}, 2, [3]int{-2, -5, -5}},
		"恐吓":   {[]int{1, 4, 3, 0}, 2, [3]int{3, -5, -8}},
		"交易":   {[]int{5, 4, 2, 3}, 1, [3]int{1, 1, 2}},
		"出老千":  {[]int{0, 3, 2, 5}, 1, [3]int{-1, -2, -5}},
		"忍气吞声": {[]int{1, 6, 4, 3}, 2, [3]int{3, -1, -1}},
		"冥想":   {[]int{1, 3, 2, 0}, 1, [3]int{2, 2, 1}},
		"朝拜":   {[]int{3, 2, 7, 6}, 1, [3]int{3, 3, -1}},
		"竞争":   {[]int{4, 3, 1, 0, 7}, 2, [3]int{3, -5, 0}},
		"合作":   {[]int{6, 1, 3}, 1, [3]int{3, 3, 5}},
		"学习":   {[]int{5, 4, 1, 2}, 1, [3]int{3, -1, 5}},
		"工作":   {[]int{1, 4}, 1, [3]int{3, -1, 8}},
		"放弃":   {[]int{7, 2, 5, 6, 1}, 2, [3]int{1, -3, -3}},
		"狡辩":   {[]int{5, 0, 2}, 2, [3]int{-2, -2, -5}},
		"怀疑":   {[]int{1, 3, 5, 4}, 2, [3]int{1, -3, 1}},
		"自我质疑": {[]int{1, 7, 6, 4, 3}, 1, [3]int{3, 5, -1}},
		"签订契约": {[]int{1, 4}, 1, [3]int{3, 2, 9}},
		"许诺":   {[]int{1, 4, 6}, 2, [3]int{5, 5, 9}},
		"跑路":   {[]int{3, 5, 2, 0}, 2, [3]int{3, 3, -5}},
		"购买":   {[]int{7, 0, 4, 2}, 1, [3]int{1, 1, 1}},
		"思考":   {[]int{5}, 1, [3]int{5, 0, 2}},
		"分析":   {[]int{5, 4, 2, 3}, 2, [3]int{2, 0, 3}},
		"创造":   {[]int{2, 7, 5, 4}, 2, [3]int{8, 1, 2}},
		"做白日梦": {[]int{7, 2}, 1, [3]int{3, 3, -2}},
		"运动":   {[]int{0, 1}, 1, [3]int{8, 1, 0}},
		"狂奔":   {[]int{0}, 1, [3]int{8, 2, 0}},
		"逃跑":   {[]int{0, 4, 3}, 2, [3]int{2, -1, 0}},
		"沉思":   {[]int{3, 2, 5, 7}, 3, [3]int{5, 2, 3}},
		"理性思考": {[]int{5, 4}, 2, [3]int{3, 1, 2}},
		"逻辑思考": {[]int{5, 2}, 2, [3]int{3, 0, 2}},
		"证明":   {[]int{4, 1, 5, 2}, 2, [3]int{3, 1, 3}},
		"同化":   {[]int{6, 4, 5, 3}, 1, [3]int{3, 1, -1}},
		"排挤":   {[]int{7, 6, 1, 3}, 2, [3]int{3, -8, -2}},
		"吹捧":   {[]int{1, 6, 4}, 1, [3]int{5, 5, -8}},
		"奉承":   {[]int{1, 6, 4}, 1, [3]int{5, 8, -9}},
		"批判":   {[]int{4, 5, 3, 7, 1}, 2, [3]int{3, -5, 3}},
		"启发":   {[]int{4, 5, 6, 3}, 2, [3]int{1, 1, 3}},
		"信仰":   {[]int{3, 7, 6}, 2, [3]int{3, 1, 9}},
		"苦中作乐": {[]int{2, 5, 7, 0}, 2, [3]int{5, -1, 0}},
		"顿悟":   {[]int{3}, 3, [3]int{3, -1, 2}},
		"迷思":   {[]int{5, 2}, 2, [3]int{1, 1, 1}},
		"偷懒":   {[]int{0, 5, 2}, 1, [3]int{1, 0, -5}},
		"不懂装懂": {[]int{1, 4, 6}, 1, [3]int{0, 0, -3}},
		"变性":   {[]int{}, 3, [3]int{3, 0, 9}},
		"内在探索": {[]int{3, 5, 7, 1}, 2, [3]int{1, 1, 3}},
		"追求平等": {[]int{7, 6, 4, 5, 3}, 2, [3]int{1, 0, 9}},
		"出柜":   {[]int{7, 4, 2, 3}, 3, [3]int{3, 5, 9}},
		"解放天性": {[]int{2, 7, 0, 1}, 3, [3]int{5, 3, 0}},
		"自闭":   {[]int{}, 2, [3]int{-3, -3, 0}},
		"蛊惑":   {[]int{0, 5, 3, 2}, 1, [3]int{9, 8, -2}},
		"放手":   {[]int{6, 5, 2}, 3, [3]int{2, -5, -1}},
		"坚持":   {[]int{1, 4, 3}, 1, [3]int{1, 2, 2}},
		"承认":   {[]int{4, 3, 5}, 1, [3]int{1, 2, 5}},
		"献祭":   {[]int{3, 1, 6, 4}, 1, [3]int{3, 1, 9}},
		"祭祀":   {[]int{1, 3, 6, 4}, 1, [3]int{1, 4, 5}},
		"跳舞":   {[]int{0, 6, 3, 1}, 1, [3]int{5, 4, 1}},
		"强化":   {[]int{1, 7}, 1, [3]int{3, 0, 3}},
		"观察":   {[]int{1, 0}, 1, [3]int{1, 1, 1}},
		"祈祷":   {[]int{3, 7}, 1, [3]int{0, 3, 2}},
		"咆哮":   {[]int{7, 4, 1}, 1, [3]int{2, -8, -3}},
		"表达情绪": {[]int{7, 1, 4}, 1, [3]int{1, 5, 2}},
		"保护":   {[]int{0, 1, 7}, 1, [3]int{1, 5, 8}},
		"养育":   {[]int{6, 7, 3}, 3, [3]int{5, 9, 9}},
		"读":    {[]int{1, 5, 7, 3}, 2, [3]int{0, 0, 1}},
		"涂鸦":   {[]int{7, 0, 2}, 1, [3]int{3, 2, 0}},
		"蹦跳":   {[]int{0}, 1, [3]int{2, 1, 0}},
		"绘画":   {[]int{0, 7, 2, 3, 4}, 2, [3]int{5, 5, 0}},
		"作曲":   {[]int{1, 7, 0, 3, 5}, 2, [3]int{5, 5, 0}},
		"漫步":   {[]int{0, 3}, 1, [3]int{0, 3, 0}},
		"盯":    {[]int{7, 0, 3}, 1, [3]int{1, 0, 0}},
		"翻":    {[]int{0, 2}, 1, [3]int{0, 0, 0}},
		"整理":   {[]int{1, 4, 6}, 1, [3]int{0, 0, 2}},
		"穿越":   {[]int{0, 3}, 2, [3]int{0, 0, 0}},
		"洞穿":   {[]int{3, 5, 6}, 2, [3]int{3, 3, 0}},
		"洗":    {[]int{0, 1, 4}, 1, [3]int{0, 1, 1}},
		"系":    {[]int{0, 1, 2, 5, 4}, 1, [3]int{1, 2, 2}},
		"折叠":   {[]int{1, 4}, 1, [3]int{1, 1, 1}},
		"打磨":   {[]int{1, 0, 4, 3}, 2, [3]int{2, 1, 5}},
		"接通":   {[]int{1, 6, 7, 2}, 1, [3]int{1, 6, 1}},
		"联系":   {[]int{2, 7, 0}, 1, [3]int{1, 6, 1}},
		"鼓励":   {[]int{6, 7, 1}, 1, [3]int{2, 6, 3}},
		"打击":   {[]int{4, 1, 7}, 1, [3]int{-5, -9, 2}},
		"点燃":   {[]int{0, 2, 3, 7, 4}, 2, [3]int{8, 5, 3}},
		"欺骗":   {[]int{0, 6, 3, 4}, 2, [3]int{-9, -9, -9}},
		"幻想":   {[]int{2, 7, 1}, 1, [3]int{9, 7, 0}},
		"联想":   {[]int{2}, 1, [3]int{0, 1, 0}},
		"回忆":   {[]int{1}, 1, [3]int{0, 5, 1}},
		"挖掘":   {[]int{3, 4, 5, 2, 0}, 2, [3]int{3, 3, 3}},
		"破坏":   {[]int{0, 4, 3, 1, 7}, 1, [3]int{6, 2, 0}},
		"建构":   {[]int{5, 2, 4, 1}, 3, [3]int{0, 0, 0}},
		"教导":   {[]int{4, 2, 5, 1}, 2, [3]int{5, 2, 9}},
		"感染":   {[]int{6, 4, 0, 7}, 1, [3]int{5, 2, 0}},
	}
}()
var CardSetNames []string = func() []string {
	names := []string{}
	for key := range CardSet {
		names = append(names, key)
	}
	return names
}()

var KeywordSetNames []string = []string{"kwA", "kwB", "kwC", "kwD", "kwE", "kwF", "kwG"}

////// Gameplay //////

const TimeLimitAppointment = 30 * time.Second
const TimeLimitCardSelection = 60 * time.Second
const TimeLimitStorytelling = 180 * time.Second
const TimeLimitStorytellingCont = 120 * time.Second

type GameplayPhaseStatusAssembly struct {
}
type GameplayPhaseStatusAppointment struct {
	Holder int
	Count  int
	Timer  PeekableTimer
}
type GameplayPhaseStatusGameplayPlayer struct {
	Relationship [][3]float32
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

func max[T interface{ int | uint64 | float64 }](x, y T) T {
	if x > y {
		return x
	} else {
		return y
	}
}
func fillRandomElements(elements []string, n int, pool []string) []string {
	if elements == nil {
		elements = []string{}
	}
	set := map[string]struct{}{}
	for _, w := range elements {
		set[w] = struct{}{}
	}
	for len(elements) < n {
		w := pool[CloudRandom(len(pool))]
		if _, ok := set[w]; !ok {
			elements = append(elements, w)
			set[w] = struct{}{}
		}
	}
	return elements
}
func fillArena(arena []string, n int) []string {
	return fillRandomElements(arena, n, KeywordSetNames)
}
func fillCards(cards []string, n int) []string {
	return fillRandomElements(cards, n, CardSetNames)
}

func GameplayPhaseStatusGameplayNew(n int, holder int, f func()) GameplayPhaseStatusGameplay {
	players := []GameplayPhaseStatusGameplayPlayer{}
	for _ = range n {
		players = append(players, GameplayPhaseStatusGameplayPlayer{
			Relationship: make([][3]float32, n),
			ActionPoints: 1,
			Hand:         fillCards(nil, 5),
		})
	}

	return GameplayPhaseStatusGameplay{
		ActCount:   1,
		RoundCount: 1,
		MoveCount:  1,
		Player:     players,
		Arena:      fillArena(nil, max(n, 3)),
		Holder:     holder,
		Step:       "selection",

		Timer: NewPeekableTimerFunc(TimeLimitCardSelection, f),
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
		{"target", validOrNil(actionTaken && ps.Target != -1, ps.Target)},
		{"holder_difficulty", validOrNil(actionTaken, ps.HolderDifficulty)},
		{"holder_result", validOrNil(actionTaken, ps.HolderResult)},
		{"target_difficulty", validOrNil(actionTaken && ps.Target != -1, ps.TargetDifficulty)},
		{"target_result", validOrNil(actionTaken && ps.Target != -1, ps.TargetResult)},
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
		Timer: NewPeekableTimerFunc(TimeLimitAppointment, func() {
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
			st.Timer.Reset(TimeLimitAppointment)
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

	if userId == -1 {
		handIndex = CloudRandom(len(st.Player[playerIndex].Hand))
		arenaIndex = CloudRandom(len(st.Arena))
	}

	if handIndex < 0 || handIndex >= len(st.Player[playerIndex].Hand) {
		return "`hand_index` out of range"
	}
	if arenaIndex < 0 || arenaIndex >= len(st.Arena) {
		return "`arena_index` out of range"
	}
	if target < -1 || target >= len(s.Players) {
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

	// Check
	card := CardSet[st.Action]
	checkResult := func(difficulty int, stats [8]int) int {
		if difficulty <= 5 {
			return 2
		} else if difficulty >= 90 {
			return -2
		}
		// Compare card requirements to stats
		count := 0
		for _, statIndex := range card.Condition {
			if stats[statIndex] >= difficulty {
				count++
			}
		}
		if count*2 >= len(card.Condition) {
			return 1
		} else {
			return -1
		}
	}
	applyRelationshipChanges := func(result int, relationship *[3]float32) {
		var multiplier float32
		switch result {
		case 2:
			multiplier = 1.5
		case 1:
			multiplier = 1.0
		case -1:
			multiplier = -1.0
		case -2:
			multiplier = -1.5
		}
		for i := range 3 {
			relationship[i] += float32(card.RelationshipChange[i]) * multiplier
		}
	}

	st.HolderResult = checkResult(st.HolderDifficulty, s.Players[playerIndex].Profile.Stats)
	// XXX: Do relationship values change when acting without a target?
	if target != -1 {
		applyRelationshipChanges(st.HolderResult, &st.Player[playerIndex].Relationship[target])
	}

	if target != -1 {
		difficulty := CloudRandom(100)
		st.TargetDifficulty = difficulty
		switch st.HolderResult {
		case 2:
			difficulty -= 20
		case 1:
			difficulty -= 10
		case -1:
			difficulty += 10
		case -2:
			difficulty += 20
		}
		st.TargetResult = checkResult(difficulty, s.Players[target].Profile.Stats)
		applyRelationshipChanges(st.TargetResult, &st.Player[target].Relationship[playerIndex])
	} else {
		st.TargetDifficulty = -1
		st.TargetResult = 0
	}

	st.Timer.Reset(TimeLimitStorytelling)

	// Remove card from hand
	st.Player[playerIndex].Hand = append(
		st.Player[playerIndex].Hand[:handIndex],
		st.Player[playerIndex].Hand[handIndex+1:]...,
	)
	// Deduct action points
	st.Player[playerIndex].ActionPoints -= 1

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
		st.Timer.Reset(TimeLimitStorytellingCont)
	} else {
		st.Step = "selection"
		isNewMove = true
		st.MoveCount += 1
		// Remove keyword from arena
		st.Arena = append(
			st.Arena[:st.Keyword],
			st.Arena[st.Keyword+1:]...,
		)
		// Replenish hand
		st.Player[st.Holder].Hand = fillCards(st.Player[st.Holder].Hand, 5)
		// Next player
		if len(st.Queue) > 0 {
			st.Holder = st.Queue[0]
			st.Queue = st.Queue[1:]
		} else {
			// Randomly pick a player with non-zero action point(s)
			nonZero := []int{}
			for i, p := range st.Player {
				if p.ActionPoints > 0 {
					nonZero = append(nonZero, i)
				}
			}
			if len(nonZero) > 0 {
				st.Holder = nonZero[CloudRandom(len(nonZero))]
			} else {
				// New round!
				st.RoundCount += 1
				st.MoveCount = 1
				// Replenish arena
				st.Arena = fillArena(st.Arena, max(len(s.Players), 3))
				// Replenish action points
				for i, _ := range st.Player {
					st.Player[i].ActionPoints = 1
				}
				// Random player
				st.Holder = CloudRandom(len(s.Players))
			}
		}
		st.Timer.Reset(TimeLimitCardSelection)
	}

	s.PhaseStatus = st
	return isNewMove, ""
}

func (s *GameplayState) Queue(userId int) string {
	st, ok := s.PhaseStatus.(GameplayPhaseStatusGameplay)
	if !ok {
		return "Not in gameplay phase"
	}

	playerIndex := s.PlayerIndex(userId)
	if playerIndex == -1 {
		return "Not in game"
	}
	if st.Player[playerIndex].ActionPoints == 0 {
		return "No action points remaining"
	}
	for i, p := range st.Queue {
		if p == playerIndex {
			return fmt.Sprintf("Already in queue (position %d)", i)
		}
	}

	st.Queue = append(st.Queue, playerIndex)
	s.PhaseStatus = st

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

// All broadcast subroutines assume the mutex is held (RLock'ed)

func (r *GameRoom) BroadcastStart() {
	for userId, conn := range r.Conns {
		conn.OutChannel <- OrderedKeysMarshal{
			{"type", "start"},
			{"holder", r.Gameplay.PhaseStatus.(GameplayPhaseStatusAppointment).Holder},
			{"my_index", r.Gameplay.PlayerIndexNullable(userId)},
		}
	}
}

func (r *GameRoom) BroadcastRoomState() {
	for userId, conn := range r.Conns {
		conn.OutChannel <- r.StateMessage(userId)
	}
}

func (r *GameRoom) BroadcastAssemblyUpdate() {
	message := OrderedKeysMarshal{
		{"type", "assembly_update"},
		{"players", r.Gameplay.PlayerReprs()},
	}
	for _, conn := range r.Conns {
		conn.OutChannel <- message
	}
}

func (r *GameRoom) BroadcastAppointmentUpdate(prevHolder int, nextHolder int, isStarting bool) {
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
}

func (r *GameRoom) BroadcastGameProgress(event string) {
	st := r.Gameplay.PhaseStatus.(GameplayPhaseStatusGameplay)
	for userId, conn := range r.Conns {
		conn.OutChannel <- OrderedKeysMarshal{
			{"type", "gameplay_progress"},
			{"gameplay_status", st.ReprWithEvent(r.Gameplay.PlayerIndex(userId), event)},
		}
	}
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
		r.BroadcastAssemblyUpdate()
	} else if message["type"] == "withdraw" {
		if err := r.Gameplay.WithdrawSeat(msg.UserId); err != "" {
			panic(err)
		}
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
		r.BroadcastStart()
	} else if message["type"] == "appointment_accept" || message["type"] == "appointment_pass" {
		prevHolder, nextHolder, isStarting, err :=
			r.Gameplay.AppointmentAcceptOrPass(msg.UserId, message["type"] == "appointment_accept", r.Signal)
		if err != "" {
			panic(err)
		}
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
		r.BroadcastGameProgress("action_check")
	} else if message["type"] == "storytelling_end" {
		isNewMove, err := r.Gameplay.StorytellingEnd(msg.UserId)
		if err != "" {
			panic(err)
		}
		var event string
		if isNewMove {
			event = "storytelling_end_new_move"
		} else {
			event = "storytelling_end_next_storyteller"
		}
		r.BroadcastGameProgress(event)
	} else if message["type"] == "queue" {
		err := r.Gameplay.Queue(msg.UserId)
		if err != "" {
			panic(err)
		}
		r.BroadcastGameProgress("queue")
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
					if err == "" {
						r.BroadcastAppointmentUpdate(prevHolder, nextHolder, isStarting)
					}
					r.Mutex.Unlock()

				case "gameplay":
					r.Mutex.Lock()
					if st, ok := r.Gameplay.PhaseStatus.(GameplayPhaseStatusGameplay); ok {
						if st.Step == "selection" {
							// Select random card
							r.Gameplay.ActionCheck(-1, -1, -1, -1)
							r.BroadcastGameProgress("action_check")
						} else if st.Step == "storytelling_holder" || st.Step == "storytelling_target" {
							// Stop storytelling
							isNewMove, _ := r.Gameplay.StorytellingEnd(-1)
							var event string
							if isNewMove {
								event = "storytelling_end_new_move"
							} else {
								event = "storytelling_end_next_storyteller"
							}
							r.BroadcastGameProgress(event)
						}
					}
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
