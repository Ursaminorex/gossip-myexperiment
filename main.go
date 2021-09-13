package main

import (
	"encoding/csv"
	"fmt"
	"github.com/marusama/cyclicbarrier"
	"math/rand"
	"os"
	"path"
	"strconv"
	"sync"
	"time"
)

const localhost = "127.0.0.1"
const pThreshold = 32 //BEBG中p的概率阈值

var (
	cfg Config
	cyc cyclicbarrier.CyclicBarrier //控制轮次的屏障
	//***********使用channel也可实现互斥同步，channel更加go一些**********************
	mutex                   sync.Mutex //递增变量互斥锁
	lockForColored          sync.Mutex //着色map的互斥锁
	lockForwaitingNum       sync.Mutex //信号计数的互斥锁
	lockForPList            sync.Mutex //传播概率map的互斥锁
	lockForChangePList      sync.Mutex //传播概率map的互斥锁
	lockForHasPushList      sync.Mutex //是否已推送map的互斥锁
	lockForPullResponseList sync.Mutex //是否回复更新map的互斥锁
	//**************************************************************************
	waitingNum     int           = 0 //到达屏障的线程个数
	udpNums        int           = 0 //udp数据包总量
	roundNums      int           = 0 //每轮的udp包的数量
	cycParties     int           = 0 //屏障个数
	waitCh, doneCh chan struct{}     //实现协程间同步关系的空通道
)

func main() {
	cfg = cfg.LoadConfig("config.json")
	csvName := "test.csv"
	if 4 == cfg.Gossip {
		csvName = "GA_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 5 == cfg.Gossip {
		csvName = "BEB_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 6 == cfg.Gossip {
		csvName = "PBEB_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 7 == cfg.Gossip {
		csvName = "NBEB_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else {
		csvName = "gossip_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	}
	f := initCsv(csvName)
	defer f.Close()
	csvWriter := csv.NewWriter(f) //创建一个新的写入文件流
	rand.Seed(time.Now().Unix())
	var port int
	colored := make(map[int]int)
	round := 0
	ch := make(chan int, cfg.Chsize) //设置缓冲区，限制单位时间线程运算的数量以缓解cpu
	cycParties = 1
	cyc = cyclicbarrier.New(cycParties) //初始化屏障
	waitCh = make(chan struct{})
	doneCh = make(chan struct{}) //由上至下的使子协程退出方法考虑使用context比done channel更好

	var notGossipSum int = 0
	pList := make(map[int]int)
	changePList := make(map[int]bool)      //是否要改变概率p
	isGossipList := make(map[int]bool)     //是否传播
	hasPushList := make(map[int]bool)      //是否已经推送给前一个邻居过
	pullResponseList := make(map[int]bool) //是否回复更新消息
	//初始化maps
	for i := cfg.Firstnode; i < cfg.Firstnode+cfg.Count; i++ {
		pList[i] = 0
		changePList[i] = false
		isGossipList[i] = false
		hasPushList[i] = false
		pullResponseList[i] = false
	}
	//go协程模拟p2p节点
	for i := 0; i < cfg.Count; i++ {
		port = cfg.Firstnode + i
		if 1 == cfg.Gossip {
			go BEBGossiper(port, &round, &notGossipSum, colored, ch)
		} else if 2 == cfg.Gossip {
			go PBEBGossiper(port, &round, &notGossipSum, colored, ch)
		} else if 3 == cfg.Gossip {
			go NBEBGossiper(port, &round, &notGossipSum, colored, ch)
		} else if 4 == cfg.Gossip {
			go originalGossiper2(port, &round, colored, ch, csvWriter)
		} else if 5 == cfg.Gossip {
			go BEBGossiper2(port, &round, isGossipList, changePList, pList, colored, ch, csvWriter)
		} else if 6 == cfg.Gossip {
			go PBEBGossiper2(port, &round, isGossipList, changePList, pullResponseList, pList, colored, ch, csvWriter)
		} else if 7 == cfg.Gossip {
			go NBEBGossiper2(port, &round, isGossipList, changePList, hasPushList, pList, colored, ch, csvWriter)
		} else {
			go originalGossiper(port, &round, colored, ch)
		}
	}

	for len(colored) < cfg.Count {
	} //所有节点均着色后立即中断传播
	close(doneCh) //发送子协程退出信号
	time.Sleep(100 * time.Millisecond)

	//输出着色情况
	printColoredMap(colored)
	printRepetitions(colored)
	csvWriter.Write([]string{strconv.Itoa(round), strconv.Itoa(udpNums), strconv.Itoa(roundNums), strconv.Itoa(len(colored))})
	csvWriter.Flush()
}

func initCsv(filename string) *os.File {
	var f *os.File
	csvDir := "csv"
	_, err := os.Stat(csvDir)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(csvDir, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
	}
	csvPath := path.Join(csvDir, filename)
	_, err = os.Stat(csvPath)
	if err != nil {
		if os.IsNotExist(err) {
			f, err = os.Create(csvPath)
			if err != nil {
				panic(err)
			}
			f.WriteString("\xEF\xBB\xBF")             // 写入UTF-8 BOM,防止中文乱码
			f.WriteString("周期,网络数据总量,每轮数据量,已着色节点数\n") // 写入UTF-8 BOM,防止中文乱码
			f.Close()
		}
	}
	f, err = os.OpenFile(csvPath, os.O_WRONLY|os.O_APPEND, os.ModeAppend|os.ModePerm) //OpenFile读取文件，不存在时则创建，使用追加模式
	if err != nil {
		panic(err)
	}
	return f
}

func printColoredMap(colored map[int]int) {
	const rowCount = 10
	for i := 0; i < 9*rowCount; i++ {
		fmt.Print("-")
	}
	fmt.Println()
	i := 0
	for k := cfg.Firstnode; k < cfg.Firstnode+cfg.Count; k++ {
		i++
		fmt.Printf("%d:%-2d", k, colored[k])
		fmt.Print("|")
		if 0 == i%rowCount {
			fmt.Println()
		}
	}
	for i := 0; i < 9*rowCount; i++ {
		fmt.Print("-")
	}
	fmt.Println()
}

func printEdgeNodes(colored map[int]int) {
	const rowCount = 10
	for i := 0; i < 9*rowCount; i++ {
		fmt.Print("-")
	}
	fmt.Println()
	n := 0
	for k := cfg.Firstnode; k < cfg.Firstnode+cfg.Count; k++ {
		if 0 == colored[k] {
			n++
			fmt.Printf("%d:%-2d", k, colored[k])
			fmt.Print("|")
			if 0 == n%rowCount {
				fmt.Println()
			}
		}
	}
	fmt.Println()
	for i := 0; i < 9*rowCount; i++ {
		fmt.Print("-")
	}
	fmt.Println()
}

func printRepetitions(colored map[int]int) {
	repeatMsgSum := 0
	for _, v := range colored {
		if v > 1 {
			repeatMsgSum += v - 1
		}
	}
	fmt.Println("Repetitions:", repeatMsgSum)
}
