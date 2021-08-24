package main

import (
	"context"
	"fmt"
	"github.com/marusama/cyclicbarrier"
	"math/rand"
	"net"
	"time"
)

var count int = 100
var startPort int = 30000
var gossip int = 3
var cyc cyclicbarrier.CyclicBarrier

func main() {
	//var wg sync.WaitGroup
	var port int
	colored := make(map[int]int)
	round := 0
	//wg.Add(count)

	go func(round *int) {
		for {
			maxWaitingNum := *round * gossip
			if 0 != maxWaitingNum && cyc.GetNumberWaiting() == maxWaitingNum {
				cyc = cyclicbarrier.New(maxWaitingNum)
			}
		}
	}(&round)

	for i := 0; i < count; i++ {
		port = startPort + i
		go gossipListener(port /* &wg,*/, &round, colored)
	}

	time.Sleep(2 * time.Second)
	for len(colored) < count {
		time.Sleep(200 * time.Millisecond)
	}

	//wg.Wait()
}

func gossipListener(port int /*wg *sync.WaitGroup,*/, round *int, colored map[int]int) {
	//defer wg.Done()
	ip := net.ParseIP("127.0.0.1")
	listen, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   ip,
		Port: port,
	})
	if err != nil {
		fmt.Println("Listen failed, err: ", err)
		return
	}
	defer func(listen *net.UDPConn) {
		err := listen.Close()
		if err != nil {
			panic("❌")
		}
	}(listen)

	fmt.Println("[", ip, ":", port, "]", "start listening")

	//var sLastRecData string = ""
	for {
		var data [128]byte
		n, addr, err := listen.ReadFromUDP(data[:]) // 接收数据
		if err != nil {
			fmt.Println("read udp failed, err: ", err)
			continue
		}
		if 0 == colored[port] {
			colored[port] = 1
		}
		fmt.Printf("data:%v addr:%v count:%v\n", string(data[:n]), addr, n)

		//if sLastRecData != string(data[:n]) {
		//	sLastRecData = string(data[:n])
		//
		//	_, err = listen.WriteToUDP(data[:n], addr) // 发送数据
		//	if err != nil {
		//		fmt.Println("Write to udp failed, err: ", err)
		//	}
		//}

		_ = cyc.Await(context.Background())

		var historyNodeList [4]int
		historyNodeList[0] = port
		var k int = 1
		for _, value := range rand.Perm(count)[:4] {
			randPort := value + startPort
			if port != randPort {
				historyNodeList[n] = randPort
				k++
			}
			if k == 4 {
				break
			}
		}

		for i := 0; i < 3; i++ {
			go func(sendData []byte) {
				_, err = listen.WriteToUDP(sendData, &net.UDPAddr{
					IP:   ip,
					Port: historyNodeList[i+1],
				}) // 发送数据
				if err != nil {
					fmt.Println("Write to udp failed, err: ", err)
				}
			}(data[:n])
			time.Sleep(10 * time.Millisecond)
		}

	}
}
