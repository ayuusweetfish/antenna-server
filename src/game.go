package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
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

var KeywordSetNames []string = []string{
	"Crush！", "一起吃饭", "吃饭", "酒逢知己千杯少", "同宿", "意外惊喜", "惊吓", "亲友的赞美", "吊桥效应", "灰头土脸", "偶遇", "撞击", "攻击", "打击", "音乐", "打架", "运动", "异地", "重逢", "游戏", "影剧", "书", "手机", "学习", "教学", "艺术", "游行", "舞会", "嘉年华", "出游", "散步", "坠落", "攀登", "相谈", "搭乘", "天空", "雨", "雪", "阴天", "晴天", "狂风", "台风", "妖风", "山脉", "巨石", "草原", "高原", "平原", "台地", "森林", "沙漠", "枕头", "水果", "蛇", "太阳", "月亮", "星星", "水流", "抚养小孩", "共同事业", "车", "马", "牛", "蛙", "羊", "鸡", "兔", "龙", "鼠", "虎", "狗", "猪", "拥挤", "同居", "逛街", "香水店", "杂货铺", "饭店", "药店", "购房", "购物", "河流", "海滩", "交通工具", "密室共处", "争吵", "视觉", "触觉", "嗅觉", "味觉", "听觉", "尴尬", "错过", "话不投机半句多", "惊吓", "欺骗", "逃避", "别离", "第三者", "言语暴力", "肢体暴力", "疾病", "死亡", "失忆", "失业", "晴朗", "高山", "神秘仪式", "占卜", "火锅", "魔鬼料理", "蝴蝶", "感冒", "厨房", "椅子", "雨季", "雾霾", "寒冷", "沙漠", "竹林", "蜜蜂", "鳄鱼", "头痛", "客厅", "窗帘", "雷雨", "炎热", "峡谷", "清新", "松树", "蚂蚁", "壁虎", "发烧", "车站", "书本", "凉爽", "闪光", "菊花", "池塘", "咳嗽", "电视", "大雾", "潮湿", "山峰", "纯洁", "桃", "莲", "星空", "雪山", "暖阳", "茉莉", "蝉鸣", "龙虾", "头晕", "饭馆", "灯笼", "多云", "寒露", "河流", "光辉", "玫瑰", "蚊子", "龟壳", "胃痛", "教室", "笔", "暴风雨", "温暖", "海洋", "优雅", "荷叶", "蜘蛛", "羽毛", "疲劳", "图书馆", "笔记本", "雪花", "凉风", "湖泊", "灵动", "菠萝", "蜥蜴", "骨折", "市场", "沙发", "晚霞", "热浪", "岛屿", "宁静", "葡萄", "蚂蟥", "扭伤", "长城", "星光", "冰霜", "暖炉", "芙蓉", "蝗虫", "鲫鱼", "失眠", "客房", "钟楼", "啤酒", "雾气", "露水", "河岸", "光环", "风车", "鹰", "龟", "心跳", "会议室", "电子书", "僧侣", "雷声", "辣", "海浪", "优秀", "沼泽", "蜈蚣", "蜥蜴", "疾病", "商店", "冰雹", "微风", "海湾", "泼水", "橙子", "蛇", "拉肚子", "饮料", "通信", "深渊", "晚霞", "热带", "岛屿", "安详", "柠檬", "蜗牛", "扭腰", "珠宝", "星系", "霜降", "暖气", "花瓣", "油炸", "海产", "眼花", "客栈", "钟声", "春天", "夏天", "秋天", "冬天", "晴空", "露珠", "流水", "光线", "花卉", "苍蝇", "枯萎", "心痛", "书架", "笔记", "紫外线", "闪电", "升温", "海岸", "优雅", "荷塘", "蜈蚣", "蟑螂", "疾病", "商店", "电脑", "青岛", "冰雹", "微风", "海湾", "活泼", "橙子", "蛇", "拉肚子", "猫咖", "书店", "旅馆", "花园", "图书馆", "餐厅", "游乐园", "转轮", "花房", "温室", "收藏品店", "历史博物馆", "动物园", "公园", "学校", "健身房", "车站", "时间隧道", "水族馆", "剑道馆", "天文馆", "糖果店", "古堡", "跳蚤市集", "古典音乐厅", "咖啡店", "工坊", "屋顶", "阳台", "比赛", "烧烤区", "溜冰场", "星空露台", "药房", "古董店", "剧场", "画廊", "市集", "秘密地点", "彩票站", "宠物店", "迷宫花园", "夜市美食街", "古着服装店", "剑道馆", "虫洞", "秘密仪式", "齿轮", "鬼打墙",
}

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

// The `GameRoom` reference is for additionally adding unseated players in assembly phase
func (s GameplayState) PlayerReprs(r *GameRoom) []OrderedKeysMarshal {
	playerReprs := []OrderedKeysMarshal{}
	for _, p := range s.Players {
		playerReprs = append(playerReprs, p.Profile.Repr())
	}

	// Unseated players in assembly phase
	if _, ok := s.PhaseStatus.(GameplayPhaseStatusAssembly); ok {
		for userId, conn := range r.Conns {
			seated := false
			for _, p := range s.Players {
				if p.User.Id == userId {
					seated = true
					break
				}
			}
			if !seated {
				playerReprs = append(playerReprs, OrderedKeysMarshal{
					{"id", nil},
					{"creator", conn.User.Repr()},
				})
			}
		}
	}

	return playerReprs
}
func (s GameplayState) Repr(r *GameRoom, userId int) OrderedKeysMarshal {
	// Players
	playerReprs := s.PlayerReprs(r)
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

func (s *GameplayState) Seat(user User, profile Profile) (string, string) {
	if _, ok := s.PhaseStatus.(GameplayPhaseStatusAssembly); !ok {
		return "Not in assembly phase", ""
	}
	// Check duplicate
	for _, p := range s.Players {
		if p.User.Id == user.Id {
			return "Already seated", ""
		}
	}
	s.Players = append(s.Players, GameplayPlayer{User: user, Profile: profile})
	logContent := fmt.Sprintf("玩家【%s】坐下", user.Nickname)
	return "", logContent
}

func (s *GameplayState) WithdrawSeat(userId int) (string, string) {
	if _, ok := s.PhaseStatus.(GameplayPhaseStatusAssembly); !ok {
		return "Not in assembly phase", ""
	}
	for i, p := range s.Players {
		if p.User.Id == userId {
			s.Players = append(s.Players[:i], s.Players[i+1:]...)
			logContent := fmt.Sprintf("玩家【%s】离座", p.User.Nickname)
			return "", logContent
		}
	}
	return "Not seated", ""
}

// (error, log content)
func (s *GameplayState) Start(roomSignalChannel chan interface{}) (string, string) {
	if _, ok := s.PhaseStatus.(GameplayPhaseStatusAssembly); !ok {
		return "Not in assembly phase", ""
	}
	st := GameplayPhaseStatusAppointment{
		Holder: CloudRandom(len(s.Players)),
		Count:  0,
		Timer: NewPeekableTimerFunc(TimeLimitAppointment, func() {
			roomSignalChannel <- GameRoomSignalTimer{Type: "appointment"}
		}),
	}
	s.PhaseStatus = st

	logContent := fmt.Sprintf(
		"座位 %d 玩家【%s】收到起始玩家指派，等待选择",
		st.Holder+1, s.Players[st.Holder].User.Nickname,
	)
	return "", logContent
}

func (s *GameplayState) Reset() {
	s.Players = []GameplayPlayer{}
	s.PhaseStatus = GameplayPhaseStatusAssembly{}
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
// - the log content
func (s *GameplayState) AppointmentAcceptOrPass(userId int, accept bool, roomSignalChannel chan interface{}) (int, int, bool, string, string) {
	st, ok := s.PhaseStatus.(GameplayPhaseStatusAppointment)
	if !ok {
		return -1, -1, false, "Not in appointment phase", ""
	}
	if userId != -1 && s.Players[st.Holder].User.Id != userId {
		return -1, -1, false, "Not move holder", ""
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
			logContent := fmt.Sprintf(
				"座位 %d 玩家【%s】跳过指派，轮到座位 %d 玩家【%s】",
				prev+1, s.Players[prev].User.Nickname,
				st.Holder+1, s.Players[st.Holder].User.Nickname,
			)
			return prev, st.Holder, false, "", logContent
		} else {
			// Random appointment
			st.Timer.Stop()
			luckyDog := CloudRandom(len(s.Players))
			s.PhaseStatus = GameplayPhaseStatusGameplayNew(len(s.Players), luckyDog, f)
			logContent := fmt.Sprintf(
				"座位 %d 玩家【%s】跳过指派。随机抽取座位 %d 玩家【%s】开始游戏",
				st.Holder+1, s.Players[st.Holder].User.Nickname,
				luckyDog+1, s.Players[luckyDog].User.Nickname,
			)
			return st.Holder, luckyDog, true, "", logContent
		}
	} else {
		st.Timer.Stop()
		s.PhaseStatus = GameplayPhaseStatusGameplayNew(len(s.Players), st.Holder, f)
		logContent := fmt.Sprintf(
			"座位 %d 玩家【%s】接受指派，作为起始玩家开始游戏",
			st.Holder+1, s.Players[st.Holder].User.Nickname,
		)
		return -1, st.Holder, true, "", logContent
	}
}

// (error message, log content)
func (s *GameplayState) ActionCheck(userId int, handIndex int, arenaIndex int, target int) (string, string) {
	st, ok := s.PhaseStatus.(GameplayPhaseStatusGameplay)
	if !ok {
		return "Not in gameplay phase", ""
	}
	if userId != -1 && s.Players[st.Holder].User.Id != userId {
		return "Not move holder", ""
	}
	if st.Step != "selection" {
		return "Not in selection step", ""
	}

	playerIndex := st.Holder

	if userId == -1 {
		userId = s.Players[st.Holder].User.Id
		handIndex = CloudRandom(len(st.Player[playerIndex].Hand))
		arenaIndex = CloudRandom(len(st.Arena))
	}

	if handIndex < 0 || handIndex >= len(st.Player[playerIndex].Hand) {
		return "`hand_index` out of range", ""
	}
	if arenaIndex < 0 || arenaIndex >= len(st.Arena) {
		return "`arena_index` out of range", ""
	}
	if target < -1 || target >= len(s.Players) {
		return "`target` out of range", ""
	}
	if target == playerIndex {
		target = -1
	}

	st.Step = "storytelling_holder"
	st.Action = st.Player[playerIndex].Hand[handIndex]
	st.Keyword = arenaIndex
	st.Target = target
	st.HolderDifficulty = CloudRandom(100)

	keyword := st.Arena[st.Keyword]

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

	// Game log
	resultString := func(result int) string {
		if result == 2 {
			return "大成功"
		} else if result == 1 {
			return "成功"
		} else if result == -1 {
			return "失败"
		} else if result == -2 {
			return "大失败"
		}
		return "……？"
	}
	logContent := ""
	if target == -1 {
		logContent = fmt.Sprintf(
			"座位 %d 玩家【%s】使用手牌【%s】与关键词【%s】\n难度判定为 %d，结果为【%s】\n轮到玩家【%s】讲述",
			playerIndex+1, s.Players[playerIndex].User.Nickname,
			st.Action, keyword,
			st.HolderDifficulty, resultString(st.HolderResult),
			s.Players[playerIndex].User.Nickname,
		)
	} else {
		logContent = fmt.Sprintf(
			"座位 %d 玩家【%s】对座位 %d 玩家【%s】使用手牌【%s】与关键词【%s】\n主动方难度判定为 %d，结果为【%s】\n被动方难度判定为 %d，结果为【%s】\n轮到玩家【%s】讲述",
			playerIndex+1, s.Players[playerIndex].User.Nickname,
			target+1, s.Players[target].User.Nickname,
			st.Action, keyword,
			st.HolderDifficulty, resultString(st.HolderResult),
			st.TargetDifficulty, resultString(st.TargetResult),
			s.Players[playerIndex].User.Nickname,
		)
	}
	return "", logContent
}

// Returns (isNewMove, isGameEnd, error, log content)
func (s *GameplayState) StorytellingEnd(userId int) (bool, bool, string, string) {
	st, ok := s.PhaseStatus.(GameplayPhaseStatusGameplay)
	if !ok {
		return false, false, "Not in gameplay phase", ""
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
		return false, false, "Not in storytelling step", ""
	}

	if userId != -1 && s.Players[storyteller].User.Id != userId {
		return false, false, "Not storyteller", ""
	}

	isNewMove := false
	isGameEnd := false
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
				actRounds := []int{1, 2, 1, 1}
				if st.RoundCount > actRounds[st.ActCount-1] {
					st.ActCount += 1
					st.RoundCount = 1
					if st.ActCount > 4 {
						// Game end!
						isGameEnd = true
					}
				}
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

	logContent := ""
	if isGameEnd {
		logContent = "游戏结束！"
	} else if nextStoryteller == -1 { // isNewMove == true
		newProgressStr := "进入下一回合，"
		if st.MoveCount == 1 {
			newProgressStr = fmt.Sprintf("【第 %d 幕，第 %d 轮】\n", st.ActCount, st.RoundCount)
		}
		logContent = fmt.Sprintf(
			"座位 %d 玩家【%s】完成讲述\n%s由座位 %d 玩家【%s】选择手牌",
			storyteller+1, s.Players[storyteller].User.Nickname,
			newProgressStr,
			st.Holder+1, s.Players[st.Holder].User.Nickname,
		)
	} else {
		logContent = fmt.Sprintf(
			"座位 %d 玩家【%s】完成讲述\n轮到座位 %d 玩家【%s】继续讲述",
			storyteller+1, s.Players[storyteller].User.Nickname,
			nextStoryteller+1, s.Players[nextStoryteller].User.Nickname,
		)
	}
	return isNewMove, isGameEnd, "", logContent
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

type GameRoomLog struct {
	Id        int
	Timestamp int64
	Content   string
}

type GameRoom struct {
	Room
	Closed    bool
	Conns     map[int]WebSocketConn
	InChannel chan GameRoomInMessage
	Signal    chan interface{}
	Gameplay  GameplayState
	Log       []GameRoomLog
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
	entries = append(entries, r.Gameplay.Repr(r, userId)...)
	return entries
}

func (r *GameRoom) LogMessage(history int) OrderedKeysMarshal {
	logsReprs := []OrderedKeysMarshal{}
	logs := r.Log
	if history > 0 {
		logs = r.Log[len(r.Log)-history:]
	}
	for _, entry := range logs {
		logsReprs = append(logsReprs, OrderedKeysMarshal{
			{"id", entry.Id},
			{"timestamp", entry.Timestamp},
			{"content", entry.Content},
		})
	}
	return OrderedKeysMarshal{
		{"type", "log"},
		{"log", logsReprs},
	}
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

func (r *GameRoom) BroadcastAssemblyUpdate(skipUserId int) {
	message := OrderedKeysMarshal{
		{"type", "assembly_update"},
		{"players", r.Gameplay.PlayerReprs(r)},
	}
	for userId, conn := range r.Conns {
		if userId != skipUserId {
			conn.OutChannel <- message
		}
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

// Assumes a write lock
func (r *GameRoom) BroadcastLog(text string) {
	lines := strings.Split(text, "\n")

	// Append to log
	// Keep only 5 latest
	for _, line := range lines {
		entry := GameRoomLog{Id: 0, Timestamp: time.Now().Unix(), Content: line}
		if len(r.Log) >= 1 {
			entry.Id = r.Log[len(r.Log)-1].Id + 1
		}
		if len(r.Log) >= 5 {
			for i := range 4 {
				r.Log[i] = r.Log[i+1]
			}
			r.Log[4] = entry
		} else {
			r.Log = append(r.Log, entry)
		}
	}

	message := r.LogMessage(len(lines))
	for _, conn := range r.Conns {
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

func (r *GameRoom) BroadcastGameEnd() {
	st := r.Gameplay.PhaseStatus.(GameplayPhaseStatusGameplay)
	for userId, conn := range r.Conns {
		conn.OutChannel <- OrderedKeysMarshal{
			{"type", "game_end"},
			{"relationship", st.Player[r.Gameplay.PlayerIndex(userId)].Relationship},
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
		err, logContent := r.Gameplay.Seat(user, profile)
		if err != "" {
			panic(err)
		}
		r.BroadcastAssemblyUpdate(-1)
		r.BroadcastLog(logContent)
	} else if message["type"] == "withdraw" {
		err, logContent := r.Gameplay.WithdrawSeat(msg.UserId)
		if err != "" {
			panic(err)
		}
		r.BroadcastAssemblyUpdate(-1)
		r.BroadcastLog(logContent)
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
		err, logContent := r.Gameplay.Start(r.Signal)
		if err != "" {
			panic(err)
		}
		r.BroadcastStart()
		r.BroadcastLog(logContent)
	} else if message["type"] == "appointment_accept" || message["type"] == "appointment_pass" {
		prevHolder, nextHolder, isStarting, err, logContent :=
			r.Gameplay.AppointmentAcceptOrPass(msg.UserId, message["type"] == "appointment_accept", r.Signal)
		if err != "" {
			panic(err)
		}
		r.BroadcastAppointmentUpdate(prevHolder, nextHolder, isStarting)
		r.BroadcastLog(logContent)
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
		err, logContent := r.Gameplay.ActionCheck(msg.UserId, int(handIndex), int(arenaIndex), int(target))
		if err != "" {
			panic(err)
		}
		r.BroadcastGameProgress("action_check")
		r.BroadcastLog(logContent)
	} else if message["type"] == "storytelling_end" {
		isNewMove, isGameEnd, err, logContent := r.Gameplay.StorytellingEnd(msg.UserId)
		if err != "" {
			panic(err)
		}
		if isGameEnd {
			r.BroadcastLog(logContent)
			r.BroadcastGameEnd()
			r.Gameplay.Reset()
		} else {
			var event string
			if isNewMove {
				event = "storytelling_end_new_move"
			} else {
				event = "storytelling_end_next_storyteller"
			}
			r.BroadcastGameProgress(event)
			r.BroadcastLog(logContent)
		}
	} else if message["type"] == "queue" {
		err := r.Gameplay.Queue(msg.UserId)
		if err != "" {
			panic(err)
		}
		r.BroadcastGameProgress("queue")
	} else if message["type"] == "comment" {
		text := fmt.Sprintf("%v", message["text"])
		playerIndexStr := ""
		playerIndex := r.Gameplay.PlayerIndex(msg.UserId)
		// If past assembly phase, display player index
		_, isAssembly := r.Gameplay.PhaseStatus.(GameplayPhaseStatusAssembly)
		if !isAssembly {
			playerIndexStr = fmt.Sprintf("座位 %d ", playerIndex+1)
		}
		logContent := fmt.Sprintf("%s玩家【%s】说：%s",
			playerIndexStr, r.Gameplay.Players[playerIndex].User.Nickname, text)
		r.BroadcastLog(logContent)
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
				stateMessage := r.StateMessage(sigNewConn.UserId)
				logMessage := r.LogMessage(0)
				if _, ok := r.Gameplay.PhaseStatus.(GameplayPhaseStatusAssembly); ok {
					r.BroadcastAssemblyUpdate(sigNewConn.UserId)
				}
				r.Mutex.RUnlock()
				conn.OutChannel <- stateMessage
				conn.OutChannel <- logMessage
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
				if _, ok := r.Gameplay.PhaseStatus.(GameplayPhaseStatusAssembly); ok {
					r.BroadcastAssemblyUpdate(sigLostConn.UserId)
				}
				r.Mutex.Unlock()
			}
			if sigTimer, ok := sig.(GameRoomSignalTimer); ok {
				println("timer", sigTimer.Type)
				switch sigTimer.Type {
				case "appointment":
					r.Mutex.Lock()
					prevHolder, nextHolder, isStarting, err, logContent :=
						r.Gameplay.AppointmentAcceptOrPass(-1, false, r.Signal)
					if err == "" {
						r.BroadcastAppointmentUpdate(prevHolder, nextHolder, isStarting)
					}
					r.BroadcastLog(logContent)
					r.Mutex.Unlock()

				case "gameplay":
					r.Mutex.Lock()
					if st, ok := r.Gameplay.PhaseStatus.(GameplayPhaseStatusGameplay); ok {
						if st.Step == "selection" {
							// Select random card
							_, logContent := r.Gameplay.ActionCheck(-1, -1, -1, -1)
							r.BroadcastGameProgress("action_check")
							r.BroadcastLog(logContent)
						} else if st.Step == "storytelling_holder" || st.Step == "storytelling_target" {
							// Stop storytelling
							isNewMove, isGameEnd, _, logContent := r.Gameplay.StorytellingEnd(-1)
							// XXX: DRY
							if isGameEnd {
								r.BroadcastLog(logContent)
								r.BroadcastGameEnd()
								r.Gameplay.Reset()
							} else {
								var event string
								if isNewMove {
									event = "storytelling_end_new_move"
								} else {
									event = "storytelling_end_next_storyteller"
								}
								r.BroadcastGameProgress(event)
								r.BroadcastLog(logContent)
							}
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
