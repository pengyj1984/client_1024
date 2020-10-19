package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

const (
	MapWidth  = 8
	MapHeight = 6
)

type CellInfo struct {
	row int				// 0 ~ MapWidth - 1
	line int			// 0 ~ MapHeight - 1
	gold int			// 这个格子的分数
	reachable int		// 这个格子上可能的人数
	opportunity int		// 机遇(到达这个位置后金币数比自己到达这个位置后金币数多的人数)
	risk int			// 风险(到达这个位置后金币数比自己到达这个位置后金币数少的人数)
}

type Save struct{
	Name string
	Token string
}

type LoginMsg struct{
	msgtype int
	token string
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
		if r := recover(); r != nil{
			fmt.Println(time.Now(), "捕获到异常: ", r)
		}
	}()


}

var ws *websocket.Conn

func sendMessage(data interface{}){
	if ws != nil{
		ws.WriteJSON(data)
	}
}

func recvMessage()(data interface{}, err error){
	if ws != nil{
		var data interface{}
		err := ws.ReadJSON(data)
		return data, err
	}else{
		return nil, errors.New("ws == nil")
	}
}

func main() {
	//online
    // uri := "ws://pgame.51wnl-cq.com:8881/ws"
    //local debug
    uri := "ws://localhost:8881/ws"
	LOGIN:
	var save Save
	file, err := os.Open("./save.txt")
	defer file.Close()
	if err != nil {
		fmt.Println(time.Now(), "找不到save.txt, 新建一个")
		file, err = os.Create("./save.txt")
		str := `{"Name":"追光骑士团", "Token":""}`
		json.Unmarshal([]byte(str), &save)
		fmt.Println(save)
		file.Write([]byte(str))
	}else{
		bytes := make([]byte, 128)
		var count int
		count, err = file.Read(bytes)
		data := bytes[0:count]
		json.Unmarshal(data, &save)
		fmt.Println(save.Name + ", " + save.Token)
	}

	ws, _, err = websocket.DefaultDialer.Dial(uri, nil)
	if err != nil{
		fmt.Println("websocket连接出错, err = ", err)
		time.Sleep(time.Second * 2)				// 睡 2 秒再试
		goto LOGIN
	}
	defer ws.Close()
	loginMsg := LoginMsg{}
	loginMsg.msgtype = 0
	loginMsg.token = save.Name
	sendMessage(loginMsg)
	
	for{
		time.Sleep(time.Millisecond * 10)
		recv, err = recvMessage()
		if err == nil{
			
		}
	}


	for {
		time.Sleep(time.Millisecond * 10)				// 睡 10 毫秒
		var frameData interface{}
		frameData, err = recvMessage()
		if err == nil{
			updateFrame(frameData)
		}else{
			fmt.Println(err)
		}
	}
}