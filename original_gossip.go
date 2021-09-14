package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/marusama/cyclicbarrier"
	"math/rand"
	"net"
	"strconv"
	"time"
)

func GA(port int, round *int, colored map[int]int, ch chan int, csvWriter *csv.Writer) {
	ip := net.ParseIP(localhost)
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

	var isFirst bool = true // 是否首次收到消息
	var firstMsg Message
	for {
		var data [10 * 1024]byte
		n, _, err := listen.ReadFromUDP(data[:]) // 接收数据
		select {
		case <-doneCh:
			return
		default:
		}
		if err != nil {
			fmt.Println("read udp failed, err: ", err)
			continue
		}

		//fmt.Printf("data:%v addr:%v count:%v\n", string(data[:n]), addr, n)
		var msg Message
		err = json.Unmarshal(data[:n], &msg) //反序列化json保存到Message结构体中
		if err != nil {
			fmt.Println("err: ", err)
			continue
		}

		lockForColored.Lock()
		colored[port]++ //记录节点收到消息的次数
		lockForColored.Unlock()

		go func() {
			//fmt.Println("reach barrier", port)
			_ = cyc.Await(context.Background()) //实现同步时钟模型，等待每轮所有消息均分发完毕才允许进入下一轮传播
			//fmt.Println("cross barrier", port)
			lockForwaitingNum.Lock()
			waitingNum++
			res := cycParties == waitingNum //检查是否当前轮次所有传播任务均完成
			lockForwaitingNum.Unlock()
			if res { //开启新的一轮传播，重置屏障
				csvWriter.Write([]string{strconv.Itoa(*round), strconv.Itoa(udpNums), strconv.Itoa(roundNums), strconv.Itoa(len(colored))})
				csvWriter.Flush()
				*round++
				cycParties = len(colored) // 计算下一轮次的总传播数
				cyc.Reset()
				cyc = cyclicbarrier.New(cycParties)
				fmt.Printf("cyclicbarrier.New(cycParties:%d), round:%d\n", cycParties, *round)
				time.Sleep(100 * time.Millisecond)
				close(waitCh)
				waitCh = make(chan struct{})
				waitingNum = 0
				roundNums = 0
			}
		}()

		if isFirst {
			isFirst = false
			firstMsg = msg
			go func(msg Message) {
				for { //阻塞等待下一轮屏障刷新
					select {
					case <-doneCh:
						return
					case <-waitCh:
						break
					}
					ch <- 1
					var randNeighbor int //随机选择待分发的节点
					randNeighborSlice := rand.Perm(cfg.Count)[:2]
					if randNeighborSlice[0] != port {
						randNeighbor = cfg.Firstnode + randNeighborSlice[0]
					} else {
						randNeighbor = cfg.Firstnode + randNeighborSlice[1]
					}

					select {
					case <-doneCh:
						return
					default:
					}
					udpAddr := net.UDPAddr{
						IP:   ip,
						Port: randNeighbor,
					}
					pMsg := Message{Data: msg.Data, Round: *round, Path: msg.Path /*strconv.Itoa(port)*/ + "->" + strconv.Itoa(udpAddr.Port)}
					sendData, _ := json.Marshal(&pMsg)
					mutex.Lock()
					udpNums++
					roundNums++
					fmt.Printf("Data=%s, Round=%d, Path=%s, updnums=%d, roundnums=%d\n", pMsg.Data, pMsg.Round, pMsg.Path, udpNums, roundNums)
					mutex.Unlock()

					select {
					case <-doneCh:
						return
					default:
					}
					_, err = listen.WriteToUDP(sendData, &udpAddr) // 发送数据
					if err != nil {
						fmt.Println("Write to udp failed, err: ", err)
					}
					time.Sleep(10 * time.Millisecond)
					<-ch
				}
			}(firstMsg)
		}
	}
}

func PGA(port int, i *int, list map[int]bool, list2 map[int]bool, list3 map[int]bool, list4 map[int]int, colored map[int]int, ch chan int, writer *csv.Writer) {

}

func NGA(port int, i *int, list map[int]bool, list2 map[int]bool, list3 map[int]bool, list4 map[int]int, colored map[int]int, ch chan int, writer *csv.Writer) {

}

func MGA(port int, i *int, list map[int]bool, list2 map[int]bool, list3 map[int]bool, list4 map[int]int, colored map[int]int, ch chan int, writer *csv.Writer) {

}

func MPGA(port int, i *int, list map[int]bool, list2 map[int]bool, list3 map[int]bool, list4 map[int]int, colored map[int]int, ch chan int, writer *csv.Writer) {

}

func MNGA(port int, i *int, list map[int]bool, list2 map[int]bool, list3 map[int]bool, list4 map[int]int, colored map[int]int, ch chan int, writer *csv.Writer) {

}

func originalGossiper(port int, round *int, colored map[int]int, ch chan int) {
	ip := net.ParseIP(localhost)
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

	for {
		var data [10 * 1024]byte
		n, _, err := listen.ReadFromUDP(data[:]) // 接收数据
		select {
		case <-doneCh:
			return
		default:
		}
		if err != nil {
			fmt.Println("read udp failed, err: ", err)
			continue
		}

		//fmt.Printf("data:%v addr:%v count:%v\n", string(data[:n]), addr, n)
		var msg Message
		err = json.Unmarshal(data[:n], &msg) //反序列化json保存到Message结构体中
		if err != nil {
			fmt.Println("err: ", err)
			continue
		}

		go func(msg Message) {
			mutex.Lock()
			fmt.Printf("Data=%s, Round=%d, Path=%s, updnums=%d, roundnums=%d\n", msg.Data, msg.Round, msg.Path, udpNums, roundNums)
			udpNums++
			roundNums++
			colored[port]++ //记录节点收到消息的次数
			mutex.Unlock()

			_ = cyc.Await(context.Background()) //实现同步时钟模型，等待每轮所有消息均分发完毕才允许进入下一轮传播
			lockForwaitingNum.Lock()
			waitingNum++
			res := cycParties == waitingNum //检查是否当前轮次所有传播任务均完成
			lockForwaitingNum.Unlock()
			if res { //开启新的一轮传播，重置屏障
				*round++
				cycParties *= cfg.Gossipfactor // 计算下一轮次的总传播数
				cyc.Reset()
				cyc = cyclicbarrier.New(cycParties)
				close(waitCh)
				waitCh = make(chan struct{})
				waitingNum = 0
				roundNums = 1
				fmt.Printf("cyclicbarrier.New(cycParties:%d), round:%d\n", cycParties, *round)
			} else { //阻塞等待下一轮屏障刷新
				select {
				case <-doneCh:
					return
				case <-waitCh:
					break
				}
			}

			randNeighborList := make([]int, cfg.Gossipfactor) //随机选择待分发的节点
			var k int = 0
			for _, value := range rand.Perm(cfg.Count)[:cfg.Gossipfactor+1] {
				randPort := value + cfg.Firstnode
				if port != randPort {
					randNeighborList[k] = randPort
					k++
				}
				if k == cfg.Gossipfactor {
					break
				}
			}

			for i := 0; i < cfg.Gossipfactor; i++ {
				select {
				case <-doneCh:
					return
				default:
				}
				go func(j int) {
					ch <- 1
					udpAddr := net.UDPAddr{
						IP:   ip,
						Port: randNeighborList[j],
					}
					pMsg := Message{Data: msg.Data, Round: msg.Round + 1, Path: msg.Path + "->" + strconv.Itoa(udpAddr.Port)}
					sendData, _ := json.Marshal(&pMsg)

					select {
					case <-doneCh:
						return
					default:
					}
					_, err = listen.WriteToUDP(sendData, &udpAddr) // 发送数据
					if err != nil {
						fmt.Println("Write to udp failed, err: ", err)
					}
					time.Sleep(100 * time.Millisecond)
					<-ch
				}(i)
			}
		}(msg)

	}
}
