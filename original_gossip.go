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
	"sync"
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

func PGA(port int, round *int, colored map[int]int, ch chan int, csvWriter *csv.Writer) {
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
		request     bool = false // 决定是否请求拉取
		response    bool = false // 是否收到了请求
		isColored   bool = false // 是否着色
		isFirst     bool = true  // 是否首次收到消息
		firstMsg    Message
		requestAddr net.UDPAddr
	)

	go func() {
		for !isColored {
			select {
			case <-doneCh:
				return
			case <-waitCh:
				break
			}
			if *round >= cfg.Pull {
				request = true
			}
			if request {
				var randNeighbor int //随机选择待分发的节点
				randNeighborSlice := rand.Perm(cfg.Count)[:2]
				if randNeighborSlice[0] != port {
					randNeighbor = cfg.Firstnode + randNeighborSlice[0]
				} else {
					randNeighbor = cfg.Firstnode + randNeighborSlice[1]
				}
				udpAddr := net.UDPAddr{
					IP:   ip,
					Port: randNeighbor,
				}
				pMsg := Message{Data: "request update", Round: *round}
				sendData, _ := json.Marshal(&pMsg)
				_, err = listen.WriteToUDP(sendData, &udpAddr) // 发送数据
				if err != nil {
					fmt.Println("Write to udp failed, err: ", err)
				}
				mutex.Lock()
				udpNums++
				roundNums++
				fmt.Printf("pull request:%v->%v, Data=%s, Round=%d, updnums=%d, roundnums=%d\n", port, udpAddr.Port, pMsg.Data, pMsg.Round, udpNums, roundNums)
				lockForColored.Lock()
				fmt.Println(udpAddr.Port, ":", colored[udpAddr.Port])
				lockForColored.Unlock()
				mutex.Unlock()
			}
		}
	}()

	for {
		var data [10 * 1024]byte
		n, addr, err := listen.ReadFromUDP(data[:]) // 接收数据
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

		if msg.Data == "gossip test" && isFirst {
			isFirst = false
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

					var udpAddr net.UDPAddr
					ch <- 1
					if response {
						lockForPullResponse.Lock()
						response = false
						lockForPullResponse.Unlock()
						udpAddr = requestAddr
						fmt.Printf("pull response:%v->%v\n", port, udpAddr.Port)
					} else {
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
						udpAddr = net.UDPAddr{
							IP:   ip,
							Port: randNeighbor,
						}
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
		go func(msg Message) {
			if msg.Data != "gossip test" && msg.Data != "request update" {
				return
			}
			if msg.Data == "request update" {
				lockForPullResponse.Lock()
				response = true
				lockForPullResponse.Unlock()
				requestAddr = *addr
				return
			}
			lockForColored.Lock()
			colored[port]++ //记录节点收到消息的次数
			lockForColored.Unlock()

			// 处理接受消息
			if !isColored {
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
				csvWriter.Write([]string{strconv.Itoa(*round), strconv.Itoa(udpNums), strconv.Itoa(roundNums), strconv.Itoa(len(colored))})
				csvWriter.Flush()
				*round++
				cycParties = len(colored) // 计算下一轮次的总传播数
				if *round > cfg.Roundmax && cycParties < int(float32(cfg.Count)*0.1) {
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
		}(msg)
	}
}

func NGA(port int, round *int, colored map[int]int, ch chan int, csvWriter *csv.Writer) {
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
		isFirst  bool = true  // 是否首次收到消息
		hasPush  bool = false // 是否推送过消息
		firstMsg Message
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

					ch <- 1
					var randNeighbor int                //随机选择待分发的节点
					if !hasPush && *round >= cfg.Push { // 首次收到消息的节点将转发给前一个邻居节点以避免边缘节点
						if port == cfg.Firstnode {
							randNeighbor = port + cfg.Count - 1
						} else {
							randNeighbor = port - 1
						}
						fmt.Println(port, " push to ", randNeighbor)
					} else {
						randNeighborSlice := rand.Perm(cfg.Count)[:2]
						if randNeighborSlice[0] != port {
							randNeighbor = cfg.Firstnode + randNeighborSlice[0]
						} else {
							randNeighbor = cfg.Firstnode + randNeighborSlice[1]
						}
					}
					if (port == cfg.Firstnode && randNeighbor == port+cfg.Count-1) || randNeighbor == port-1 {
						hasPush = true
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
				if *round > cfg.Roundmax && cycParties < int(float32(cfg.Count)*0.1) {
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

func MGA(port int, round *int, colored map[int]int, ch chan int, csvWriter *csv.Writer) {
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
	var lockForHistoryNodes sync.Mutex
	historyNodes := make(map[int]int) // 历史节点列表
	historyNodes[port] = 1
	for {
		var data [10 * 1024 * 10]byte
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
			lockForHistoryNodes.Lock()
			for k, v := range msg.HistoryNodes {
				_, ok := historyNodes[k]
				if !ok {
					historyNodes[k] = v
				}
			}
			lockForHistoryNodes.Unlock()
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
			//if msg.HistoryNodes == nil {
			//	msg.HistoryNodes = make(map[int]int)
			//}
			//msg.HistoryNodes[port] = 1
			//fmt.Println(msg.HistoryNodes)
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
					//fmt.Println(msg.HistoryNodes)
					lockForHistoryNodes.Lock()
					randNeighbor = selectRandNeighborWithMemory(historyNodes, len(historyNodes))
					historyNodes[randNeighbor] = 1
					//for k, v := range historyNodes {
					//	_, ok := msg.HistoryNodes[k]
					//	if !ok {
					//		msg.HistoryNodes[k] = v
					//	}
					//}
					lockForHistoryNodes.Unlock()

					select {
					case <-doneCh:
						return
					default:
					}
					udpAddr := net.UDPAddr{
						IP:   ip,
						Port: randNeighbor,
					}
					lockForHistoryNodes.Lock()
					fmt.Println("historyNodes length:", len(historyNodes))
					pMsg := Message{Data: msg.Data, Round: *round, Path: msg.Path /*strconv.Itoa(port)*/ + "->" + strconv.Itoa(udpAddr.Port), HistoryNodes: historyNodes}
					sendData, _ := json.Marshal(&pMsg)
					lockForHistoryNodes.Unlock()
					//fmt.Println(pMsg.HistoryNodes)
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

//func MPGA(port int, round *int, colored map[int]int, ch chan int, csvWriter *csv.Writer) {
//
//}
//
//func MNGA(port int, round *int, colored map[int]int, ch chan int, csvWriter *csv.Writer) {
//
//}
//
//func originalGossiper(port int, round *int, colored map[int]int, ch chan int) {
//	ip := net.ParseIP(localhost)
//	listen, err := net.ListenUDP("udp", &net.UDPAddr{
//		IP:   ip,
//		Port: port,
//	})
//	if err != nil {
//		fmt.Println("Listen failed, err: ", err)
//		return
//	}
//	defer func(listen *net.UDPConn) {
//		err := listen.Close()
//		if err != nil {
//			panic("❌")
//		}
//	}(listen)
//
//	fmt.Println("[", ip, ":", port, "]", "start listening")
//
//	for {
//		var data [10 * 1024]byte
//		n, _, err := listen.ReadFromUDP(data[:]) // 接收数据
//		select {
//		case <-doneCh:
//			return
//		default:
//		}
//		if err != nil {
//			fmt.Println("read udp failed, err: ", err)
//			continue
//		}
//
//		//fmt.Printf("data:%v addr:%v count:%v\n", string(data[:n]), addr, n)
//		var msg Message
//		err = json.Unmarshal(data[:n], &msg) //反序列化json保存到Message结构体中
//		if err != nil {
//			fmt.Println("err: ", err)
//			continue
//		}
//
//		go func(msg Message) {
//			mutex.Lock()
//			fmt.Printf("Data=%s, Round=%d, Path=%s, updnums=%d, roundnums=%d\n", msg.Data, msg.Round, msg.Path, udpNums, roundNums)
//			udpNums++
//			roundNums++
//			colored[port]++ //记录节点收到消息的次数
//			mutex.Unlock()
//
//			_ = cyc.Await(context.Background()) //实现同步时钟模型，等待每轮所有消息均分发完毕才允许进入下一轮传播
//			lockForwaitingNum.Lock()
//			waitingNum++
//			res := cycParties == waitingNum //检查是否当前轮次所有传播任务均完成
//			lockForwaitingNum.Unlock()
//			if res { //开启新的一轮传播，重置屏障
//				*round++
//				cycParties *= cfg.Gossipfactor // 计算下一轮次的总传播数
//				cyc.Reset()
//				cyc = cyclicbarrier.New(cycParties)
//				close(waitCh)
//				waitCh = make(chan struct{})
//				waitingNum = 0
//				roundNums = 1
//				fmt.Printf("cyclicbarrier.New(cycParties:%d), round:%d\n", cycParties, *round)
//			} else { //阻塞等待下一轮屏障刷新
//				select {
//				case <-doneCh:
//					return
//				case <-waitCh:
//					break
//				}
//			}
//
//			randNeighborList := make([]int, cfg.Gossipfactor) //随机选择待分发的节点
//			var k int = 0
//			for _, value := range rand.Perm(cfg.Count)[:cfg.Gossipfactor+1] {
//				randPort := value + cfg.Firstnode
//				if port != randPort {
//					randNeighborList[k] = randPort
//					k++
//				}
//				if k == cfg.Gossipfactor {
//					break
//				}
//			}
//
//			for i := 0; i < cfg.Gossipfactor; i++ {
//				select {
//				case <-doneCh:
//					return
//				default:
//				}
//				go func(j int) {
//					ch <- 1
//					udpAddr := net.UDPAddr{
//						IP:   ip,
//						Port: randNeighborList[j],
//					}
//					pMsg := Message{Data: msg.Data, Round: msg.Round + 1, Path: msg.Path + "->" + strconv.Itoa(udpAddr.Port)}
//					sendData, _ := json.Marshal(&pMsg)
//
//					select {
//					case <-doneCh:
//						return
//					default:
//					}
//					_, err = listen.WriteToUDP(sendData, &udpAddr) // 发送数据
//					if err != nil {
//						fmt.Println("Write to udp failed, err: ", err)
//					}
//					time.Sleep(100 * time.Millisecond)
//					<-ch
//				}(i)
//			}
//		}(msg)
//
//	}
//}
