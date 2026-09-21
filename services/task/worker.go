package task

import "fmt"

// Run 是异步任务入口。二次开发时在这里注册队列消费者或定时任务。
func Run() error { fmt.Println("task service ready: no tasks registered"); return nil }
