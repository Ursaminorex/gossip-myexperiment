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

const count int = 100       //节点数量
const startPort int = 30000 //起始端口
const gossip int = 3        //八卦因子

var cyc cyclicbarrier.CyclicBarrier //控制轮次的屏障
var signal int = 0                  //刷新屏障的信号量
var lockForColored sync.Mutex       //已着色节点map的互斥锁
var lockForUdpNums sync.Mutex       //记录udp包总数的互斥锁
var lockForSignal sync.Mutex        //信号计数的互斥锁

var udpNums int = 0              //udp数据包总量
var roundNums int = 0            //每轮的udp包的数量
var waitCh, doneCh chan struct{} //实现协程间同步关系的空通道

func main() {
	rand.Seed(time.Now().Unix())
	var port int
	colored := make(map[int]int)
	round := 0
	ch := make(chan int, 3)         //设置缓冲区，限制单位时间线程运算的数量以缓解cpu
	cyc = cyclicbarrier.New(gossip) //初始化屏障，首轮任务数为gossip=3
	waitCh = make(chan struct{})
	doneCh = make(chan struct{})
	//defer close(ch)

	//多线程模拟p2p节点
	for i := 0; i < count; i++ {
		port = startPort + i
		go gossipListener(port, &round, colored, ch)
	}

	for len(colored) < count { //所有节点均着色后立即中断传播
	}
	close(doneCh) //发送子协程退出信号
	time.Sleep(100 * time.Millisecond)

	//检查着色情况
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
		} else {
			lockForUdpNums.Lock()
			udpNums++
			roundNums++
			fmt.Printf("Data=%s, Round=%d, Path=%s, updnums=%d, roundnums=%d\n", msg.Data, msg.Round, msg.Path, udpNums, roundNums)
			lockForUdpNums.Unlock()
		}

		lockForColored.Lock()
		colored[port]++ //记录节点在最终一致后收到数据的次数
		//if 0 == colored[port] {
		//	colored[port] = 1
		//} else {
		//	colored[port]++
		//}
		lockForColored.Unlock()

		var historyNodeList [gossip]int //随机选择待分发的节点
		var k int = 0
		for _, value := range rand.Perm(count)[:gossip] {
			randPort := value + startPort
			if port != randPort {
				historyNodeList[k] = randPort
				k++
			}
			if k == gossip {
				break
			}
		}

		for i := 0; i < gossip; i++ {
			go func(j int) {
				select {
				case <-doneCh:
					return
				default:
				}
				_ = cyc.Await(context.Background()) //实现同步时钟模型，等待每轮所有消息均分发完毕才允许进入下一轮传播
				//fmt.Println("ready to send to:", historyNodeList[j+1])
				lockForSignal.Lock()
				signal++
				maxWaitingNum := math.Pow(float64(gossip), float64(*round+1)) //计算每轮次的传播数
				res := int(maxWaitingNum) == signal                           //检查是否当前轮次所有传播任务均完成
				lockForSignal.Unlock()
				if res { //开启新的一轮传播，重置屏障
					*round++
					maxWaitingNum = math.Pow(float64(gossip), float64(*round+1))
					cyc.Reset()
					cyc = cyclicbarrier.New(int(maxWaitingNum))
					close(waitCh)
					waitCh = make(chan struct{})
					signal = 0
					fmt.Printf("cyclicbarrier.New(maxWaitingNum:%d), round:%d\n", int(maxWaitingNum), *round+1)
				} else { //阻塞等待下一轮屏障刷新
					select {
					case <-waitCh:
						break
					}
				}
				if 0 != roundNums {
					roundNums = 0
				}

				ch <- 1
				udpAddr := net.UDPAddr{
					IP:   ip,
					Port: historyNodeList[j],
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
