package background

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
)

var ErrAddJobTimeout = errors.New("failed to add new job in time")

const queueBufferMultiplier = 2

type PoolConfig struct {
	Concurrency   int
	DoJobTimeout  time.Duration
	AddJobTimeout time.Duration
}

type JobFunc func(context.Context) error

type Job struct {
	ID   string
	Name string
	do   JobFunc
}

type JobResult struct {
	Job Job
	Err error
}

func NewJob(name string, jobFunc JobFunc) Job {
	return Job{
		ID:   uuid.New().String(),
		Name: name,
		do:   jobFunc,
	}
}

func (job Job) Do(ctx context.Context) JobResult {
	maybeErr := job.do(ctx)
	return JobResult{Job: job, Err: maybeErr}
}

type Worker struct {
	JobTimeout time.Duration
}

func (worker Worker) Work(ctx context.Context, job Job) JobResult {
	ctx, cancel := context.WithTimeout(ctx, worker.JobTimeout)
	defer cancel()
	// хотя мы и передаем контекст с таймайуом мы не можем гарантировать
	// что джоб вовремя остановится, поэтому запускаем ее в горутине и сами отслеживаем время выполнения
	resultCh := make(chan JobResult)
	go func() {
		log.Printf("starting job %s [%s]", job.Name, job.ID)
		resultCh <- job.Do(ctx)
	}()
	for {
		select {
		case <-ctx.Done():
			log.Printf("deadline exceeded for job %s [%s]", job.Name, job.ID)
			return JobResult{Job: job, Err: ctx.Err()}
		case result := <-resultCh:
			log.Printf("finished job %s [%s]; err? %v", job.Name, job.ID, result.Err)
			return result
		}
	}
}

type Pool struct {
	queue chan Job
	cfg   PoolConfig
	done  chan struct{}
}

func NewPool(cfg PoolConfig) *Pool {
	done := make(chan struct{})
	queue := make(chan Job, cfg.Concurrency*queueBufferMultiplier)
	results := make(chan JobResult, cfg.Concurrency*queueBufferMultiplier)
	pool := Pool{
		done:  done,
		cfg:   cfg,
		queue: queue,
	}

	// инициализируем воркеров и управляем каналами в отдельной горутине
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		for i := 0; i < cfg.Concurrency; i++ {
			pool.addWorker(ctx, queue, results)
		}
		<-done
		cancel()
		close(queue)
		close(results)
	}()

	// читаем из канала с результами исполнения джобов и пишем их статус в лог
	go func() {
		for result := range results {
			if result.Err != nil {
				log.Printf("job %s [%s] returned an error: %s", result.Job.Name, result.Job.ID, result.Err)
			} else {
				log.Printf("job %s [%s] succeeded", result.Job.Name, result.Job.ID)
			}
		}
	}()

	return &pool
}

func (pool *Pool) Add(ctx context.Context, job Job) error {
	ctx, cancel := context.WithTimeout(ctx, pool.cfg.AddJobTimeout)
	defer cancel()
	select {
	case <-ctx.Done():
		log.Printf("failed to add job %s [%s] due to blocked queue", job.Name, job.ID)
		return ErrAddJobTimeout
	case pool.queue <- job:
		log.Printf("enqueued job %s [%s]", job.Name, job.ID)
		return nil
	}
}

func (pool *Pool) Close() {
	close(pool.done)
}

func (pool *Pool) addWorker(ctx context.Context, queue <-chan Job, results chan<- JobResult) *Worker {
	worker := Worker{JobTimeout: pool.cfg.DoJobTimeout}
	go func() {
		for {
			select {
			case job := <-queue:
				log.Printf("obtained new job %s [%s] from queue", job.Name, job.ID)
				results <- worker.Work(ctx, job)
			case <-ctx.Done():
				log.Printf("worker exited due to canceled context")
				return
			}
		}
	}()
	return &worker
}
