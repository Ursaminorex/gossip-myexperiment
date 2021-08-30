package main

import (
	"fmt"
	"github.com/marusama/cyclicbarrier"
	"math/rand"
	"sync"
	"time"
)

const localhost = "127.0.0.1"

var (
	cfg               Config
	cyc               cyclicbarrier.CyclicBarrier     //控制轮次的屏障
	lockForColored    sync.Mutex                      //已着色节点map的互斥锁
	lockForUdpNums    sync.Mutex                      //记录udp包总数的互斥锁
	lockForRoundNums  sync.Mutex                      //记录每轮udp包总数的互斥锁
	lockForwaitingNum sync.Mutex                      //信号计数的互斥锁
	waitingNum        int                         = 0 //到达屏障的线程个数
	udpNums           int                         = 0 //udp数据包总量
	roundNums         int                         = 0 //每轮的udp包的数量
	cycParties        int                         = 0 //屏障个数
	waitCh, doneCh    chan struct{}                   //实现协程间同步关系的空通道
)

func main() {
	cfg = cfg.LoadConfig("config.json")
	rand.Seed(time.Now().Unix())
	var port int
	colored := make(map[int]int)
	round := 1
	ch := make(chan int, cfg.Chsize) //设置缓冲区，限制单位时间线程运算的数量以缓解cpu
	cycParties = cfg.Gossipfactor
	cyc = cyclicbarrier.New(cycParties) //初始化屏障
	waitCh = make(chan struct{})
	doneCh = make(chan struct{})

	var notGossipSum int = 0
	//多线程模拟p2p节点
	for i := 0; i < cfg.Count; i++ {
		port = cfg.Firstnode + i
		if 1 == cfg.Gossip {
			go BEBGossiper(port, &round, &notGossipSum, colored, ch)
		} else {
			go originalGossiper(port, &round, colored, ch)
		}
	}

	for len(colored) < cfg.Count {
	} //所有节点均着色后立即中断传播
	close(doneCh) //发送子协程退出信号
	time.Sleep(100 * time.Millisecond)

	//输出着色情况
	rowCount := 10
	i := 0
	for k := cfg.Firstnode; k < cfg.Firstnode+cfg.Count; k++ {
		i++
		fmt.Printf("%d:%-2d", k, colored[k])
		fmt.Print("|")
		if 0 == i%rowCount {
			fmt.Println()
		}
	}
}
