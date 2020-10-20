package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

const (
	MapWidth  = 8
	MapHeight = 6
)

type CellInfo struct {
	row         int // 0 ~ MapWidth - 1
	line        int // 0 ~ MapHeight - 1
	gold        int // 这个格子的分数
	reachable   int // 这个格子上可能的人数
	opportunity int // 机遇(到达这个位置后金币数比自己到达这个位置后金币数多的人数)
	risk        int // 风险(到达这个位置后金币数比自己到达这个位置后金币数少的人数)
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

func updateFrame(frameData Game) {
	// 捕获处理过程中的异常, 保证不会出现闪退
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(time.Now(), "捕获到异常: ", r)
		}
	}()

	fmt.Println("RoundId: ", frameData.RoundID)
	// 整理出所有的玩家信息
	fmt.Println("Sorted: ", len(frameData.Sorted))
	players := make([]*PlayerInfo, 0, 8)
	for i := 0; i < len(frameData.Tilemap); i++ {
		for j := 0; j < len(frameData.Tilemap[i]); j++ {
			if len(frameData.Tilemap[i][j].Players) > 0 {
				//fmt.Println("x, y = ", i, j, "; gold = ", frameData.Tilemap[i][j].Gold)
				for k := 0; k < len(frameData.Tilemap[i][j].Players); k++ {
					//fmt.Println("gold = ", frameData.Tilemap[i][j].P[k].Gold, ", name = ", frameData.Tilemap[i][j].P[k].Name)
					p := frameData.Tilemap[i][j].Players[k]
					players = append(players, &PlayerInfo{Name: p.Name, Gold: p.Gold, X: i, Y: j})
				}
			}
		}
	}

	fmt.Println("players.count = ", len(players))
}

func sendMessage(data interface{}) error {
	if ws != nil {
		return ws.WriteJSON(data)
	}
	return errors.New("ws is nil")
}

func recvMessage(data interface{}) (err error) {
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

var ws *websocket.Conn
var scores []int

func main() {
	//online
	uri := "ws://pgame.51wnl-cq.com:8881/ws"
	//local debug
	//uri := "ws://localhost:8881/ws"
	token := "追光骑士团"
LOGIN:
	err := login(uri, token)
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
				s.Token = token
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
				fmt.Println("游戏结束")
				for i := 0; i < len(data.Sorted); i++ {
					fmt.Println("Name = ", data.Sorted[i].Name, ", Gold = ", data.Sorted[i].Gold)
				}
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
