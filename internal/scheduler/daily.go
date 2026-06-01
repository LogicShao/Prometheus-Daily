package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"m-daily-news/internal/daily"
	"m-daily-news/internal/generate"
)

type Runner interface {
	Run(ctx context.Context, rawDate string) (*generate.Result, error)
}

type Daily struct {
	runner Runner
	at     TimeOfDay
	now    func() time.Time
}

type TimeOfDay struct {
	Hour   int
	Minute int
}

func NewDaily(runner Runner, at TimeOfDay) *Daily {
	return &Daily{runner: runner, at: at, now: time.Now}
}

func ParseTimeOfDay(raw string) (TimeOfDay, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return TimeOfDay{}, errors.New("schedule must use HH:MM")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return TimeOfDay{}, fmt.Errorf("invalid schedule hour: %w", err)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return TimeOfDay{}, fmt.Errorf("invalid schedule minute: %w", err)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return TimeOfDay{}, errors.New("schedule time out of range")
	}
	return TimeOfDay{Hour: hour, Minute: minute}, nil
}

func (d *Daily) Start(ctx context.Context) {
	go d.loop(ctx)
}

func (d *Daily) loop(ctx context.Context) {
	for {
		next := d.nextRun(d.now())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			d.runOnce(ctx)
		}
	}
}

func (d *Daily) nextRun(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), d.at.Hour, d.at.Minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func (d *Daily) runOnce(ctx context.Context) {
	result, err := d.runner.Run(ctx, "")
	if errors.Is(err, daily.ErrExists) {
		log.Printf("daily scheduler skipped: today's daily already exists")
		return
	}
	if errors.Is(err, generate.ErrRunning) {
		log.Printf("daily scheduler skipped: generation already running")
		return
	}
	if err != nil {
		log.Printf("daily scheduler failed: %v", err)
		return
	}
	log.Printf("daily scheduler generated %s: %s", result.Date, result.File)
}
