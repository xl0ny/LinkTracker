package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type linkChecker interface {
	Check(ctx context.Context, link domain.Link) (domain.LinkCheckOutcome, error)
}

type botNotifier interface {
	Notify(ctx context.Context, chatID int64, link domain.Link, description string) error
	NotifyFailedLinks(ctx context.Context, chatID int64, links []string) error
}

type schedulerRrepository interface {
	ListSubscribedLinks(ctx context.Context, limit, offset int) ([]domain.SubscribedLink, error)
	UpdateLinkUpdatedAt(ctx context.Context, chatID int64, linkURL string, t time.Time) error
}

type Scheduler struct {
	repo        schedulerRrepository
	linkChecker linkChecker
	botNotifier botNotifier
	batchSize   int
	workers     int
	interval    time.Duration

	jobs       chan scheduledJob
	workersWG  sync.WaitGroup
	producerWG sync.WaitGroup
	runMu      sync.Mutex

	onWorkerStart func()
}

func NewScheduler(
	repo schedulerRrepository,
	lc linkChecker,
	bn botNotifier,
	batchSize, workers int,
	interval time.Duration,
) *Scheduler {
	if batchSize < 1 {
		batchSize = 100
	}
	if workers < 1 {
		workers = 1
	}
	if interval <= 0 {
		interval = time.Minute
	}
	return &Scheduler{
		repo:        repo,
		linkChecker: lc,
		botNotifier: bn,
		batchSize:   batchSize,
		workers:     workers,
		interval:    interval,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	s.jobs = make(chan scheduledJob, s.batchSize)
	s.startWorkers(ctx)

	runProducerLocked := func() {
		s.runMu.Lock()
		defer s.runMu.Unlock()
		s.produce(ctx)
	}

	sch := gocron.NewScheduler(time.UTC)
	_, err := sch.Every(s.interval).WaitForSchedule().Do(func() {
		s.producerWG.Go(func() {
			runProducerLocked()
		})
	})
	if err != nil {
		slog.Error("scheduler: register periodic job", slog.String("error", err.Error()))
		return
	}
	runProducerLocked()
	sch.StartAsync()
	<-ctx.Done()
	sch.Stop()
	s.producerWG.Wait()
	close(s.jobs)
	s.workersWG.Wait()
}

type linkProcessError struct {
	ChatID int64
	Link   string
	Err    error
}

func (s *Scheduler) processLink(ctx context.Context, sub domain.SubscribedLink) error {
	out, err := s.linkChecker.Check(ctx, sub.Link)
	if err != nil {
		return fmt.Errorf("check link: %w", err)
	}
	if sub.Link.LastUpdated.IsZero() && !out.Latest.IsZero() {
		if err := s.repo.UpdateLinkUpdatedAt(ctx, sub.ChatID, sub.Link.URL, out.Latest); err != nil {
			return fmt.Errorf("update link date: %w", err)
		}
		return nil
	}
	if !out.Changed {
		return nil
	}
	if err := s.botNotifier.Notify(ctx, sub.ChatID, sub.Link, out.Description); err != nil {
		return fmt.Errorf("notify update: %w", err)
	}
	if err := s.repo.UpdateLinkUpdatedAt(ctx, sub.ChatID, sub.Link.URL, out.Latest); err != nil {
		return fmt.Errorf("update link date: %w", err)
	}
	return nil
}

func (s *Scheduler) reportFailedLinks(ctx context.Context, failedByChat map[int64][]string) {
	for chatID, links := range failedByChat {
		if len(links) == 0 {
			continue
		}
		if err := s.botNotifier.NotifyFailedLinks(ctx, chatID, links); err != nil {
			slog.Error(
				"scheduler: failed links report notification error",
				slog.Int64("chat_id", chatID),
				slog.String("links", strings.Join(links, ", ")),
				slog.String("error", err.Error()),
			)
		}
	}
}

func (s *Scheduler) collectFailures(errs []linkProcessError) map[int64][]string {
	failedByChat := make(map[int64][]string)
	for _, item := range errs {
		slog.Error(
			"scheduler: link processing error",
			slog.Int64("chat_id", item.ChatID),
			slog.String("url", item.Link),
			slog.String("error", item.Err.Error()),
		)
		failedByChat[item.ChatID] = append(failedByChat[item.ChatID], item.Link)
	}
	return failedByChat
}

func (s *Scheduler) loadAllSubscribedLinks(ctx context.Context) ([]domain.SubscribedLink, error) {
	var (
		all    []domain.SubscribedLink
		offset int
	)
	for {
		batch, err := s.repo.ListSubscribedLinks(ctx, s.batchSize, offset)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		offset += len(batch)
	}
	return all, nil
}

func (s *Scheduler) logStart(total int) {
	slog.Info(
		"scheduler: check all links start",
		slog.Int("links", total),
		slog.Int("batch_size", s.batchSize),
		slog.Int("workers", s.workers),
		slog.Duration("interval", s.interval),
	)
}

func (s *Scheduler) logDone(total, failed int) {
	slog.Info(
		"scheduler: check all links done",
		slog.Int("links", total),
		slog.Int("failed", failed),
	)
}

type scheduledJob struct {
	sub domain.SubscribedLink
	run *runState
}

type runState struct {
	wg     sync.WaitGroup
	mu     sync.Mutex
	errors []linkProcessError
}

func (r *runState) addError(item linkProcessError) {
	r.mu.Lock()
	r.errors = append(r.errors, item)
	r.mu.Unlock()
}

func (r *runState) getErrors() []linkProcessError {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]linkProcessError, len(r.errors))
	copy(out, r.errors)
	return out
}

func (s *Scheduler) startWorkers(ctx context.Context) {
	for i := 0; i < s.workers; i++ {
		s.workersWG.Go(func() {
			if s.onWorkerStart != nil {
				s.onWorkerStart()
			}
			for job := range s.jobs {
				if err := s.processLink(ctx, job.sub); err != nil {
					job.run.addError(linkProcessError{ChatID: job.sub.ChatID, Link: job.sub.Link.URL, Err: err})
				}
				job.run.wg.Done()
			}
		})
	}
}

func (s *Scheduler) enqueueJob(ctx context.Context, job scheduledJob) bool {
	select {
	case <-ctx.Done():
		return false
	case s.jobs <- job:
		return true
	}
}

func (s *Scheduler) produce(ctx context.Context) {
	links, err := s.loadAllSubscribedLinks(ctx)
	if err != nil {
		slog.Error("scheduler: list subscribed links error", slog.String("error", err.Error()))
		return
	}
	if len(links) == 0 {
		return
	}
	s.logStart(len(links))

	run := &runState{}
	for _, sub := range links {
		run.wg.Add(1)
		if ok := s.enqueueJob(ctx, scheduledJob{sub: sub, run: run}); !ok {
			run.wg.Done()
			break
		}
	}
	run.wg.Wait()

	errs := run.getErrors()
	failedByChat := s.collectFailures(errs)
	s.reportFailedLinks(ctx, failedByChat)
	s.logDone(len(links), len(errs))
}
