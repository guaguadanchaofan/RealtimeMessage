# A股关键消息钉钉群机器人

Go + Docker 实现，支持按源调度、Redis 去重、钉钉加签推送、全局限流、热加载（SIGHUP 或定时）。

## 运行

### Docker Compose

```bash
docker compose up -d --build
```

### 本地运行

```bash
go mod download

go run ./cmd/dingbot -config config.yaml
```

## 热加载

- 定时：`runtime.reload_interval_seconds` > 0
- 手动：`kill -HUP <pid>`

## 配置

详见 `config.yaml`，支持 per-source 的 `poll_interval_seconds` / `timeout_ms` / `retry.max_attempts`。
钉钉 `webhook`/`secret` 直接填在配置文件里。

## 注意

- 单次请求超时会强制截断为 <= 10s，重试最多 3 次。
- 同源不会并发，超过 tick 会记录 missed。
