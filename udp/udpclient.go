package main

import (
	"fmt"
	"net"
)

func main() {
	ip := net.ParseIP("127.0.0.1")
	socket, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   ip,
		Port: 30000,
	})
	if err != nil {
		fmt.Println("连接UDP服务器失败，err: ", err)
		return
	}
	defer socket.Close()
	//data := make([]byte, 128)
	var sendData string
	sendData = "{\"data\":\"gossip test\", \"round\":0, \"path\":\"30000\"}"
	_, err = socket.Write([]byte(sendData)) // 发送数据
	if err != nil {
		fmt.Println("发送数据失败，err: ", err)
		return
	}
	fmt.Println("sendData: ", sendData)
	//for {
	//	fmt.Print("请输入要发送的内容：")
	//	fmt.Scan(&sendData)
	//	if sendData == "exit" {
	//		break
	//	}
	//	_, err = socket.Write([]byte(sendData)) // 发送数据
	//	if err != nil {
	//		fmt.Println("发送数据失败，err: ", err)
	//		return
	//	}
	//	n, remoteAddr, err := socket.ReadFromUDP(data) // 接收数据
	//	if err != nil {
	//		fmt.Println("接收数据失败, err: ", err)
	//		return
	//	}
	//	fmt.Printf("recv:%v addr:%v count:%v\n", string(data[:n]), remoteAddr, n)
	//}
}
