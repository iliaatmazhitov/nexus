package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iliaatmazhitov/nexus/internal/api"
	"github.com/iliaatmazhitov/nexus/internal/health"
	"golang.org/x/sync/errgroup"
)

const (
	drainTimeout = 0 * time.Second
	stopTimeout  = 10 * time.Second
)

// Запуск сервиса
func run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	h := health.New()
	router, err := api.NewRouter(api.Dependencies{Health: h})

	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	// TODO: проанализировать таймауты, продумать более точные значения
	// Пока так:
	// * ReadHeaderTimeout - 5 секунд, в целом заголовки проходят достаточно быстро, 5 секунд возможно даже излишне много
	// * IdleTimeout - 60 секунд, компромисс между переиспользованием соединений и тем, что каждое открытое соединение
	//   занимает память и файловый дескриптор. Чутка подумал за будущий балансировщик, пока 60 вроде норм, но возможно увеличим.
	// ReadTimeout, WriteTimeout не ставлю, потому что ReadTimeout может поломать загрузку файлов, которая у нас будет
	// в будущем, а WriteTimeout может поломать WebSocket
	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,  // защита от Slowloris
		IdleTimeout:       60 * time.Second, // не держим простаивающие соединения вечно
		Addr:              ":8080",
		Handler:           router,
	}

	g, gctx := errgroup.WithContext(ctx)

	// Задача: сервер
	g.Go(func() error {
		logger.Info("server started", "addr", server.Addr)
		h.SetReady(true)
		err := server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen: %w", err)
		}

		return nil
	})

	// Задача: остановка
	g.Go(func() error {
		<-gctx.Done()
		h.SetReady(false)
		logger.Info("not ready, draining", "drain", drainTimeout)
		time.Sleep(drainTimeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), stopTimeout)
		logger.Info("shutting down")
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		return err

	})

	if err := g.Wait(); err != nil {
		return err
	}

	logger.Info("server stoped")
	return nil
}

func main() {
	ctx := context.Background()

	if err := run(ctx); err != nil {
		slog.Error("service failed", "err", err)
		os.Exit(1)
	}
}
