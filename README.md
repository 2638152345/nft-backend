# NFT Auction Backend 

一个独立的 Go 后端项目，作为 NFT 拍卖的数据桥梁：
- 监听链上拍卖合约事件并同步到 MySQL
- 向前端提供拍卖列表、出价历史、统计等 REST API
- 集成 Alchemy NFT API 查询钱包 NFT 列表（带短缓存）



## 1. 技术栈

- Go 1.22
- Gin (HTTP API)
- GORM + MySQL 8
- go-ethereum (RPC / event logs)
- Alchemy NFT API

## 2. 功能清单

- 全量历史回填 + 实时轮询同步以下事件：
  - `AuctionCreated`
  - `AuctionBid`
  - `AuctionEnded`
  - `AuctionCancelled`
- 拍卖列表查询（分页、排序、过滤）
- 某拍卖出价历史查询
- 平台统计（拍卖总数、出价总数）
- 钱包 NFT 列表查询（Alchemy + 60 秒缓存）
- 同步状态查询（处理进度、滞后区块）
- 同步可靠性：断点续传、幂等去重、重试、重组缓冲

## 3. 项目结构

```text
backend/
├── cmd/server/main.go                 # 程序入口
├── internal/
│   ├── api/                           # Gin 路由与 handler
│   ├── config/                        # 配置读取与校验
│   ├── db/                            # MySQL 与迁移执行
│   ├── indexer/                       # 事件 ABI 解码 + 同步器
│   ├── repository/                    # 数据库访问层
│   ├── service/                       # 业务服务层
│   ├── model/                         # DB 模型定义
│   └── alchemy/                       # Alchemy 客户端
├── migrations/001_init.sql            # 数据表与索引
├── .env.example
├── go.mod
└── README.md
```

## 4. 数据库设计

迁移文件：`migrations/001_init.sql`

主要表：
- `auctions`
  - 拍卖主表（状态、最高价、卖家、NFT 信息、时间）
- `auction_bids`
  - 出价历史（含 `tx_hash + log_index` 唯一键）
- `processed_logs`
  - 已处理日志去重表（幂等核心）
- `sync_state`
  - 同步断点状态（重启恢复核心）
- `wallet_nft_cache`
  - 钱包 NFT 接口缓存（减少第三方 API 压力）

## 5. 同步可靠性设计

### 同步流程

1. 读取 `sync_state.last_processed_block`
2. 计算起始区块：`max(AUCTION_DEPLOY_BLOCK, last_processed_block - REORG_BUFFER)`
3. 计算安全头：`latest - CONFIRMATIONS`
4. 按 `BLOCK_CHUNK_SIZE` 分段回填 `[start, safe_head]`
5. 回填完成后进入轮询，每 `POLL_INTERVAL_SEC` 秒继续同步

### 稳定性策略

- 幂等：`processed_logs(tx_hash, log_index)` 去重
- 容错：RPC/DB 失败自动重试（指数退避，上限 30 秒）
- 重启恢复：从断点继续，不丢历史
- 轻微重组防护：每次回退 `REORG_BUFFER` 区块重扫
- 仅处理确认后区块：避免未确认块导致脏数据

## 6. API 文档

统一前缀：`/api/v1`

### 6.1 获取拍卖列表

`GET /auctions`

Query 参数：
- `page` 默认 `1`
- `size` 默认 `20`，最大 `100`
- `sort_by`: `created_time | highest_bid | end_time`，默认 `created_time`
- `order`: `asc | desc`，默认 `desc`
- `status`: `active | ended | cancelled`（可选）
- `seller`（可选）
- `nft_address`（可选）

### 6.2 获取拍卖出价历史

`GET /auctions/:id/bids`

Query 参数：
- `page` 默认 `1`
- `size` 默认 `20`
- `order`: `asc | desc`，默认 `desc`

### 6.3 获取平台统计

`GET /stats`

返回字段：
- `total_auctions`
- `total_bids`

### 6.4 获取钱包 NFT 列表

`GET /wallets/:address/nfts`

Query 参数：
- `page` 默认 `1`
- `size` 默认 `20`

行为：
- page=1 优先读本地缓存（默认 TTL 60 秒）
- 缓存过期或无缓存时请求 Alchemy

### 6.5 获取同步状态

`GET /sync/status`

返回字段：
- `last_processed_block`
- `safe_head`
- `lag`
- `contract_address`
- `deploy_block`

### 6.6 响应格式

成功：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {}
}
```

失败：

```json
{
  "code": 400,
  "msg": "invalid sort_by"
}
```

## 7. 环境变量

先复制：

```bash
cp .env.example .env
```

关键变量说明：
- `MYSQL_DSN`：MySQL 连接串
- `RPC_HTTP_URL`：链上 RPC URL（建议使用 Alchemy/Infura）
- `AUCTION_CONTRACT_ADDRESS`：拍卖合约地址
- `AUCTION_DEPLOY_BLOCK`：合约部署区块（强烈建议填写正确值）
- `ALCHEMY_API_KEY`：Alchemy API Key

其他变量可直接使用默认值，见 `.env.example`。

## 8. 本地运行

在 `backend` 目录执行：

```bash
go mod tidy
go run ./cmd/server
```

默认端口：`8080`

健康检查：

```bash
curl http://127.0.0.1:8080/healthz
```

## 9. 常用调试命令

```bash
go test ./...
```

```bash
curl "http://127.0.0.1:8080/api/v1/auctions?page=1&size=20"
```

```bash
curl "http://127.0.0.1:8080/api/v1/sync/status"
```

## 10. 说明与边界

- `start_time` 按当前链上事件所在区块时间推导（当前合约是“立即开始”语义）
- 金额字段按链上最小单位存储与返回（字符串，避免前端精度丢失）
- 当前版本未加入鉴权，默认用于作业演示环境

## 11. 安全提醒

- 本服务不需要私钥，不要在后端环境变量中保存热钱包私钥
- 如果私钥曾经泄露，必须立即在钱包侧轮换并废弃旧私钥
