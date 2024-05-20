package workers

import (
	"bufio"
	"bytes"
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/shellhub-io/shellhub/api/store"
	"github.com/shellhub-io/shellhub/pkg/models"
	log "github.com/sirupsen/logrus"
)

// Worker constains methods used to started and control ShellHub's workers.
type Worker interface {
	Init(task chan Task)
}

// HeartBeatWorker worker manages heartbeat tasks, signaling the online status of devices. It aggregates heartbeat data
// and updates the online status of devices accordingly. The maximum number of devices to wait for before triggering is
// defined by the `SHELLHUB_ASYNQ_GROUP_MAX_SIZE` (default is 500). Another triggering mechanism involves a timeout
// defined in the `SHELLHUB_ASYNQ_GROUP_MAX_DELAY` environment variable.
type HeartBeatWorker struct {
	store store.Store
}

func (w *HeartBeatWorker) Init(tasks chan Task) {
	ctx := context.Background()

	for task := range tasks {
		scanner := bufio.NewScanner(bytes.NewReader(task.Payload))
		scanner.Split(bufio.ScanLines)

		devices := make([]models.ConnectedDevice, 0)
		for scanner.Scan() {
			parts := strings.Split(scanner.Text(), "=")
			if len(parts) != 2 {
				log.WithFields(
					log.Fields{
						"component": "worker",
						"task":      TaskHeartbeat,
					}).
					Warn("failed to parse queue payload due to lack of '='.")

				continue
			}

			lastSeen, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				log.WithFields(
					log.Fields{
						"component": "worker",
						"task":      TaskHeartbeat,
					}).
					WithError(err).
					Warn("failed to parse timestamp to integer.")

				continue
			}

			parts = strings.Split(parts[0], ":")
			if len(parts) != 2 {
				log.WithFields(
					log.Fields{
						"component": "worker",
						"task":      TaskHeartbeat,
					}).
					Warn("failed to parse queue payload due to lack of ':'.")

				continue
			}

			device := models.ConnectedDevice{
				UID:      parts[1],
				TenantID: parts[0],
				LastSeen: time.Unix(lastSeen, 0),
			}

			devices = append(devices, device)
		}

		if err := w.store.DeviceSetOnline(ctx, devices); err != nil {
			log.
				WithError(err).
				WithFields(log.Fields{
					"component": "worker",
					"task":      TaskHeartbeat,
				}).
				Error("failed to set devices as online")
		}
	}
}

// CleanUpWorker worker is designed to delete recorded sessions older than a specified number of days. The retention
// period is determined by the value of the `SHELLHUB_RECORD_RETENTION` environment variable. To disable this worker,
// set `SHELLHUB_RECORD_RETENTION` to 0 (default behavior). It uses a cron expression from `SHELLHUB_RECORD_RETENTION`
// to schedule its periodic execution.
type CleanUpWorker struct {
	store store.Store

	SessionRecordCleanupRetention int
}

func (w *CleanUpWorker) Init(tasks chan Task) {
	ctx := context.Background()

	for range tasks {
		lte := time.Now().UTC().AddDate(0, 0, w.SessionRecordCleanupRetention*(-1))

		deletedCount, updatedCount, err := w.store.SessionDeleteRecordFrameByDate(ctx, lte)
		if err != nil {
			log.WithFields(log.Fields{
				"deleted_count": deletedCount,
				"updated_count": updatedCount,
			}).Error(err)
		}
	}
}
