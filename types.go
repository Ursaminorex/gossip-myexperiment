package main

import (
	"encoding/json"
	"os"
)

type Message struct {
	Data         string // 消息内容
	Round        int    // 轮次
	Path         string // 单源路径
	HistoryNodes string
}

type Config struct {
	Count        int //节点数量
	Firstnode    int //起始端口
	Gossipfactor int //八卦因子
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
		}
	}
	return Config{
		Count:        c.Count,
		Firstnode:    c.Firstnode,
		Gossipfactor: c.Gossipfactor,
	}
}
