package mspool

import (
	"github.com/ErizJ/JG0/mslog"
	"time"
)

type Worker struct {
	pool *Pool
	task chan func()
	//执行任务开始时间
	lastTime time.Time
}

func (w *Worker) Run() {
	w.pool.incrRunning()
	go w.running()
}

func (w *Worker) running() {
	defer func() {
		if err := recover(); err != nil {
			if w.pool.PanicHandler != nil {
				w.pool.PanicHandler()
			} else {
				mslog.Default().Error(err)
			}
		}
		w.pool.decrRunning()
		w.pool.workerCache.Put(w)
		w.pool.cond.Signal()
	}()
	for f := range w.task {
		if f == nil {
			return
		}
		f()
		w.pool.PutWorker(w)
	}
}
