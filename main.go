package main

import (
	"fmt"
	"github.com/marusama/cyclicbarrier"
	"math/rand"
	"sync"
	"time"
)

var cfg Config
var cyc cyclicbarrier.CyclicBarrier //控制轮次的屏障
var signal int = 0                  //刷新屏障的信号量
var lockForColored sync.Mutex       //已着色节点map的互斥锁
var lockForUdpNums sync.Mutex       //记录udp包总数的互斥锁
var lockForSignal sync.Mutex        //信号计数的互斥锁
var udpNums int = 0                 //udp数据包总量
var roundNums int = 0               //每轮的udp包的数量
var waitCh, doneCh chan struct{}    //实现协程间同步关系的空通道

func main() {
	cfg = cfg.LoadConfig("config.json")
	rand.Seed(time.Now().Unix())
	var port int
	colored := make(map[int]int)
	round := 0
	ch := make(chan int, cfg.Chsize)          //设置缓冲区，限制单位时间线程运算的数量以缓解cpu
	cyc = cyclicbarrier.New(cfg.Gossipfactor) //初始化屏障，首轮任务数为gossip=3
	waitCh = make(chan struct{})
	doneCh = make(chan struct{})

	//多线程模拟p2p节点
	for i := 0; i < cfg.Count; i++ {
		port = cfg.Firstnode + i
		go originalGossiper(port, &round, colored, ch)
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
