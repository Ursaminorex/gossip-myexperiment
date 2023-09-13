package main

import (
	"encoding/csv"
	"fmt"
	"github.com/marusama/cyclicbarrier"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const localhost = "127.0.0.1"
const pThreshold = 32 //BEBG中p的概率阈值

var (
	cfg Config
	cyc cyclicbarrier.CyclicBarrier //控制轮次的屏障
	//***********使用channel也可实现互斥同步，channel更加go一些**********************
	mutex               sync.Mutex //递增变量互斥锁
	lockForColored      sync.Mutex //着色map的互斥锁
	lockForwaitingNum   sync.Mutex //信号计数的互斥锁
	lockForPList        sync.Mutex //传播概率map的互斥锁
	lockForChangePList  sync.Mutex //传播概率map的互斥锁
	lockForHasPushList  sync.Mutex //是否已推送map的互斥锁
	lockForPullResponse sync.Mutex //是否回复更新的互斥锁
	lockForFanout       sync.Mutex //因子衰减互斥锁
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
	if 1 == cfg.Gossip {
		csvName = "GA_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 2 == cfg.Gossip {
		csvName = "BEBG_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 3 == cfg.Gossip {
		csvName = "PBEBG_" + strconv.Itoa(cfg.Count) + "_" + strconv.Itoa(cfg.Pull) + "nodes.csv"
	} else if 4 == cfg.Gossip {
		csvName = "NBEBG_" + strconv.Itoa(cfg.Count) + "_" + strconv.Itoa(cfg.Push) + "nodes.csv"
	} else if 5 == cfg.Gossip {
		csvName = "PGA_" + strconv.Itoa(cfg.Count) + "_" + strconv.Itoa(cfg.Pull) + "nodes.csv"
	} else if 6 == cfg.Gossip {
		csvName = "NGA_" + strconv.Itoa(cfg.Count) + "_" + strconv.Itoa(cfg.Push) + "nodes.csv"
	} else if 7 == cfg.Gossip {
		csvName = "MGA_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 8 == cfg.Gossip {
		csvName = "MBEBG_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 9 == cfg.Gossip {
		csvName = "MPBEBG_" + strconv.Itoa(cfg.Count) + "_" + strconv.Itoa(cfg.Pull) + "nodes.csv"
	} else if 10 == cfg.Gossip {
		csvName = "MNBEBG_" + strconv.Itoa(cfg.Count) + "_" + strconv.Itoa(cfg.Push) + "nodes.csv"
		//} else if 11 == cfg.Gossip {
		//	csvName = "MPGA_" + strconv.Itoa(cfg.Count) + "_" + strconv.Itoa(cfg.Pull) + "nodes.csv"
		//} else if 12 == cfg.Gossip {
		//	csvName = "MNGA_" + strconv.Itoa(cfg.Count) + "_" + strconv.Itoa(cfg.Push) + "nodes.csv"
	} else if 11 == cfg.Gossip {
		csvName = "originalGossiper_" + strconv.Itoa(cfg.Gossipfactor) + "fout_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 12 == cfg.Gossip {
		csvName = "NHDG_" + strconv.Itoa(cfg.Gossipfactor) + "fout_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 13 == cfg.Gossip {
		csvName = "BEBGossiper_" + strconv.Itoa(cfg.Gossipfactor) + "fout_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 21 == cfg.Gossip {
		csvName = "asyngossip_" + strconv.Itoa(cfg.Gossipfactor) + "fout_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	} else if 22 == cfg.Gossip {
		csvName = "asynNHDG_" + strconv.Itoa(cfg.Gossipfactor) + "fout_" + strconv.Itoa(cfg.Count) + "nodes.csv"
	}
	var f *os.File
	if cfg.Gossip >= 20 {
		f = initAsynCsv(csvName)
	} else {
		f = initSynCsv(csvName)
	}
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
	fanoutList := make(map[int]int)        //fanout列表
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
			go GA(port, &round, colored, ch, csvWriter)
		} else if 2 == cfg.Gossip {
			go BEBG(port, &round, isGossipList, changePList, pList, colored, ch, csvWriter)
		} else if 3 == cfg.Gossip {
			go PBEBG(port, &round, isGossipList, changePList, pullResponseList, pList, colored, ch, csvWriter)
		} else if 4 == cfg.Gossip {
			go NBEBG(port, &round, isGossipList, changePList, hasPushList, pList, colored, ch, csvWriter)
		} else if 5 == cfg.Gossip {
			go PGA(port, &round, colored, ch, csvWriter)
		} else if 6 == cfg.Gossip {
			go NGA(port, &round, colored, ch, csvWriter)
		} else if 7 == cfg.Gossip {
			go MGA(port, &round, colored, ch, csvWriter)
		} else if 8 == cfg.Gossip {
			go MBEBG(port, &round, isGossipList, changePList, pList, colored, ch, csvWriter)
		} else if 9 == cfg.Gossip {
			go MPBEBG(port, &round, isGossipList, changePList, pullResponseList, pList, colored, ch, csvWriter)
		} else if 10 == cfg.Gossip {
			go MNBEBG(port, &round, isGossipList, changePList, hasPushList, pList, colored, ch, csvWriter)
			//} else if 11 == cfg.Gossip {
			//	go MPGA(port, &round, colored, ch, csvWriter)
			//} else if 12 == cfg.Gossip {
			//	go MNGA(port, &round, colored, ch, csvWriter)
			//} else if 13 == cfg.Gossip {
			//	go BEBGossiper(port, &round, &notGossipSum, colored, ch)
			//} else if 14 == cfg.Gossip {
			//	go PBEBGossiper(port, &round, &notGossipSum, colored, ch)
			//} else if 15 == cfg.Gossip {
			//	go NBEBGossiper(port, &round, &notGossipSum, colored, ch)
		} else if 11 == cfg.Gossip {
			go originalGossiper(port, &round, colored, ch, csvWriter)
		} else if 12 == cfg.Gossip {
			go NHDG(port, &round, fanoutList, colored, ch, csvWriter)
		} else if 13 == cfg.Gossip {
			go BEBGossiper(port, &round, &notGossipSum, colored, ch, csvWriter)
		} else if 21 == cfg.Gossip {
			go asynOriginalGossiper(port, colored, ch)
		} else if 22 == cfg.Gossip {
			go asynNHDG(port, colored, ch)
		}
	}

	var elapsed = time.Nanosecond
	if cfg.Gossip > 20 {
		go func() {
			<-waitCh
			tick := time.NewTicker(1 * time.Second)
			t := time.Now()
			n := 1
			for {
				select {
				case <-doneCh:
					tick.Stop()
					elapsed = time.Since(t)
					return
				case <-tick.C:
					//耗时,网络数据总量,已着色节点数，冗余消息数,覆盖率
					row := fmt.Sprintf("%d,%d,%d,%.4f,%.4f", n, udpNums, len(colored), float32(udpNums-len(colored))/float32(cfg.Count), float32(len(colored))/float32(cfg.Count))
					csvWriter.Write(strings.Split(row, ","))
					csvWriter.Flush()
					n += 1
				}
			}
		}()
	} else {
		go func() {
			for {
				select {
				case <-doneCh:
					return
				default:
					if float32(len(colored))/float32(cfg.Count) >= 0.9 {
						row := fmt.Sprintf("%d,%d,%d,%d,%.4f,%.4f", round, udpNums-1, roundNums-1, len(colored), float32(udpNums-len(colored))/float32(cfg.Count), float32(len(colored))/float32(cfg.Count))
						csvWriter.Write(strings.Split(row, ","))
						csvWriter.Flush()
						return
					}
				}
			}
		}()
	}

	<-doneCh
	//for len(colored) < cfg.Count {
	//} //所有节点均着色后立即中断传播
	////输出着色情况
	//close(doneCh) //发送子协程退出信号
	//printColoredMap(colored)
	//printRepetitions(colored)
	//time.Sleep(100 * time.Millisecond)

	//if cfg.Gossip > 20 {
	//	fmt.Println("总耗时： ", elapsed)
	//}
	//if cfg.Gossip < 20 {
	//	row := fmt.Sprintf("%d,%d,%d,%d,%.4f,%.4f", round, udpNums, roundNums, len(colored), float32(udpNums-len(colored))/float32(cfg.Count), float32(len(colored))/float32(cfg.Count))
	//	csvWriter.Write(strings.Split(row, ","))
	//	csvWriter.Flush()
	//} else {
	//	row := fmt.Sprintf("%dms,%d,%d,%.4f,%.4f", elapsed.Milliseconds(), udpNums, len(colored), float32(udpNums-len(colored))/float32(cfg.Count), float32(len(colored))/float32(cfg.Count))
	//	csvWriter.Write(strings.Split(row, ","))
	//	csvWriter.Flush()
	//}
}

func initSynCsv(filename string) *os.File {
	var f *os.File
	csvDir := "csv"

	if 100 == cfg.Count {
		csvDir += "/100nodes"
	} else if 1000 == cfg.Count {
		csvDir += "/1000nodes"
	} else if 10000 == cfg.Count {
		csvDir += "/10000nodes"
	}

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
			f.WriteString("\xEF\xBB\xBF") // 写入UTF-8 BOM,防止中文乱码
			f.WriteString("周期,网络数据总量,每轮数据量,已着色节点数,冗余消息数,覆盖率\n")
			f.Close()
		}
	}
	f, err = os.OpenFile(csvPath, os.O_WRONLY|os.O_APPEND, os.ModeAppend|os.ModePerm) //OpenFile打开文件，使用追加写入模式
	if err != nil {
		panic(err)
	}
	return f
}

func initAsynCsv(filename string) *os.File {
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
			f.WriteString("\xEF\xBB\xBF") // 写入UTF-8 BOM,防止中文乱码
			f.WriteString("耗时(秒),网络数据总量,已着色节点数,冗余消息数,覆盖率\n")
			f.Close()
		}
	}
	f, err = os.OpenFile(csvPath, os.O_WRONLY|os.O_APPEND, os.ModeAppend|os.ModePerm) //OpenFile打开文件，使用追加写入模式
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

func selectRandNeighborWithMemory(historyNodes map[int]int, count int) int {
	var selectNeighbor int
	for _, value := range rand.Perm(cfg.Count)[:count+1] {
		randPort := value + cfg.Firstnode
		_, ok := historyNodes[randPort]
		if ok {
			continue
		}
		selectNeighbor = randPort
		break
	}
	return selectNeighbor
}
