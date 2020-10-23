package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

const (
	MapWidth  = 10
	MapHeight = 8
)

type CellInfo struct {
	x                int     // 0 ~ MapWidth - 1
	y                int     // 0 ~ MapHeight - 1
	gold             int     // 这个格子的分数
	cost             int     // 我移动过来的消耗
	left             int     // 我移动过来的剩余
	reachable        int     // 这个格子上可能的人数(除了我自己)
	opportunity      int     // 机遇(到达这个位置后金币数比自己到达这个位置后金币数多的人数)
	risk             int     // 风险(到达这个位置后金币数比自己到达这个位置后金币数少的人数)
	totalLeft        int     // 能到达这个位置的所有玩家剩余金币总和(用来算平均分)
	totalOpportunity int     // 所有到达这个位置后金币比我多的人的金币总和
	totalRisk        int     // 所有到达这个位置后金币比我少的人的金币综合
	expected         float64 // 预期收益
}

type GameScore struct {
	Name string
	Gold int
}

type PlayerInfo struct {
	Name string
	X    int
	Y    int
	Gold int
}

/*
	msgtype 0登陆 -1登陆失败 1请准备 2准备好了 3游戏信息 4玩家行动 5游戏结束
*/
type Msg struct {
	Msgtype int
	Token   string
	RoundID int
	X       int
	Y       int
	Sorted  []*GameScore `json:"Results,omitempty"`
}

type Tile struct {
	Gold    int
	Players []*GameScore `json:"Players,omitempty"`
}

type Game struct {
	GameID  uint64
	Msgtype int
	status  int //-1无效0准备1开始
	RoundID int
	Wid     int
	Hei     int

	Tilemap [MapHeight][MapWidth]*Tile

	roundRecords []string
	Sorted       []*GameScore `json:"Results,omitempty"`
}

type Record struct {
	RoundId  int
	X        int
	Y        int
	Gold     int
	Expected float64
	TargetX  int
	TargetY  int
	Crowded  int
}

func saveGame(gameID uint64) error {
	filename := fmt.Sprintf("./log%d-%d-%d-%d.txt", time.Now().YearDay(), time.Now().Hour(), time.Now().Minute(), time.Now().Second())
	file, err := os.Create(filename)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer file.Close()

	gameStr := fmt.Sprintf("Game: %d\n", gameID)
	file.Write([]byte(gameStr))
	for i := 0; i < len(records); i++ {
		bytes, e := json.Marshal(records[i])
		if e == nil {
			file.Write(bytes)
			file.Write([]byte("\n"))
		} else {
			return e
		}
	}

	return nil
}

func sendMessage(data interface{}) error {
	if ws != nil {
		return ws.WriteJSON(data)
	}
	return errors.New("ws is nil")
}

func recvMessage(data interface{}) error {
	if ws != nil {
		return ws.ReadJSON(data)
	} else {
		return errors.New("ws == nil")
	}
}

func login(uri, token string) error {
	if ws != nil {
		ws.Close()
	}

	var err error
	ws, _, err = websocket.DefaultDialer.Dial(uri, nil)
	if err != nil {
		fmt.Println("websocket连接出错, err = ", err)
		time.Sleep(time.Second)
		return err
	}

	loginMsg := Msg{}
	loginMsg.Msgtype = 0
	loginMsg.Token = token
	err = sendMessage(loginMsg)
	if err != nil {
		fmt.Println("Send error while login msg, err = ", err)
		time.Sleep(time.Second)
		return err
	}

	for {
		time.Sleep(time.Millisecond * 10)
		data := Msg{}
		err = recvMessage(&data)
		if err == nil {
			if data.Msgtype == -1 {
				time.Sleep(time.Second * 2)
				fmt.Println("服务器拒绝, 登录失败")
				return errors.New("login failed")
			}
			fmt.Println("登录成功, msg = ", data)
			return nil
		} else {
			return err
		}
	}
}

func sendMove(x, y, roundId int) error {
	if x < 0 {
		x = 0
	} else if x >= MapWidth {
		x = MapWidth - 1
	}

	if y < 0 {
		y = 0
	} else if y >= MapHeight {
		y = MapHeight - 1
	}

	msg := Msg{}
	msg.Msgtype = 4
	msg.Token = myToken
	msg.RoundID = roundId
	msg.X = x
	msg.Y = y
	return sendMessage(msg)
}

func updateFrame(frameData Game) {
	// 捕获处理过程中的异常, 保证不会出现闪退
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(time.Now(), "捕获到异常: ", r)
		}
	}()

	roundId := frameData.RoundID
	// 整理出所有的玩家信息
	players := make([]*PlayerInfo, 0, 8)
	for i := 0; i < len(frameData.Tilemap); i++ {
		for j := 0; j < len(frameData.Tilemap[i]); j++ {
			if len(frameData.Tilemap[i][j].Players) > 0 {
				playerLen := len(frameData.Tilemap[i][j].Players)
				for k := 0; k < playerLen; k++ {
					p := frameData.Tilemap[i][j].Players[k]
					if p.Name == myName || p.Name == myToken {
						myX = j
						myY = i
						myGold = p.Gold
						records[roundId].Gold = p.Gold
						records[roundId].RoundId = roundId
						records[roundId].X = myX
						records[roundId].Y = myY
						records[roundId].Crowded = playerLen
					} else {
						players = append(players, &PlayerInfo{Name: p.Name, Gold: p.Gold, X: j, Y: i})
					}
				}
			}
		}
	}

	// 给所有玩家拍个序
	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			if players[i].Gold < players[j].Gold {
				temp := players[i]
				players[i] = players[j]
				players[j] = temp
			}
		}
	}

	isfirst := myGold > players[0].Gold

	// 根据棋盘数据生成 cells info
	cells := make([]*CellInfo, 0, 48)
	for i := 0; i < len(frameData.Tilemap); i++ {
		for j := 0; j < len(frameData.Tilemap[i]); j++ {
			c := CellInfo{x: j, y: i, gold: frameData.Tilemap[i][j].Gold, reachable: 0, opportunity: 0, risk: 0}
			if i == myY && j == myX {
				c.cost = 1
				c.left = myGold - 1
				c.totalLeft += c.left
			} else {
				cost := int(math.Floor((math.Abs(float64(i-myY)) + math.Abs(float64(j-myX))) * 1.5))
				if cost > myGold {
					// 不可到达
					continue
				} else {
					c.cost = cost
					c.left = myGold - cost
					c.totalLeft += c.left
				}
			}

			// 计算其他玩家到这里的消耗
			for k := 0; k < len(players); k++ {
				player := players[k]
				cost := int(math.Floor((math.Abs(float64(i-player.Y)) + math.Abs(float64(j-player.X))) * 1.5))
				if cost >= player.Gold {
					c.reachable++
					left := player.Gold - cost
					if left > c.left {
						c.opportunity++
						c.totalOpportunity += left
					} else {
						c.risk++
						c.totalRisk += left
					}
					c.totalLeft += left
				}
			}

			// 计算预期收益
			if isfirst || (myGold >= (roundId*11) && roundId >= 50) {
				// 第一名的时候, 每次都找负分数的地方躲
				if c.gold == 0 || c.gold == 1 || c.gold == -1 {
					c.expected = 10
				} else if c.gold == -4 {
					c.expected = -10
				} else {
					c.expected = float64(-c.gold)
				}
			} else {
				if c.gold == -4 {
					// a.当金币数量为 -4 时，随机选取该格子中1一个玩家 (马上获得 40% 的利息，利息最高为 10 个金币) 或者 (失去 4个金币) 概率为50%。
					c.expected = (math.Max(float64(c.left)*0.4, 10)*0.5-2)/float64(c.reachable+1) - float64(c.cost)
					if c.reachable > 0 {
						oppoProfit := (float64(c.totalOpportunity) * 0.25) / (float64(c.reachable) * float64(1+c.risk))
						riskProfit := (float64(c.left) * 0.25 * float64(c.risk)) / float64(c.reachable)
						c.expected += oppoProfit
						c.expected -= riskProfit
					}
				} else if c.gold > 0 && c.gold%5 == 0 {
					// b.当金币数量大于0且能整除5，该格子中的玩家金币数量全部为 (该格所有玩家金币总数+该金币数量)/玩家数量 并取整。
					t := float64(c.totalOpportunity)*oppoWeight + float64(c.totalRisk)*riskWeight + float64(c.left)
					c.expected = t/float64(c.reachable+1) - float64(c.left) - float64(c.cost)
				} else if c.gold == 7 || c.gold == 11 {
					// c.当金币数量为 7或者11 时，如果该格子只有1个玩家，该玩家获得对应数量金币，如果该格子有多个玩家，随机选取一个玩家失去对应数量金币。
					if c.reachable == 0 {
						c.expected = float64(c.gold)
					} else {
						c.expected = float64(-c.gold) / float64(c.reachable+1)
						oppoProfit := (float64(c.totalOpportunity) * 0.25) / (float64(c.reachable) * float64(1+c.risk))
						riskProfit := (float64(c.left) * 0.25 * float64(c.risk)) / float64(c.reachable)
						c.expected += oppoProfit
						c.expected -= riskProfit
					}
				} else if c.gold == 8 {
					// d.当金币数量为 8 时，该格子玩家平分（取整）这8个金币。
					c.expected = 8/float64(c.reachable+1) - float64(c.cost)
					if c.reachable > 0 {
						oppoProfit := (float64(c.totalOpportunity) * 0.25) / (float64(c.reachable) * float64(1+c.risk))
						riskProfit := (float64(c.left) * 0.25 * float64(c.risk)) / float64(c.reachable)
						c.expected += oppoProfit
						c.expected -= riskProfit
					}
				} else {
					c.expected = float64(c.gold)/(float64(c.opportunity)*oppoWeight+float64(c.risk)*riskWeight+1) - float64(c.cost)
					if c.reachable > 0 {
						oppoProfit := (float64(c.totalOpportunity) * 0.25) / (float64(c.reachable) * float64(1+c.risk))
						riskProfit := (float64(c.left) * 0.25 * float64(c.risk)) / float64(c.reachable)
						c.expected += oppoProfit
						c.expected -= riskProfit
					}
				}
			}

			cells = append(cells, &c)
		}
	}

	// cells 根据 expected 来排序
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			if cells[i].expected < cells[j].expected {
				temp := cells[i]
				cells[i] = cells[j]
				cells[j] = temp
			}
		}
	}

	// 找出预期值最高的n个, 再随机选一个
	highest := make([]*CellInfo, 0, 8)
	bestEx := cells[0].expected
	highest = append(highest, cells[0])
	for i := 0; i < len(cells); i++ {
		if cells[i].expected == bestEx {
			highest = append(highest, cells[i])
		} else {
			break
		}
	}

	rnd := rand.Int31n(int32(len(highest)))

	records[roundId].TargetX = cells[rnd].x
	records[roundId].TargetY = cells[rnd].y
	records[roundId].Expected = cells[rnd].expected
	fmt.Println(records[roundId])
	sendMove(cells[rnd].x, cells[rnd].y, frameData.RoundID)
}

var myX, myY, myGold int

var myName string = "追光骑士团"
var myToken string = "DyKiQSgpDhrtMQSsVgvs7NWtS7A79XLI"

var ws *websocket.Conn
var records [96]Record

var oppoWeight float64 = 1
var riskWeight float64 = 1

func main() {
	//online
	uri := "ws://pgame.51wnl-cq.com:8881/ws"
	//local debug
	//uri := "ws://localhost:8881/ws"
	rand.Seed(time.Now().UnixNano())
LOGIN:
	err := login(uri, myToken)
	if err != nil {
		goto LOGIN
	}
	defer func() {
		if ws != nil {
			ws.Close()
		}
	}()

PREPARE:
	for {
		time.Sleep(time.Millisecond * 10) // 睡 10 毫秒
		data := Msg{}
		err = recvMessage(&data)
		if err == nil {
			if data.Msgtype == 1 {
				fmt.Println("准备")
				s := Msg{}
				s.Msgtype = 2
				s.Token = myToken
				sendMessage(s)
				goto GAMELOOP
			}
		} else {
			goto LOGIN
		}
	}
GAMELOOP:
	for {
		time.Sleep(time.Millisecond * 10) // 睡 10 毫秒
		data := Game{}
		err = recvMessage(&data)
		if err == nil {
			if data.Msgtype == 5 {
				saveGame(data.GameID)
				fmt.Println("游戏结束, GameId = ", data.GameID)
				for i := 0; i < len(data.Sorted); i++ {
					fmt.Println("Name = ", data.Sorted[i].Name, ", Gold = ", data.Sorted[i].Gold)
				}
				fmt.Println(records)
				goto PREPARE
			} else if data.Msgtype == 3 {
				updateFrame(data)
			}
		} else {
			fmt.Println(err)
			goto LOGIN
		}
	}
}
