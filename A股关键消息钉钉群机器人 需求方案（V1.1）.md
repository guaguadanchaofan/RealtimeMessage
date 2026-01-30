# A股关键消息钉钉群机器人 需求方案（V1.1）

## 1. 项目概述

### 1.1 项目名称

A股关键消息钉钉群机器人（Quality-first）

### 1.2 目标

从多个消息源实时获取市场消息，按“少而精”策略筛选关键内容，Redis 去重后推送到钉钉群。

### 1.3 核心原则

- 少而精：只推高重要度消息；中低重要度不刷屏（可选汇总）
- 可配置：新增/修改消息源、主题词、阈值、频控无需改代码或尽量少改
- 可靠：每个接口请求超时≤10s，最多重试3次；失败不影响其他源
- 可追溯：推送必须带来源/时间/链接

------

## 2. 范围

### 2.1 V1 必须实现

- 多消息源（HTTP JSON / RSS）
- **每个消息源可配置抓取频率**（poll interval）
- 单次请求超时≤10s、最多重试3次（强制上限）
- 规则打分筛选（主题词、强触发词、来源权重）
- Redis 去重（TTL 可配）
- 钉钉 webhook 加签推送（markdown/text）
- 全局推送频控（每分钟最多 N 条）
- 日志（抓取、解析、打分、去重、推送、错误原因）

### 2.2 V2 可选

- 中低分消息定时汇总推送
- 黑白名单（屏蔽源/屏蔽词/只推指定主题）
- LLM 生成摘要与重要性判断
- 管理后台/热更新配置

------

## 3. 关键约束（Hard Requirements）

### 3.1 每个接口请求超时与重试

对每个 source 抓取：

- 单次请求超时：`timeout_ms <= 10000`
- 最大尝试次数：`max_attempts <= 3`（第一次 + 最多2次重试）
- 可重试条件：网络错误、HTTP 5xx、HTTP 429（按退避）
- 不重试：HTTP 4xx（除 429）、解析错误（默认）

> 说明：即使配置写更大，上线时也必须强制截断到不超过上述上限。

### 3.2 每个接口抓取频率（新增）

- 每个 source 都可配置 `poll_interval_seconds`
- 系统调度必须保证：
  - **各 source 独立节奏执行**，不会被其他 source 的慢请求阻塞
  - 同一 source 在“上一次执行尚未结束”时，不应并发开启下一次（避免叠请求）
    - 策略：跳过/延后（建议跳过并记录一次 “missed_tick” 计数）

------

## 4. 用户故事

- 群成员：希望看到的都是“影响市场的关键消息”，不要电报式刷屏。
- 管理员：希望通过配置文件新增消息源、改主题词权重、改频率阈值即可生效。
- 运维：希望服务稳定，接口失败会降级，不影响其他源。

------

## 5. 功能需求

### 5.1 配置加载与校验

- 启动加载 `config.yaml`
- 校验失败则拒绝启动（明确错误）
- （可选）支持热加载：定时 reload 或 SIGHUP

### 5.2 消息源采集（per-source 调度）

每个 source 独立配置：

- URL
- 请求超时
- 重试策略
- 抓取频率
- 解析策略（auto / mapping）

调度要求：

- 使用“每源一个 worker”（goroutine 或独立任务）：
  - worker 内部按自己的 poll interval 循环
  - 每次抓取串行执行（同源不并发）
  - 抓取失败不会阻塞其他源

### 5.3 解析与标准化

内部标准消息结构：

- `id`
- `title`
- `content`
- `url`
- `time`
- `source`

解析模式：

- **auto**：自动在 JSON 里找 `data/list/items/result` 的数组，并尝试识别 `title/content/url/time/id`
- **mapping**：配置里显式指定 list_path 与字段名（更稳，推荐生产）

### 5.4 打分与筛选（少而精）

分数构成：

- source.base_score
- 命中 topics（主题词）加分：每主题有 weight
- 命中 strong triggers 加分：额外加权
- （可选）交易时段修正：盘中更敏感、盘后更严格

筛选：

- `score >= push_threshold` 才允许推送
- （可选 V2）`digest_threshold <= score < push_threshold` 进入汇总池

### 5.5 去重（Redis）

- 去重键生成优先级可配置：
  - url → id → source+title → source+title+time
- Redis：`SETNX key value EX ttl`
- TTL 默认 72h，可配置

### 5.6 钉钉推送

- Webhook + 加签（timestamp/sign）
- msg_type：markdown/text 可配置
- 模板可配置（markdown 模板推荐）

### 5.7 推送频控（全局）

- `max_push_per_minute`：每分钟最多推 N 条（少而精推荐 1~3）
- 超过后丢弃或进入汇总（V2）
- 记录频控丢弃数量用于调参

### 5.8 日志与可观测性

至少日志包含：

- source 抓取成功/失败、耗时、重试次数
- 解析条数、丢弃条数、推送条数
- 去重命中次数
- 推送失败原因（errcode/errmsg）

（可选）metrics：Prometheus 指标

------

## 6. 配置文件规范（重点：每源频率可配）

### 6.1 顶层结构

- runtime
- network
- redis
- dingding
- scoring
- sources
- topics
- triggers
- push
- dedupe
- logging

### 6.2 配置字段定义

#### runtime

- `timezone`：默认 Asia/Shanghai
- `default_poll_interval_seconds`：默认频率（source 未配置则用）

#### network（强制上限）

- `default_timeout_ms`：默认 8000~10000
- `retry.max_attempts`：默认 3（强制 ≤3）
- `retry.backoff_ms`、`retry.multiplier`、`retry.jitter_ms`
- `retry.retry_on_status`：429/5xx

#### sources（每个 source 独立频率）

- `name`
- `type`：http_json / rss
- `url`
- `poll_interval_seconds`：**每源频率**
- `timeout_ms`：≤10000
- `retry.max_attempts`：≤3
- `base_score`
- `headers`（可选）
- `parser`（auto/mapping）

#### topics

- `name`
- `weight`
- `keywords`

#### triggers

- `strong.weight`
- `strong.keywords`

#### scoring

- `push_threshold`
- `market_hours`（可选）

#### push

- `max_push_per_minute`
- `template.markdown`

#### dedupe

- `ttl_hours`
- `key_strategy`

------

## 7. config.yaml 示例（含 per-source 频率）

```
runtime:
  timezone: "Asia/Shanghai"
  default_poll_interval_seconds: 60

network:
  default_timeout_ms: 10000
  retry:
    max_attempts: 3
    backoff_ms: 400
    multiplier: 2.0
    jitter_ms: 200
    retry_on_status: [429, 500, 502, 503, 504]

redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 0
  key_prefix: "dingbot:"

dingding:
  webhook: "${DING_WEBHOOK}"
  secret: "${DING_SECRET}"
  msg_type: "markdown"
  title: "A股关键消息"
  timeout_ms: 8000

scoring:
  push_threshold: 90
  market_hours:
    enabled: true
    in_session_bonus: 5
    off_session_penalty: 10

sources:
  - name: "cls_all"
    type: "http_json"
    url: "https://api.98dou.cn/api/hotlist/cls/all"
    poll_interval_seconds: 30          # 该源 30s 一次
    timeout_ms: 10000
    retry:
      max_attempts: 3
    base_score: 50
    parser:
      mode: "auto"

  - name: "pbc_rss"
    type: "rss"
    url: "（填央行RSS链接）"
    poll_interval_seconds: 180         # 该源 3min 一次（低频但关键）
    timeout_ms: 10000
    retry:
      max_attempts: 3
    base_score: 80

topics:
  - name: "货币政策"
    weight: 50
    keywords: ["降准","降息","LPR","MLF","逆回购","公开市场","政策利率"]

  - name: "监管与风险"
    weight: 40
    keywords: ["证监会","交易所","立案","调查","处罚","风险提示","退市","违约","暴雷"]

  - name: "权重行业"
    weight: 25
    keywords: ["银行","券商","保险","地产","煤炭","石油","中字头"]

triggers:
  strong:
    weight: 30
    keywords: ["紧急","临时","重大","暂停","停牌","复牌","重组","回购","预增","预亏"]

push:
  max_push_per_minute: 2
  template:
    markdown: |
      #### 【${source}】${title}
      > 时间：${time}
      > 评分：${score}
      > 命中：${reasons}

      ${content}

      ${link}

dedupe:
  ttl_hours: 72
  key_strategy: ["url","id","source_title","source_title_time"]

logging:
  level: "info"
  json: false
```

------

## 8. 调度与并发设计（落实 per-source 频率）

### 8.1 调度策略

- 每个 source 启动一个 worker：
  - `ticker = poll_interval_seconds`
  - tick 到来时执行一次抓取（含重试），执行完成后等待下一个 tick
- 同源不并发：若一次抓取超过 interval，则下一个 tick 到来时跳过并记录 `missed_tick`

### 8.2 失败隔离

- 某个 source 连续失败不会影响其他 source 的 worker
- 每个 worker 内部有自己的超时与重试上限（<=10s, <=3）

------

## 9. 验收标准（AC）

1. 配置文件可新增/修改 source 的 URL、poll interval、timeout、重试、base_score
2. 任一 source 单次请求不会超过 10s；重试不超过 3 次
3. 不同 source 频率不同且互不阻塞（一个源卡住不影响其他源继续推送）
4. 72h 内同一消息不会重复推送（Redis 去重验证）
5. 推送频率不超过 `max_push_per_minute`
6. 日志可定位：抓取耗时、重试次数、解析条数、去重命中、推送结果与失败原因