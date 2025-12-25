# Alpha Trading 辩论竞技场模块 - 技术文档

**语言:** [English](DEBATE_MODULE.md) | [中文](DEBATE_MODULE.zh-CN.md)

## 概述

辩论竞技场是一个多 AI 协作决策系统，多个具有不同性格的 AI 模型对市场状况进行辩论并达成交易决策共识。系统支持多轮辩论、实时流推送、投票机制和自动交易执行。

---

## 1. 架构概览

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            辩论竞技场系统                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│  │  多头 AI    │    │  空头 AI    │    │  分析 AI   │    │  风控 AI    │  │
│  │     🐂      │    │     🐻      │    │     📊      │    │     🛡️      │  │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘    └──────┬──────┘  │
│         │                  │                  │                  │          │
│         └──────────────────┴──────────────────┴──────────────────┘          │
│                                    │                                        │
│                          ┌─────────▼─────────┐                              │
│                          │    辩论引擎       │                              │
│                          │  (debate/engine)  │                              │
│                          └─────────┬─────────┘                              │
│                                    │                                        │
│         ┌──────────────────────────┼──────────────────────────┐            │
│         │                          │                          │            │
│  ┌──────▼──────┐         ┌─────────▼─────────┐      ┌────────▼────────┐   │
│  │  市场数据   │         │   投票系统        │      │   自动执行器    │   │
│  │    组装     │         │   与共识机制      │      │   (可选)        │   │
│  └─────────────┘         └───────────────────┘      └─────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 文件结构

```
├── debate/
│   └── engine.go          # 核心辩论引擎逻辑
├── api/
│   └── debate.go          # HTTP 处理器和 SSE 流
├── store/
│   └── debate.go          # 数据库操作和模式
└── web/src/pages/
    └── DebateArenaPage.tsx # 前端 UI
```

---

## 2. 性格系统

### 2.1 可用性格

| 性格 | 图标 | 名称 | 描述 | 交易偏向 |
|------|------|------|------|----------|
| Bull | 🐂 | 激进多头 | 寻找做多机会 | 乐观，趋势跟随 |
| Bear | 🐻 | 谨慎空头 | 关注风险 | 悲观，做空偏向 |
| Analyst | 📊 | 数据分析师 | 纯数据驱动 | 无偏见，客观分析 |
| Contrarian | 🔄 | 逆势者 | 挑战多数观点 | 另类视角 |
| Risk Manager | 🛡️ | 风控经理 | 关注风险控制 | 仓位管理，止损 |

### 2.2 性格提示词增强

**文件位置:** `debate/engine.go:buildDebateSystemPrompt()` (365-426行)

```
## 辩论模式 - 第 {round}/{max_rounds} 轮

你作为 {emoji} {personality} 参与辩论。

### 你的辩论角色:
{personality_description}

### 辩论规则:
1. 分析所有候选币种
2. 用具体数据支持论点
3. 回应其他参与者 (第2轮起)
4. 有说服力但基于数据
5. 可以推荐多个不同操作的币种

### 输出格式 (严格 JSON):
<reasoning>
  - 带数据引用的市场分析
  - 主要交易论点
  - 对他人的回应 (第2轮起)
</reasoning>

<decision>
[
  {"symbol": "BTCUSDT", "action": "open_long", "confidence": 75, ...},
  {"symbol": "ETHUSDT", "action": "open_short", "confidence": 80, ...}
]
</decision>
```

---

## 3. 辩论执行流程

### 3.1 会话创建

```
POST /api/debates
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. 验证用户认证                                              │
│ 2. 解析 CreateDebateRequest:                                │
│    - name, strategy_id, symbol, max_rounds, participants    │
│    - interval_minutes, prompt_variant, auto_execute         │
│ 3. 验证策略所有权                                            │
│ 4. 自动选择币种 (如未提供):                                  │
│    - 静态币种 → 使用策略第一个币种                           │
│    - CoinPool → 从 AI500 API 获取                           │
│    - OI Top → 从 OI 排行 API 获取                           │
│    - Mixed → 先尝试池，回退到 OI                            │
│ 5. 设置默认值:                                               │
│    - max_rounds: 3 (范围 2-5)                               │
│    - interval_minutes: 5                                    │
│    - prompt_variant: "balanced"                             │
│ 6. 在数据库创建 DebateSession                               │
│ 7. 添加带 AI 模型和性格的参与者                              │
│ 8. 返回完整会话及参与者                                      │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 辩论轮次执行

**文件位置:** `debate/engine.go:runDebate()` (157-289行)

```
┌─────────────────────────────────────────────────────────────┐
│ 每轮 (1 到 max_rounds):                                     │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ 1. 广播 "round_start" 事件                              │ │
│ │ 2. 每个参与者 (按 speak_order):                         │ │
│ │    ┌─────────────────────────────────────────────────┐  │ │
│ │    │ a. 构建性格增强的系统提示词                      │  │ │
│ │    │ b. 构建用户提示词:                               │  │ │
│ │    │    - 市场数据 (来自策略引擎)                     │  │ │
│ │    │    - 之前的辩论消息 (第2轮起)                    │  │ │
│ │    │ c. 调用 AI 模型，60秒超时                        │  │ │
│ │    │ d. 从响应解析多币种决策                          │  │ │
│ │    │ e. 保存消息到数据库                              │  │ │
│ │    │ f. 广播 "message" 事件                           │  │ │
│ │    └─────────────────────────────────────────────────┘  │ │
│ │ 3. 广播 "round_end" 事件                                │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ 所有轮次后:                                                 │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ 1. 进入投票阶段 (status = "voting")                     │ │
│ │ 2. 收集所有参与者的最终投票                             │ │
│ │ 3. 确定多币种共识                                       │ │
│ │ 4. 存储最终决策                                         │ │
│ │ 5. 更新状态为 "completed"                               │ │
│ │ 6. 广播 "consensus" 事件                                │ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. 共识算法

### 4.1 投票收集

**文件位置:** `debate/engine.go:collectVotes()` (542-567行)

```
每个参与者:
┌─────────────────────────────────────────────────────────────┐
│ 1. 构建投票系统提示词                                        │
│ 2. 构建带辩论摘要的投票用户提示词                            │
│ 3. 调用 AI 模型获取最终投票                                  │
│ 4. 解析多币种决策                                            │
│ 5. 验证/修复币种与 session.Symbol 一致                       │
│ 6. 保存投票到数据库                                          │
│ 7. 广播 "vote" 事件                                          │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 多币种共识确定

**文件位置:** `debate/engine.go:determineMultiCoinConsensus()` (752-924行)

**算法:**

```
1. 收集所有投票中的所有币种决策
2. 按 symbol → action → 聚合数据 分组

3. 对每个投票决策:
   weight = confidence / 100.0
   累加:
   ┌─────────────────────────────────────────────────────────┐
   │ score += weight                                         │
   │ total_confidence += confidence                          │
   │ total_leverage += leverage                              │
   │ total_position_pct += position_pct                      │
   │ total_stop_loss += stop_loss                            │
   │ total_take_profit += take_profit                        │
   │ count++                                                 │
   └─────────────────────────────────────────────────────────┘

4. 对每个币种:
   找到胜出操作 (最高 score)
   计算平均值:
   ┌─────────────────────────────────────────────────────────┐
   │ avg_confidence = total_confidence / count               │
   │ avg_leverage = clamp(total_leverage / count, 1, 20)     │
   │ avg_position_pct = clamp(total_pct / count, 0.1, 1.0)   │
   │ avg_stop_loss = 默认 3% (如未设置)                       │
   │ avg_take_profit = 默认 6% (如未设置)                     │
   └─────────────────────────────────────────────────────────┘

5. 返回共识决策数组
```

### 4.3 共识示例

**输入投票:**
```
AI1 (多头):   BTC open_long  (conf=80, lev=10, pos=0.3)
AI2 (空头):   BTC open_short (conf=60, lev=5, pos=0.2)
AI3 (分析):   BTC open_long  (conf=70, lev=8, pos=0.25)
```

**计算:**
```
open_long:
  score = 0.80 + 0.70 = 1.50
  avg_conf = (80 + 70) / 2 = 75
  avg_lev = (10 + 8) / 2 = 9
  avg_pos = (0.3 + 0.25) / 2 = 0.275

open_short:
  score = 0.60
  avg_conf = 60
  avg_lev = 5
  avg_pos = 0.2

胜出: open_long (score 1.50 > 0.60)
```

**输出:**
```json
{
  "symbol": "BTCUSDT",
  "action": "open_long",
  "confidence": 75,
  "leverage": 9,
  "position_pct": 0.275,
  "stop_loss": 0.03,
  "take_profit": 0.06
}
```

---

## 5. 自动执行

### 5.1 执行流程

**文件位置:** `debate/engine.go:ExecuteConsensus()` (932-1052行)

```
POST /api/debates/:id/execute
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. 验证会话状态 = completed                                  │
│ 2. 验证 final_decision 存在且未执行                         │
│ 3. 验证操作是 open_long 或 open_short                       │
│ 4. 获取当前市场价格                                          │
│ 5. 获取账户余额:                                             │
│    - 尝试 available_balance                                  │
│    - 回退到 total_equity 或 wallet_balance                  │
│ 6. 计算仓位大小:                                             │
│    position_size_usd = available_balance × position_pct     │
│    (最小 $12 以满足交易所要求)                               │
│ 7. 计算止损和止盈价格:                                       │
│    ┌───────────────────────────────────────────────────┐    │
│    │ open_long:                                        │    │
│    │   SL = price × (1 - stop_loss_pct)               │    │
│    │   TP = price × (1 + take_profit_pct)             │    │
│    │ open_short:                                       │    │
│    │   SL = price × (1 + stop_loss_pct)               │    │
│    │   TP = price × (1 - take_profit_pct)             │    │
│    └───────────────────────────────────────────────────┘    │
│ 8. 创建 Decision 对象                                        │
│ 9. 调用 executor.ExecuteDecision()                          │
│ 10. 更新 final_decision:                                    │
│     - executed = true/false                                 │
│     - executed_at = 时间戳                                   │
│     - error 消息 (如失败)                                    │
└─────────────────────────────────────────────────────────────┘
```

---

## 6. API 接口

### 6.1 接口列表

| 接口 | 方法 | 描述 |
|------|------|------|
| `/api/debates` | GET | 列出用户所有辩论 |
| `/api/debates/personalities` | GET | 获取 AI 性格配置 |
| `/api/debates/:id` | GET | 获取辩论详情 |
| `/api/debates` | POST | 创建新辩论 |
| `/api/debates/:id/start` | POST | 开始辩论执行 |
| `/api/debates/:id/cancel` | POST | 取消运行中的辩论 |
| `/api/debates/:id/execute` | POST | 执行共识交易 |
| `/api/debates/:id` | DELETE | 删除辩论 |
| `/api/debates/:id/messages` | GET | 获取所有消息 |
| `/api/debates/:id/votes` | GET | 获取所有投票 |
| `/api/debates/:id/stream` | GET | SSE 实时流 |

### 6.2 创建辩论请求

```json
POST /api/debates
{
  "name": "BTC 市场辩论",
  "strategy_id": "strategy-uuid",
  "symbol": "BTCUSDT",
  "max_rounds": 3,
  "interval_minutes": 5,
  "prompt_variant": "balanced",
  "auto_execute": false,
  "trader_id": "trader-uuid",
  "enable_oi_ranking": true,
  "oi_ranking_limit": 10,
  "oi_duration": "1h",
  "participants": [
    {"ai_model_id": "deepseek-v3", "personality": "bull"},
    {"ai_model_id": "qwen-max", "personality": "bear"},
    {"ai_model_id": "gpt-5.2", "personality": "analyst"}
  ]
}
```

---

## 7. 实时更新 (SSE)

### 7.1 SSE 接口

**文件位置:** `api/debate.go:HandleDebateStream()` (407-453行)

```
GET /api/debates/:id/stream
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. 验证用户所有权                                            │
│ 2. 设置 SSE 头:                                              │
│    Content-Type: text/event-stream                          │
│    Cache-Control: no-cache                                  │
│    Connection: keep-alive                                   │
│ 3. 发送初始状态                                              │
│ 4. 订阅事件                                                  │
│ 5. 流式推送更新直到客户端断开                                │
└─────────────────────────────────────────────────────────────┘
```

### 7.2 事件类型

| 事件 | 触发时机 | 数据 |
|------|----------|------|
| `initial` | 连接开始 | 完整会话状态 |
| `round_start` | 轮次开始 | `{round, status}` |
| `message` | AI 发言 | DebateMessage 对象 |
| `round_end` | 轮次结束 | `{round, status}` |
| `vote` | AI 投票 | DebateVote 对象 |
| `consensus` | 辩论完成 | DebateDecision 对象 |
| `error` | 发生错误 | `{error: string}` |

---

## 8. 数据库模式

### 8.1 表结构

**debate_sessions:**
```sql
CREATE TABLE debate_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  strategy_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  symbol TEXT NOT NULL,
  max_rounds INTEGER DEFAULT 3,
  current_round INTEGER DEFAULT 0,
  interval_minutes INTEGER DEFAULT 5,
  prompt_variant TEXT DEFAULT 'balanced',
  final_decision TEXT,
  final_decisions TEXT,
  auto_execute BOOLEAN DEFAULT 0,
  trader_id TEXT,
  enable_oi_ranking BOOLEAN DEFAULT 0,
  oi_ranking_limit INTEGER DEFAULT 10,
  oi_duration TEXT DEFAULT '1h',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**debate_participants:**
```sql
CREATE TABLE debate_participants (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  ai_model_id TEXT NOT NULL,
  ai_model_name TEXT NOT NULL,
  provider TEXT NOT NULL,
  personality TEXT NOT NULL,
  color TEXT NOT NULL,
  speak_order INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (session_id) REFERENCES debate_sessions(id) ON DELETE CASCADE
);
```

**debate_messages:**
```sql
CREATE TABLE debate_messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  round INTEGER NOT NULL,
  ai_model_id TEXT NOT NULL,
  ai_model_name TEXT NOT NULL,
  provider TEXT NOT NULL,
  personality TEXT NOT NULL,
  message_type TEXT NOT NULL,
  content TEXT NOT NULL,
  decision TEXT,
  decisions TEXT,
  confidence INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (session_id) REFERENCES debate_sessions(id) ON DELETE CASCADE
);
```

**debate_votes:**
```sql
CREATE TABLE debate_votes (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  ai_model_id TEXT NOT NULL,
  ai_model_name TEXT NOT NULL,
  action TEXT NOT NULL,
  symbol TEXT NOT NULL,
  confidence INTEGER DEFAULT 0,
  leverage INTEGER DEFAULT 5,
  position_pct REAL DEFAULT 0.2,
  stop_loss_pct REAL DEFAULT 0.03,
  take_profit_pct REAL DEFAULT 0.06,
  reasoning TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (session_id) REFERENCES debate_sessions(id) ON DELETE CASCADE
);
```

---

## 9. 前端组件

### 9.1 页面结构

**文件位置:** `web/src/pages/DebateArenaPage.tsx`

```
DebateArenaPage
├── 左侧边栏 (w-56)
│   ├── 新建辩论按钮
│   ├── 辩论会话列表
│   │   └── SessionItem (状态, 名称, 时间戳)
│   └── 在线交易员列表
│       └── TraderItem (名称, 状态, AI 模型)
│
├── 主内容区
│   ├── 头部栏
│   │   ├── 会话信息 (名称, 状态, 币种)
│   │   ├── 参与者头像
│   │   └── 投票摘要
│   │
│   ├── 内容区 (双栏)
│   │   ├── 左: 讨论记录
│   │   │   ├── 轮次标题
│   │   │   └── MessageCards (可展开)
│   │   │
│   │   └── 右: 最终投票
│   │       └── VoteCards (操作, 置信度, 理由)
│   │
│   └── 共识栏
│       ├── 最终决策显示
│       └── 执行按钮 (如果 auto_execute 禁用)
│
└── 弹窗
    ├── CreateModal
    │   ├── 名称输入
    │   ├── 策略选择器
    │   ├── 币种输入 (自动填充)
    │   ├── 最大轮数选择器
    │   └── 参与者选择器 (AI 模型 + 性格)
    │
    └── ExecuteModal
        └── 交易员选择器
```

### 9.2 状态颜色

```typescript
const STATUS_COLOR = {
  pending: 'bg-gray-500',
  running: 'bg-blue-500 animate-pulse',
  voting: 'bg-yellow-500 animate-pulse',
  completed: 'bg-green-500',
  cancelled: 'bg-red-500',
}
```

### 9.3 操作样式

```typescript
const ACT = {
  open_long: {
    color: 'text-green-400',
    bg: 'bg-green-500/20',
    icon: <TrendingUp />,
    label: 'LONG'
  },
  open_short: {
    color: 'text-red-400',
    bg: 'bg-red-500/20',
    icon: <TrendingDown />,
    label: 'SHORT'
  },
  hold: {
    color: 'text-blue-400',
    bg: 'bg-blue-500/20',
    icon: <Minus />,
    label: 'HOLD'
  },
  wait: {
    color: 'text-gray-400',
    bg: 'bg-gray-500/20',
    icon: <Clock />,
    label: 'WAIT'
  },
}
```

---

## 10. 集成点

### 10.1 策略系统

辩论会话依赖保存的策略:
- **币种来源配置:** static/pool/OI top
- **市场数据指标:** K线、时间周期、技术指标
- **风控参数:** 杠杆限制、仓位大小
- **自定义提示词:** 角色定义、交易规则

### 10.2 AI 模型系统

每个参与者需要:
- AI 模型配置 (provider, API key, 自定义 URL)
- 支持的 providers: deepseek, qwen, openai, claude, gemini, grok, kimi
- 客户端初始化带超时处理 (每次调用 60s)

### 10.3 交易员系统

自动执行需要:
- 运行中状态的活跃交易员
- 交易员必须有有效的交易所连接
- 执行器接口: `ExecuteDecision()`, `GetBalance()`

---

## 总结

辩论竞技场模块提供了一个复杂的多 AI 协作决策系统:

- **多性格辩论:** 5 种独特的 AI 性格 (多头、空头、分析师、逆势者、风控经理)，具有独特的交易偏向
- **共识机制:** 基于置信度的加权投票来确定最终决策
- **实时更新:** SSE 流推送实时辩论进度
- **自动执行:** 可选的基于共识的自动交易执行
- **策略集成:** 与策略配置深度集成，用于市场数据和风控参数
- **多币种支持:** 能够同时分析和决策多个币种

该系统使用户能够利用多个 AI 视角做出更稳健的交易决策，同时保持对执行的完全控制。

---

**文档版本:** 1.0.0
**最后更新:** 2025-01-15
