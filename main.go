package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/marusama/cyclicbarrier"
	"math"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

const count int = 100
const startPort int = 30000
const gossip int = 3

var cyc cyclicbarrier.CyclicBarrier
var signal int = 0
var lockForColored sync.Mutex
var lockForUdpNums sync.Mutex

//var lockForRoundNums sync.Mutex
var udpNums int = 0
var roundNums int = 0

func main() {
	var port int
	colored := make(map[int]int)
	round := 1
	ch := make(chan int, 3)
	cyc = cyclicbarrier.New(3)
	go func(round *int) {
		var maxWaitingNum float64
		for {
			if 1 == signal {
				maxWaitingNum = math.Pow(float64(gossip), float64(*round))
				if cyc.GetNumberWaiting() == int(maxWaitingNum) {
					cyc = cyclicbarrier.New(int(maxWaitingNum))
					fmt.Printf("cyclicbarrier.New(maxWaitingNum:%d)\n", int(maxWaitingNum))
					signal = 0
				}
			}
		}
	}(&round)

	for i := 0; i < count; i++ {
		port = startPort + i
		go gossipListener(port, &round, colored, ch)
	}

	time.Sleep(2 * time.Second)
	for len(colored) < count {
	}

	rowCount := 10
	i := 0
	for k := startPort; k < startPort+count; k++ {
		i++
		fmt.Print(k, ":", colored[k])
		fmt.Print("|")
		if 0 == i%rowCount {
			fmt.Println()
		}
	}

}

func gossipListener(port int, round *int, colored map[int]int, ch chan int) {
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
		var data [10 * 1024]byte
		n, _, err := listen.ReadFromUDP(data[:]) // 接收数据
		if err != nil {
			fmt.Println("read udp failed, err: ", err)
			continue
		}

		//fmt.Printf("data:%v addr:%v count:%v\n", string(data[:n]), addr, n)
		var msg Message
		err = json.Unmarshal(data[:n], &msg)
		if err != nil {
			fmt.Println("err: ", err)
			continue
		} else {
			lockForUdpNums.Lock()
			udpNums++
			roundNums++
			fmt.Printf("Data=%s, Round=%d, Path=%s, updnums=%d, roundnums=%d\n", msg.Data, msg.Round, msg.Path, udpNums, roundNums)
			lockForUdpNums.Unlock()
		}

		lockForColored.Lock()
		if 0 == colored[port] {
			colored[port] = 1
		}
		lockForColored.Unlock()

		var historyNodeList [gossip + 1]int
		historyNodeList[0] = port
		var k int = 1
		for _, value := range rand.Perm(count)[:4] {
			randPort := value + startPort
			if port != randPort {
				historyNodeList[k] = randPort
				k++
			}
			if k == 4 {
				break
			}
		}

		for i := 0; i < gossip; i++ {
			go func(j int) {
				_ = cyc.Await(context.Background())
				*round = msg.Round + 1
				signal = 1
				if 0 != roundNums {
					roundNums = 0
				}

				ch <- 1
				udpAddr := net.UDPAddr{
					IP:   ip,
					Port: historyNodeList[j+1],
				}
				pMsg := Message{Data: msg.Data, Round: msg.Round + 1, Path: msg.Path + "->" + strconv.Itoa(udpAddr.Port)}
				sendData, _ := json.Marshal(&pMsg)
				_, err = listen.WriteToUDP(sendData, &udpAddr) // 发送数据
				if err != nil {
					fmt.Println("Write to udp failed, err: ", err)
				}
				time.Sleep(100 * time.Millisecond)
				<-ch
			}(i)
			time.Sleep(10 * time.Millisecond)
		}

	}
}
