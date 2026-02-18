//go:build windows

package scheduler

import (
	"fmt"
	"log/slog"

	"github.com/robfig/cron/v3"
)

// slogAdapter wraps slog.Logger to provide Printf method for cron compatibility
type slogAdapter struct {
	logger *slog.Logger
}

// Printf implements the interface required by cron.VerbosePrintfLogger
func (a *slogAdapter) Printf(format string, args ...interface{}) {
	a.logger.Info(fmt.Sprintf(format, args...))
}

// Scheduler wraps cron.Cron with SkipIfStillRunning mode to prevent job overlap
type Scheduler struct {
	cron   *cron.Cron
	logger *slog.Logger
}

// New creates a new Scheduler with SkipIfStillRunning mode enabled
func New(logger *slog.Logger) *Scheduler {
	// Wrap slog.Logger for cron.Logger compatibility
	adapter := &slogAdapter{logger: logger.With("component", "scheduler")}
	cronLogger := cron.VerbosePrintfLogger(adapter)

	// Create cron with SkipIfStillRunning wrapper to prevent overlapping jobs
	c := cron.New(
		cron.WithChain(cron.SkipIfStillRunning(cronLogger)),
		cron.WithLogger(cronLogger),
	)

	return &Scheduler{
		cron:   c,
		logger: logger,
	}
}

// AddJob registers a new job with the given cron expression
func (s *Scheduler) AddJob(cronExpr string, jobFunc func()) error {
	_, err := s.cron.AddFunc(cronExpr, jobFunc)
	if err != nil {
		return fmt.Errorf("failed to add job with schedule %q: %w", cronExpr, err)
	}

	s.logger.Info("Job scheduled", "schedule", cronExpr)
	return nil
}

// Start begins the scheduler and all registered jobs
func (s *Scheduler) Start() {
	s.logger.Info("Starting scheduler")
	s.cron.Start()
}

// Stop halts the scheduler and waits for all running jobs to complete
func (s *Scheduler) Stop() {
	s.logger.Info("Stopping scheduler")
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Info("Scheduler stopped, all jobs completed")
}
