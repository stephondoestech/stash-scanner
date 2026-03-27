package scheduler

import (
	"context"
	"fmt"
	"time"
)

type Scheduler struct {
	interval  time.Duration
	dailyTime string
}

func New(interval time.Duration, dailyTime string) *Scheduler {
	return &Scheduler{
		interval:  interval,
		dailyTime: dailyTime,
	}
}

func (s *Scheduler) Run(ctx context.Context, run func(context.Context) error) error {
	if err := run(ctx); err != nil {
		return err
	}

	for {
		wait, err := s.nextWait(time.Now())
		if err != nil {
			return err
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			if err := run(ctx); err != nil {
				return err
			}
		}
	}
}

func (s *Scheduler) nextWait(now time.Time) (time.Duration, error) {
	if s.interval > 0 {
		return s.interval, nil
	}

	if s.dailyTime == "" {
		return 0, fmt.Errorf("scheduler is not configured")
	}

	parsed, err := time.Parse("15:04", s.dailyTime)
	if err != nil {
		return 0, fmt.Errorf("parse daily time: %w", err)
	}

	next := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		parsed.Hour(),
		parsed.Minute(),
		0,
		0,
		now.Location(),
	)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}

	return next.Sub(now), nil
}
