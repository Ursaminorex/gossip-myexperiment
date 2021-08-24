package main

type Message struct {
	Data string			// 消息内容
	Round int			// 轮次
	Path []string		// 单源路径
	//HistoryNodes []string
}
