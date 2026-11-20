// Command worker runs background jobs: notification delivery, GPS persistence,
// offer timeouts, scheduled dispatch, stale-driver sweeps, petrol crowd ratings,
// luxury-bus food dispatch and analytics flushes.
//
// Every periodic job takes a Redis lock named "job:<name>" for its interval, so
// running several worker replicas never executes the same job twice.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"sarathi/app/bootstrap"
	"sarathi/internal/cache"
	"sarathi/internal/common/utils"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.New(ctx)
	if err != nil {
		panic(err)
	}
	defer app.Close()
	s, log := app.Services, app.Log
	consumer := app.Cfg.InstanceID

	var wg sync.WaitGroup
	goLoop := func(name string, fn func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				if err := fn(ctx); err != nil && ctx.Err() == nil {
					log.Error("worker loop crashed, restarting", "loop", name, "err", err)
					time.Sleep(2 * time.Second)
				}
			}
		}()
	}
	every := func(name string, interval time.Duration, fn func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := time.NewTicker(interval)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					lk, err := s.Locker.Acquire(ctx, "job:"+name, interval)
					if errors.Is(err, cache.ErrLockNotAcquired) {
						continue // another replica runs it this tick
					}
					if err != nil {
						log.Warn("job lock", "job", name, "err", err)
						continue
					}
					if err := fn(ctx); err != nil {
						log.Warn("job failed", "job", name, "err", err)
					}
					_ = lk.Release(context.WithoutCancel(ctx))
				}
			}
		}()
	}

	// stream consumers (consumer groups: safe on many replicas)
	goLoop("notifications", func(ctx context.Context) error { return s.Notify.Run(ctx, consumer) })
	goLoop("gps-persist", func(ctx context.Context) error { return s.GPS.RunPersister(ctx, consumer) })

	// periodic jobs
	every("offer-timeouts", time.Second, func(ctx context.Context) error { _, err := s.Dispatch.ExpireDue(ctx); return err })
	every("scheduled-dispatch", 30*time.Second, func(ctx context.Context) error { _, err := s.Booking.DispatchDue(ctx); return err })
	every("stale-drivers", 30*time.Second, func(ctx context.Context) error { _, err := s.Driver.SweepStale(ctx); return err })
	every("stale-vehicles", time.Minute, func(ctx context.Context) error { _, err := s.GPS.CleanupStale(ctx); return err })
	every("petrol-ratings", 10*time.Minute, s.Petrol.RecomputeRatings)
	every("lux-food-dispatch", time.Minute, func(ctx context.Context) error { _, err := s.Luxury.DispatchFoodOrders(ctx); return err })
	every("station-warmup", 6*time.Hour, func(ctx context.Context) error {
		if err := s.Petrol.WarmUp(ctx); err != nil {
			return err
		}
		return s.EV.WarmUp(ctx)
	})
	every("analytics-flush", time.Hour, func(ctx context.Context) error {
		yesterday := utils.DateKey(time.Now().AddDate(0, 0, -1))
		return s.Analytics.FlushDay(ctx, yesterday)
	})

	log.Info("worker started", "consumer", consumer)
	<-ctx.Done()
	wg.Wait()
	log.Info("worker stopped")
}
