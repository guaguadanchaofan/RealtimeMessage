package core

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"realtime-message/internal/config"
	"realtime-message/internal/dedupe"
	"realtime-message/internal/logging"
	"realtime-message/internal/push"
	"realtime-message/internal/scoring"
)

type Manager struct {
	cfgPath string
	logger  *logging.Logger
	cancel  context.CancelFunc
}

func NewManager(cfgPath string, logger *logging.Logger) *Manager {
	return &Manager{cfgPath: cfgPath, logger: logger}
}

func (m *Manager) Start(ctx context.Context) error {
	cfg, err := config.Load(m.cfgPath)
	if err != nil {
		return err
	}
	if err := m.applyRuntime(cfg); err != nil {
		return err
	}
	m.runWithConfig(ctx, cfg)
	m.handleSignals(ctx)
	m.handleReload(ctx, cfg.Runtime.ReloadIntervalSeconds)
	<-ctx.Done()
	return nil
}

func (m *Manager) runWithConfig(ctx context.Context, cfg config.Config) {
	if m.cancel != nil {
		m.cancel()
	}
	workerCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	store := dedupe.New(cfg.Redis, cfg.Dedupe)
	pusher := &push.DingTalk{
		Webhook:  cfg.Dingding.Webhook,
		Secret:   cfg.Dingding.Secret,
		MsgType:  cfg.Dingding.MsgType,
		Title:    cfg.Dingding.Title,
		Timeout:  time.Duration(cfg.Dingding.TimeoutMS) * time.Millisecond,
		Template: cfg.Push.Template.Markdown,
	}
	rate := push.NewRateLimiter(cfg.Push.MaxPushPerMinute)
	scoreEngine := scoring.Engine{Topics: cfg.Topics, Triggers: cfg.Triggers, Scoring: cfg.Scoring}

	scores := map[string]int{}
	for _, src := range cfg.Sources {
		scores[src.Name] = src.BaseScore
	}
	scoring.SetBaseScores(scores)

	for _, src := range cfg.Sources {
		src := src
		if src.PollIntervalSeconds <= 0 {
			src.PollIntervalSeconds = cfg.Runtime.DefaultPollIntervalSeconds
		}
		worker := NewWorker(src, cfg.Network, scoreEngine, store, pusher, rate, m.logger)
		go worker.Run(workerCtx)
	}
	m.logger.Info("workers started", logging.Field{Key: "sources", Val: len(cfg.Sources)})
}

func (m *Manager) handleSignals(ctx context.Context) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				m.reload(ctx, "signal")
			}
		}
	}()
}

func (m *Manager) handleReload(ctx context.Context, interval int) {
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.reload(ctx, "timer")
			}
		}
	}()
}

func (m *Manager) reload(ctx context.Context, reason string) {
	cfg, err := config.Load(m.cfgPath)
	if err != nil {
		m.logger.Error("reload failed", logging.Field{Key: "err", Val: err})
		return
	}
	if err := m.applyRuntime(cfg); err != nil {
		m.logger.Error("reload runtime failed", logging.Field{Key: "err", Val: err})
		return
	}
	m.logger.Info("reloading", logging.Field{Key: "reason", Val: reason})
	m.runWithConfig(ctx, cfg)
}

func (m *Manager) applyRuntime(cfg config.Config) error {
	m.logger.SetJSON(cfg.Logging.JSON)
	if cfg.Runtime.Timezone != "" {
		loc, err := time.LoadLocation(cfg.Runtime.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone: %w", err)
		}
		time.Local = loc
	}
	return nil
}
