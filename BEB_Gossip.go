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

func BEBGossiper2(port int, round *int, isGossipList, changePList map[int]bool, pList, colored map[int]int, ch chan int, csvWriter *csv.Writer) {
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

	var (
		isColored bool = false // 是否着色
		isFirst   bool = true  // 是否首次收到消息
		firstMsg  Message
	)
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

		if isFirst {
			isFirst = false
			lockForChangePList.Lock()
			changePList[port] = false
			lockForChangePList.Unlock()
			lockForPList.Lock()
			pList[port] = 1
			lockForPList.Unlock()
			firstMsg = msg
			// 按周期传播
			go func(msg Message) {
				for { //阻塞等待下一轮屏障刷新
					select {
					case <-doneCh:
						return
					case <-waitCh:
						break
					}
					//fmt.Println("cross waitch:",port)
					//fmt.Println("isGossipList[",port,"]:", isGossipList[port])
					if !isGossipList[port] {
						continue
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

		// 全局时钟控制
		go func() {
			lockForColored.Lock()
			colored[port]++ //记录节点收到消息的次数
			lockForColored.Unlock()

			// 处理接受消息
			if isColored {
				lockForChangePList.Lock()
				changePList[port] = true
				lockForChangePList.Unlock()
			} else {
				isColored = true
			}
			//fmt.Println("reach barrier", port)
			_ = cyc.Await(context.Background()) //实现同步时钟模型，等待每轮所有消息均分发完毕才允许进入下一轮传播
			//fmt.Println("cross barrier", port)
			lockForwaitingNum.Lock()
			waitingNum++
			res := cycParties == waitingNum //检查是否当前轮次所有传播任务均完成
			lockForwaitingNum.Unlock()
			if res { //开启新的一轮传播，重置屏障
				sum := 0
				for k, v := range changePList {
					if !v {
						continue
					}
					if pList[k] < pThreshold {
						pList[k] *= 2
					}
					changePList[k] = false
				}
				for k, v := range pList {
					if v == 1 {
						isGossipList[k] = true
					} else if v > 1 {
						if pList[k] > 1 && 0 == (rand.Intn(pList[k])+1)/pList[k] {
							isGossipList[k] = false
							sum++
						} else {
							isGossipList[k] = true
						}
					}
				}
				*round++
				cycParties = len(colored) - sum // 计算下一轮次的总传播数
				//if cfg.Count-len(colored) < cfg.Minedges || (*round > 25 && cycParties < int(float32(cfg.Count)*0.1)) {
				if *round >= 23 {
					fmt.Printf("round:%d, cycParties:%d\n", *round, cycParties)
					printColoredMap(colored)
					printEdgeNodes(colored)
					printRepetitions(colored)
				} else {
					cyc.Reset()
					cyc = cyclicbarrier.New(cycParties)
					fmt.Printf("cyclicbarrier.New(cycParties:%d), round:%d\n", cycParties, *round)
					time.Sleep(100 * time.Millisecond)
					waitingNum = 0
					roundNums = 0
					close(waitCh)
					waitCh = make(chan struct{})
				}
			}
		}()
	}
}

func BEBGossiper(port int, round, notGossipSum *int, colored map[int]int, ch chan int) {
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

	var (
		p         int  = 1     // 初始概率为1
		isColored bool = false // 是否着色
	)
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
			var isGossip bool = true // 根据概率决定是否传播
			mutex.Lock()
			fmt.Printf("Data=%s, Round=%d, Path=%s, updnums=%d, roundnums=%d\n", msg.Data, msg.Round, msg.Path, udpNums, roundNums)
			udpNums++
			roundNums++
			colored[port]++ //记录节点收到消息的次数
			if isColored {
				if p < pThreshold {
					p *= 2
				}
				if 0 == (rand.Intn(p)+1)/p {
					isGossip = false
					*notGossipSum++
					fmt.Println("notGossipSum:", *notGossipSum)
					//printColoredMap(colored)
				}
			} else {
				isColored = true
			}
			mutex.Unlock()

			//fmt.Println("reach barrier")
			_ = cyc.Await(context.Background()) //实现同步时钟模型，等待每轮所有消息均分发完毕才允许进入下一轮传播
			//fmt.Println("cross barrier")
			//fmt.Println("ready to send to:", randNeighborList[j+1])
			lockForwaitingNum.Lock()
			waitingNum++
			res := cycParties == waitingNum //检查是否当前轮次所有传播任务均完成
			lockForwaitingNum.Unlock()

			if res { //开启新的一轮传播，重置屏障
				*round++
				cycParties = cycParties*cfg.Gossipfactor - *notGossipSum*cfg.Gossipfactor // 计算下一轮次的总传播数
				if cycParties <= 0 {
					fmt.Printf("cycParties:%d\n", cycParties)
					printColoredMap(colored)
					printEdgeNodes(colored)
				} else {
					fmt.Printf("cyclicbarrier.New(cycParties:%d), round:%d\n", cycParties, *round)
					cyc.Reset()
					cyc = cyclicbarrier.New(cycParties)
				}
				waitingNum = 0
				roundNums = 1
				*notGossipSum = 0
				close(waitCh)
				waitCh = make(chan struct{})
			} else { //阻塞等待下一轮屏障刷新
				select {
				case <-doneCh:
					return
				case <-waitCh:
					break
				}
			}

			if !isGossip {
				return
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
