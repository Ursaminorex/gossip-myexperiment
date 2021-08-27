package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
)

type Port struct {
	Firstnode int
}

func main() {
	port := Port{}
	path := "config.json"
	file, err := os.Open(path)
	if err != nil {
		panic("can not open file:" + path)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&port)
	if err != nil {
		port.Firstnode = 30000
	}
	ip := net.ParseIP("127.0.0.1")

	socket, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   ip,
		Port: port.Firstnode,
	})
	if err != nil {
		fmt.Println("连接UDP服务器失败，err: ", err)
		return
	}
	defer socket.Close()
	//data := make([]byte, 128)
	var sendData string
	sendData = "{\"data\":\"gossip test\", \"round\":0, \"path\":\"" + strconv.Itoa(port.Firstnode) + "\"}"
	_, err = socket.Write([]byte(sendData)) // 发送数据
	if err != nil {
		fmt.Println("发送数据失败，err: ", err)
		return
	}
	fmt.Println("sendData: ", sendData)
}
