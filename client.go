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

type Save struct {
	Name  string
	Token string
}

type GameScore struct {
	Name string
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
	P       []*GameScore `json:"Players,omitempty"`
	players map[string]*PlayerInfo
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
}

type PlayerInfo struct {
	Key  string
	X    int
	Y    int
	Gold int
}

func updateFrame(frameData interface{}) {
	// 捕获处理过程中的异常, 保证不会出现闪退
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(time.Now(), "捕获到异常: ", r)
		}
	}()

	fmt.Println(frameData)
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
				time.Sleep(time.Second)
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

func main() {
	//online
	uri := "ws://pgame.51wnl-cq.com:8881/ws"
	//local debug
	//uri := "ws://localhost:8881/ws"
	token := "追光骑士团1"
LOGIN:
	err := login(uri, token)
	if err != nil {
		goto LOGIN
	}
	defer ws.Close()

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
			} else {
				updateFrame(data)
			}
		} else {
			fmt.Println(err)
			goto LOGIN
		}
	}
}
