package workers

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/shellhub-io/shellhub/api/store"
	log "github.com/sirupsen/logrus"
)

const (
	TaskSessionCleanup = "session_record:cleanup"
	TaskHeartbeat      = "api:heartbeat"
)

type Workers struct {
	store store.Store

	addr      asynq.RedisConnOpt
	srv       *asynq.Server
	mux       *asynq.ServeMux
	env       *Envs
	scheduler *asynq.Scheduler

	Channels map[string]chan Task
	Workers  map[string]Worker
}

// NewWorkers creates a new Workers instance with the provided store. It initializes
// the worker's components, such as server, scheduler, and environment settings.
func NewWorkers(store store.Store) (*Workers, error) {
	env, err := getEnvs()
	if err != nil {
		log.WithFields(log.Fields{"component": "worker"}).
			WithError(err).
			Error("Failed to parse the envs.")

		return nil, err
	}

	addr, err := asynq.ParseRedisURI(env.RedisURI)
	if err != nil {
		log.WithFields(log.Fields{"component": "worker"}).
			WithError(err).
			Errorf("Failed to parse redis URI: %s.", env.RedisURI)

		return nil, err
	}

	mux := asynq.NewServeMux()
	srv := asynq.NewServer(
		addr,
		asynq.Config{ //nolint:exhaustruct
			// NOTICE:
			// To include any new task binding to a new queue (e.g., "queue:group" where 'queue' is the new queue),
			// ensure that the created queue is added here. Failure to do so will result in the server not executing the task handler.
			Queues: map[string]int{
				"api":            1,
				"session_record": 1,
			},
			GroupAggregator: asynq.GroupAggregatorFunc(
				func(group string, tasks []*asynq.Task) *asynq.Task {
					var b strings.Builder

					for _, task := range tasks {
						b.WriteString(fmt.Sprintf("%s\n", task.Payload()))
					}

					return asynq.NewTask(TaskHeartbeat, []byte(b.String()))
				},
			),
			GroupMaxDelay:    time.Duration(env.AsynqGroupMaxDelay) * time.Second,
			GroupGracePeriod: time.Duration(env.AsynqGroupGracePeriod) * time.Second,
			GroupMaxSize:     env.AsynqGroupMaxSize,
			Concurrency:      runtime.NumCPU(),
		},
	)
	scheduler := asynq.NewScheduler(addr, nil)

	w := &Workers{
		addr:      addr,
		env:       env,
		srv:       srv,
		mux:       mux,
		scheduler: scheduler,
		store:     store,
		Channels:  make(map[string]chan Task),
		Workers:   make(map[string]Worker),
	}

	return w, nil
}

func (w *Workers) registerHeartbeat(tasks chan Task) {
	w.mux.HandleFunc(TaskHeartbeat, func(ctx context.Context, task *asynq.Task) error {
		log.WithFields(log.Fields{
			"component": "worker",
			"task":      TaskHeartbeat,
		}).Trace("Executing heartbeat worker.")

		tasks <- Task{
			Payload: task.Payload(),
		}

		return nil
	})
}

func (w *Workers) registerSessionCleanup(tasks chan Task) {
	if w.env.SessionRecordCleanupRetention < 1 {
		log.WithFields(
			log.Fields{
				"component": "worker",
				"task":      TaskSessionCleanup,
			}).
			Warnf("Aborting cleanup worker due to SHELLHUB_RECORD_RETENTION equal to %d.", w.env.SessionRecordCleanupRetention)

		return
	}

	w.mux.HandleFunc(TaskSessionCleanup, func(ctx context.Context, _ *asynq.Task) error {
		log.WithFields(log.Fields{
			"component": "worker",
			"task":      TaskSessionCleanup,
		}).Info("running CleanUp worker's task")

		tasks <- Task{}

		return nil
	})

	task := asynq.NewTask(TaskSessionCleanup, nil, asynq.TaskID(TaskSessionCleanup), asynq.Queue("session_record"))
	if _, err := w.scheduler.Register(w.env.SessionRecordCleanupSchedule, task); err != nil {
		log.WithFields(
			log.Fields{
				"component": "worker",
				"task":      TaskSessionCleanup,
			}).
			WithError(err).
			Error("Failed to register the scheduler.")
	}
}

func (w *Workers) asynqScheduler(ctx context.Context) {
	w.registerSessionCleanup(w.Channels["clean_up"])
	w.registerHeartbeat(w.Channels["heartbeat"])

	go func() {
		if err := w.srv.Run(w.mux); err != nil {
			log.WithFields(log.Fields{"component": "worker"}).
				WithError(err).
				Error("Unable to run the server.")
		}
	}()

	go func() {
		if err := w.scheduler.Run(); err != nil {
			log.WithFields(log.Fields{"component": "worker"}).
				WithError(err).
				Error("Unable to run the scheduler.")
		}
	}()

	<-ctx.Done()

	log.Info("Shutdown workers")

	w.srv.Shutdown()
	w.scheduler.Shutdown()
}

// Start initiates the workers and its server and scheduler.
func (w *Workers) Init(ctx context.Context) {
	w.Workers["clean_up"] = &CleanUpWorker{
		store:                         w.store,
		SessionRecordCleanupRetention: w.env.SessionRecordCleanupRetention,
	}

	w.Workers["heartbeat"] = &HeartBeatWorker{
		store: w.store,
	}

	for id, worker := range w.Workers {
		w.Channels[id] = make(chan Task)

		go worker.Init(w.Channels[id])
	}

	go w.asynqScheduler(ctx)
}
