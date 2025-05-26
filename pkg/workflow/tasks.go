package workflow

import (
	"log"
	"sync"
	"time"
)

type Task interface {
	Execute() error
}

type TaskManager struct {
	tickers []*time.Ticker
	mu      sync.Mutex
	running map[string]bool
}

func (tm *TaskManager) StartTasks() {
	task1 := &MaintenanceToFeishuTask{
		productName: "JumpServer", maxValue: 1000,
		feishuRecords: make(map[string]Record),
	}
	task2 := &MaintenanceRecordToFeishuTask{
		feishuRecords: make(map[string]Record),
	}
	tm.startCronJob("企业基本数据回传飞书", task1)
	tm.startCronJob("维护记录数据回传飞书", task2)
}

func (tm *TaskManager) startCronJob(name string, task Task) {
	go func() {
		timer := time.NewTimer(0)
		for {
			<-timer.C

			log.Printf("开始执行任务: %v", name)
			if err := task.Execute(); err != nil {
				log.Printf("[%s] 任务执行失败: %v", name, err)
			}

			timer.Reset(1 * time.Minute)
		}
	}()
}

func (tm *TaskManager) Stop() {
	log.Println("正在停止所有定时任务...")
	for _, ticker := range tm.tickers {
		ticker.Stop()
	}
}
