package main

import (
	"encoding/json"
	"os"
)

type Message struct {
	Data         string      // 消息内容
	Round        int         // 轮次
	Path         string      // 单源路径
	HistoryNodes map[int]int // 历史节点列表
}

type Config struct {
	Count        int // 节点数量
	Firstnode    int // 起始端口
	Gossipfactor int // 八卦因子
	Chsize       int // 限制单位时间线程运算的数量以缓解cpu
	Gossip       int // gossip算法类型，0是original gossip，1是BEBG，2是PBEBG，3是NBEBG
	Minedges     int // 边缘节点数最小值
	Push         int // 推送给前一个邻居的轮次阈值
	Pull         int // 向其他节点发送拉取信息请求的轮次
	Roundmax     int // 最大轮次阈值
}

func (c Config) LoadConfig(path string) Config {
	file, err := os.Open(path)
	if err != nil {
		panic("can not open file:" + path)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&c)
	if err != nil {
		return Config{
			Count:        10,
			Firstnode:    30000,
			Gossipfactor: 1,
			Chsize:       10,
			Gossip:       0,
			Minedges:     3,
			Push:         10,
			Pull:         20,
			Roundmax:     25,
		}
	}
	return Config{
		Count:        c.Count,
		Firstnode:    c.Firstnode,
		Gossipfactor: c.Gossipfactor,
		Chsize:       c.Chsize,
		Gossip:       c.Gossip,
		Minedges:     c.Minedges,
		Push:         c.Push,
		Pull:         c.Pull,
		Roundmax:     c.Roundmax,
	}
}
