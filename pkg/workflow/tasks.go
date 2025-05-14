package workflow

import (
	"log"
	"time"
)

type Task interface {
	Execute() error
}

type TaskManager struct {
	tickers []*time.Ticker
}

func (tm *TaskManager) StartTasks() {
	tm.startCronJob("支持门户数据回传飞书", SupportToFeishuTask{})
}

func (tm *TaskManager) startCronJob(name string, task Task) {
	ticker := time.NewTicker(1 * time.Minute)
	tm.tickers = append(tm.tickers, ticker)

	go func() {
		for range ticker.C {
			log.Printf("开始执行任务: %v", name)
			if err := task.Execute(); err != nil {
				log.Printf("[%s] 任务执行失败: %v", name, err)
			}
		}
	}()
}

func (tm *TaskManager) Stop() {
	log.Println("正在停止所有定时任务...")
	for _, ticker := range tm.tickers {
		ticker.Stop()
	}
}
