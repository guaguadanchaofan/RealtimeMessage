package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"realtime-message/internal/config"
	"realtime-message/internal/dedupe"
	"realtime-message/internal/fetcher"
	"realtime-message/internal/logging"
	"realtime-message/internal/model"
	"realtime-message/internal/parser"
	"realtime-message/internal/push"
	"realtime-message/internal/scoring"
)

type Worker struct {
	source   config.SourceConfig
	network  config.NetworkConfig
	scoring  scoring.Engine
	store    *dedupe.Store
	pusher   *push.DingTalk
	rate     *push.RateLimiter
	logger   *logging.Logger
	missed   atomic.Int64
}

func NewWorker(src config.SourceConfig, netcfg config.NetworkConfig, score scoring.Engine, store *dedupe.Store, pusher *push.DingTalk, rate *push.RateLimiter, logger *logging.Logger) *Worker {
	return &Worker{
		source:  src,
		network: netcfg,
		scoring: score,
		store:   store,
		pusher:  pusher,
		rate:    rate,
		logger:  logger,
	}
}

func (w *Worker) Run(ctx context.Context) {
	interval := w.source.PollIntervalSeconds
	if interval <= 0 {
		interval = 60
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	var running atomic.Bool
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !running.CompareAndSwap(false, true) {
				w.missed.Add(1)
				w.logger.Warn("missed tick", logging.Field{Key: "source", Val: w.source.Name}, logging.Field{Key: "missed", Val: w.missed.Load()})
				continue
			}
			start := time.Now()
			w.fetchOnce(ctx)
			w.logger.Info("fetch done", logging.Field{Key: "source", Val: w.source.Name}, logging.Field{Key: "elapsed_ms", Val: time.Since(start).Milliseconds()})
			running.Store(false)
		}
	}
}

func (w *Worker) fetchOnce(ctx context.Context) {
	timeout := clampTimeout(w.source.TimeoutMS, w.network.DefaultTimeoutMS)
	retry := clampRetry(w.source.Retry, w.network.Retry)

	client := fetcher.New(time.Duration(timeout)*time.Millisecond, retry.RetryOnStatus, retry.MaxAttempts, retry.BackoffMS, retry.Multiplier, retry.JitterMS)

	req, _ := http.NewRequest("GET", w.source.URL, nil)
	for k, v := range w.source.Headers {
		req.Header.Set(k, v)
	}

	status, body, err := client.Do(ctx, req)
	if err != nil {
		w.logger.Error("fetch failed", logging.Field{Key: "source", Val: w.source.Name}, logging.Field{Key: "status", Val: status}, logging.Field{Key: "err", Val: err})
		return
	}

	var msgs []model.Message
	switch strings.ToLower(w.source.Type) {
	case "rss":
		msgs, err = parser.ParseRSS(w.source.Name, body)
	default:
		msgs, err = parser.ParseJSON(w.source.Name, body, w.source.Parser)
	}
	if err != nil {
		w.logger.Error("parse failed", logging.Field{Key: "source", Val: w.source.Name}, logging.Field{Key: "err", Val: err})
		return
	}
	w.logger.Info("parsed", logging.Field{Key: "source", Val: w.source.Name}, logging.Field{Key: "count", Val: len(msgs)})

	for _, m := range msgs {
		scored := w.scoring.Score(m)
		if scored.Score < w.scoring.Scoring.PushThreshold {
			continue
		}
		seen, key, err := w.store.Seen(ctx, scored.Message)
		if err != nil {
			w.logger.Error("dedupe failed", logging.Field{Key: "source", Val: w.source.Name}, logging.Field{Key: "err", Val: err})
			continue
		}
		if seen {
			w.logger.Info("dedupe hit", logging.Field{Key: "source", Val: w.source.Name}, logging.Field{Key: "key", Val: key})
			continue
		}
		if !w.rate.Allow() {
			w.logger.Warn("rate limited", logging.Field{Key: "source", Val: w.source.Name})
			continue
		}
		content := w.render(scored)
		if strings.ToLower(w.pusher.MsgType) == "text" {
			err = w.pusher.SendText(content)
		} else {
			err = w.pusher.SendMarkdown(content)
		}
		if err != nil {
			w.logger.Error("push failed", logging.Field{Key: "source", Val: w.source.Name}, logging.Field{Key: "err", Val: err})
		} else {
			w.logger.Info("pushed", logging.Field{Key: "source", Val: w.source.Name}, logging.Field{Key: "score", Val: scored.Score})
		}
	}
}

func (w *Worker) render(msg model.ScoredMessage) string {
	values := map[string]string{
		"source":  msg.Source,
		"title":   msg.Title,
		"content": msg.Content,
		"time":    msg.Time.Format("2006-01-02 15:04:05"),
		"score":   fmt.Sprintf("%d", msg.Score),
		"reasons": strings.Join(msg.Reasons, ","),
		"link":    msg.URL,
	}
	if w.pusher.Template == "" {
		return fmt.Sprintf("[%s] %s\n%s\n%s", msg.Source, msg.Title, msg.Content, msg.URL)
	}
	return push.RenderTemplate(w.pusher.Template, values)
}

func clampTimeout(srcTimeout, defaultTimeout int) int {
	if srcTimeout <= 0 {
		srcTimeout = defaultTimeout
	}
	if srcTimeout > 10000 {
		return 10000
	}
	return srcTimeout
}

func clampRetry(src, def config.RetryConfig) config.RetryConfig {
	ret := src
	if ret.MaxAttempts <= 0 {
		ret.MaxAttempts = def.MaxAttempts
	}
	if ret.MaxAttempts > 3 {
		ret.MaxAttempts = 3
	}
	if ret.BackoffMS <= 0 {
		ret.BackoffMS = def.BackoffMS
	}
	if ret.Multiplier <= 0 {
		ret.Multiplier = def.Multiplier
	}
	if ret.JitterMS <= 0 {
		ret.JitterMS = def.JitterMS
	}
	if len(ret.RetryOnStatus) == 0 {
		ret.RetryOnStatus = def.RetryOnStatus
	}
	return ret
}
