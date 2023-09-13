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
	"strings"
	"sync"
	"time"
)

func asynNHDG(port int, colored map[int]int, ch chan int) {
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

	isFirst := true
	isColored := false
	fanout := cfg.Gossipfactor
	var lockForFanout sync.Mutex //因子衰减互斥锁
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

		if isColored {
			lockForFanout.Lock()
			if fanout > 0 {
				fanout--
			}
			lockForFanout.Unlock()
		} else {
			isColored = true
		}

		if port == cfg.Firstnode && isFirst {
			isFirst = false
			close(waitCh)
			time.Sleep(10 * time.Millisecond)
		}

		go func(msg Message) {
			mutex.Lock()
			fmt.Printf("fanout=%d, Data=%s, Path=%s, updnums=%d, coverage=%.2f%%\n", fanout, msg.Data, msg.Path, udpNums, float32(len(colored))/float32(cfg.Count/100))
			udpNums++
			colored[port]++ //记录节点收到消息的次数
			mutex.Unlock()

			if fanout <= 0 {
				return
			}
			randNeighborList := make([]int, fanout) //随机选择待分发的节点
			var k = 0
			for _, value := range rand.Perm(cfg.Count)[:fanout+1] {
				randPort := value + cfg.Firstnode
				if port != randPort {
					randNeighborList[k] = randPort
					k++
				}
				if k == fanout {
					break
				}
			}

			for i := 0; i < fanout; i++ {
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
					pMsg := Message{Data: msg.Data, Path: msg.Path + "->" + strconv.Itoa(udpAddr.Port)}
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

func NHDG(port int, round *int, fanoutList, colored map[int]int, ch chan int, csvWriter *csv.Writer) {
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

	isColored := false
	fanout := cfg.Gossipfactor
	//var lockForFanoutList sync.Mutex //因子衰减列表互斥锁
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
			var fout int
			lockForFanout.Lock()
			if isColored {
				if fanout > 0 {
					fanout--
				}
			} else {
				isColored = true
			}
			fout = fanout
			fanoutList[port] += fanout
			lockForFanout.Unlock()
			mutex.Lock()
			fmt.Printf("fanout=%d, Data=%s, Round=%d, Path=%s, updnums=%d, roundnums=%d, coverage=%.2f%%\n", fout, msg.Data, msg.Round, msg.Path, udpNums, roundNums, float32(len(colored))/float32(cfg.Count/100))
			udpNums++
			roundNums++
			colored[port]++ //记录节点收到消息的次数
			//row := fmt.Sprintf("%d,%d,%d,%d,%.4f,%.4f", *round, udpNums-1, roundNums-1, len(colored), float32(udpNums-len(colored))/float32(cfg.Count), float32(len(colored))/float32(cfg.Count))
			//csvWriter.Write(strings.Split(row, ","))
			//csvWriter.Flush()
			mutex.Unlock()

			_ = cyc.Await(context.Background()) //实现同步时钟模型，等待每轮所有消息均分发完毕才允许进入下一轮传播
			lockForwaitingNum.Lock()
			waitingNum++
			res := cycParties == waitingNum //检查是否当前轮次所有传播任务均完成
			lockForwaitingNum.Unlock()
			if res { //开启新的一轮传播，重置屏障
				sumFanout := 0
				for k, v := range fanoutList {
					if v > 0 {
						sumFanout += v
					}
					fanoutList[k] = 0
				}

				row := fmt.Sprintf("%d,%d,%d,%d,%.4f,%.4f", *round, udpNums-1, roundNums-1, len(colored), float32(udpNums-len(colored))/float32(cfg.Count), float32(len(colored))/float32(cfg.Count))
				csvWriter.Write(strings.Split(row, ","))
				csvWriter.Flush()
				*round++
				cycParties = sumFanout // 计算下一轮次的总传播数
				if cycParties <= 0 {
					printColoredMap(colored)
					printEdgeNodes(colored)
					//row := fmt.Sprintf("%d,%d,%d,%d,%.4f,%.4f", *round, udpNums-1, roundNums-1, len(colored), float32(udpNums-len(colored))/float32(cfg.Count), float32(len(colored))/float32(cfg.Count))
					//csvWriter.Write(strings.Split(row, ","))
					//csvWriter.Flush()
					close(doneCh) //发送子协程退出信号
					time.Sleep(100 * time.Millisecond)
					return
				}
				if *round >= cfg.Roundmax {
					fmt.Printf("round:%d, cycParties:%d\n", *round, cycParties)
					printColoredMap(colored)
					printEdgeNodes(colored)
					printRepetitions(colored)
				} else {
					cyc.Reset()
					cyc = cyclicbarrier.New(cycParties)
					close(waitCh)
					waitCh = make(chan struct{})
					waitingNum = 0
					roundNums = 1
					fmt.Printf("cyclicbarrier.New(cycParties:%d), round:%d\n", cycParties, *round)
				}
			} else { //阻塞等待下一轮屏障刷新
				select {
				case <-doneCh:
					return
				case <-waitCh:
					break
				}
			}

			if fout <= 0 {
				return
			}
			randNeighborList := make([]int, fout) //随机选择待分发的节点
			var k = 0
			for _, value := range rand.Perm(cfg.Count)[:fout+1] {
				randPort := value + cfg.Firstnode
				if port != randPort {
					randNeighborList[k] = randPort
					k++
				}
				if k == fout {
					break
				}
			}

			for i := 0; i < fout; i++ {
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
