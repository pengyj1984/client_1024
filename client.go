package main

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/gorilla/websocket"
)

const (
	MapWidth  = 8
	MapHeight = 6
)

type CellInfo struct {
	x           int // 0 ~ MapWidth - 1
	y           int // 0 ~ MapHeight - 1
	gold        int // 这个格子的分数
	cost        int // 我移动过来的消耗
	left        int // 我移动过来的剩余
	reachable   int // 这个格子上可能的人数(除了我自己)
	opportunity int // 机遇(到达这个位置后金币数比自己到达这个位置后金币数多的人数)
	risk        int // 风险(到达这个位置后金币数比自己到达这个位置后金币数少的人数)
	totalLeft   int // 能到达这个位置的所有玩家剩余金币总和(用来算平均分)

	expected int // 预期收益
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

	fmt.Println("RoundId: ", frameData.RoundID)
	// 整理出所有的玩家信息
	players := make([]*PlayerInfo, 0, 8)
	//fmt.Println("len(frameData.Tilemap) = ", len(frameData.Tilemap))		-- MapHeight
	//fmt.Println("len(frameData.Tilemap[0]) = ", len(frameData.Tilemap[0]))		-- MapWidth
	for i := 0; i < len(frameData.Tilemap); i++ {
		for j := 0; j < len(frameData.Tilemap[i]); j++ {
			if len(frameData.Tilemap[i][j].Players) > 0 {
				for k := 0; k < len(frameData.Tilemap[i][j].Players); k++ {
					p := frameData.Tilemap[i][j].Players[k]
					if p.Name == myName {
						myX = j
						myY = i
						myGold = p.Gold
						scores[frameData.RoundID] = p.Gold
					} else {
						players = append(players, &PlayerInfo{Name: p.Name, Gold: p.Gold, X: j, Y: i})
					}
				}
			}
		}
	}
	fmt.Println("X, Y, Gold = ", myX, ", ", myY, ", ", myGold)

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
					} else {
						c.risk++
					}
					c.totalLeft += left
				}
			}

			// 计算预期收益
			if c.gold == -4 {
				// a.当金币数量为 -4 时，随机选取该格子中1一个玩家 (马上获得 40% 的利息，利息最高为 10 个金币) 或者 (失去 4个金币) 概率为50%。
				c.expected = int((math.Max(float64(myGold)*0.4, 10)*0.5 - 2) / float64(c.reachable+1))
			} else if c.gold > 0 && c.gold%5 == 0 {
				// b.当金币数量大于0且能整除5，该格子中的玩家金币数量全部为 (该格所有玩家金币总数+该金币数量)/玩家数量 并取整。
				c.expected = int(float64(c.totalLeft+c.gold)/float64(c.reachable+1)) - c.left + int(float64(c.opportunity)*0.8) - int(float64(c.risk)*1.2)
			} else if c.gold == 7 || c.gold == 11 {
				// c.当金币数量为 7或者11 时，如果该格子只有1个玩家，该玩家获得对应数量金币，如果该格子有多个玩家，随机选取一个玩家失去对应数量金币。
				if c.reachable == 0 {
					c.expected = c.gold
				} else {
					c.expected = int(float64(-c.gold) / float64(c.reachable+1))
				}
			} else if c.gold == 8 {
				// d.当金币数量为 8 时，该格子玩家平分（取整）这8个金币。
				c.expected = int(float64(c.totalLeft)/float64(c.reachable+1)) - c.left + int(float64(c.opportunity)*0.8) - int(float64(c.risk)*1.2)
			} else {
				c.expected = int(float64(c.gold)/float64(c.reachable+1) + float64(c.opportunity)*0.8 - float64(c.risk)*1.2)
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

	fmt.Println("Target = ", cells[0].x, ", ", cells[0].y, "; Expected = ", cells[0].expected)
	sendMove(cells[0].x, cells[0].y, frameData.RoundID)
}

var myX, myY, myGold int

var myName string = "追光骑士团"
var myToken string = "追光骑士团"

var ws *websocket.Conn
var scores [96]int

func main() {
	//online
	uri := "ws://pgame.51wnl-cq.com:8881/ws"
	//local debug
	//uri := "ws://localhost:8881/ws"
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
				fmt.Println("游戏结束, GameId = ", data.GameID)
				for i := 0; i < len(data.Sorted); i++ {
					fmt.Println("Name = ", data.Sorted[i].Name, ", Gold = ", data.Sorted[i].Gold)
				}
				fmt.Println(scores)
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
