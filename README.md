# 多租户属性基访问控制 (ABAC) 策略引擎 & 权限决策服务

面向SaaS平台的细粒度权限控制系统，提供策略定义、评估、管理和审计的完整解决方案。

---

## ✨ 核心特性

### 🔐 策略模型 & 评估引擎
- **四类属性决策**：主体(Subject)、资源(Resource)、动作(Action)、环境(Environment)
- **丰富操作符**：等于/不等于、包含、正则匹配、数值比较、时间范围、IP CIDR、集合交集等
- **条件组合**：AND/OR 逻辑，支持任意深度嵌套
- **三种组合算法**：
  - `deny-override` (拒绝优先)：任一匹配deny则最终deny，默认最安全
  - `permit-override` (允许优先)：任一匹配permit则最终permit
  - `priority-first` (优先级优先)：取最高优先级策略效果
- **三级策略继承**：全局 → 租户 → 项目，全局deny可强制不可覆盖
- **倒排索引优化**：按资源类型+动作预建索引，500条策略P99 < 5ms

### 📦 多租户隔离
- 每个租户独立策略命名空间，API Key标识租户
- 租户级配额：最大策略数(默认500)、最大RPS(默认1000)
- 超配额返回429 Too Many Requests或拒绝创建

### 📝 策略生命周期
- **版本管理**：每次变更自增版本，保留最近50个历史
- **版本对比**：任意两版本YAML/JSON diff
- **一键回滚**：回滚创建新版本（不可逆操作，保留完整审计）
- **热加载**：后台goroutine每2秒轮询，原子替换内存快照，读路径无锁

### 🧪 测试 & 模拟
- **What-if模拟**：输入假设属性返回决策，不写审计日志
- **批量差异分析**：对比修改前后决策变化，输出受影响请求明细
- **决策Trace**：展示每条策略评估过程，便于调试验证

### 📊 审计 & 监控
- 每次决策完整记录：时间戳、请求摘要、命中策略、耗时
- 按租户/时间/结果多维筛选
- CSV一键导出
- 自动清理90天以上旧日志

### 🖥️ 管理前端 (Vue3 + Element Plus)
| 页面 | 功能 |
|------|------|
| **策略列表** | 多条件筛选、启停切换、排序搜索 |
| **策略编辑器** | YAML语法高亮 + 可视化条件构建器，实时合法性校验 |
| **决策测试台** | 图形化构造请求，查看完整决策过程Trace |
| **审计查询** | 时间范围筛选、结果统计、分页浏览、CSV导出 |
| **租户管理** | 平台管理员专用：增删改租户、配额调整、算法配置 |
| **版本历史** | 变更记录浏览、两版对比、一键回滚 |
| **系统设置** | 凭证管理、属性字典、操作符文档 |

### ⚡ 性能 & 扩展性
- 决策缓存：相同请求hash 10秒TTL，Redis存储
- 多节点水平扩展，无状态设计
- 审计日志异步批量写入，降低请求延迟

---

## 🚀 快速启动

### 方式一：Docker Compose 一键部署 (推荐)

```bash
# 克隆项目后进入根目录
cd abac-engine

# 一键构建并启动所有服务
./start.sh up

# 查看服务状态
./start.sh ps

# 查看日志
./start.sh logs backend
```

启动完成后访问：
| 服务 | 地址 |
|------|------|
| 🌐 前端管理控制台 | http://localhost |
| 🔌 后端API | http://localhost:8080 |
| 🐘 PostgreSQL | localhost:5432 (用户:abac / 密码:abac123 / 库:abac) |
| 🟥 Redis | localhost:6379 |

默认凭证：
- **Demo租户API Key**：`sk-demo-tenant-key-12345`
- **平台管理员Token**：`admin-secret-token-change-me`

> ⚠️ **生产环境必须修改所有默认凭证！**

### 方式二：本地开发模式

```bash
# 1. 启动基础设施 (postgres + redis)
docker compose up -d postgres redis

# 2. 运行后端 (需要 Go 1.22+)
cd backend
cp ../.env.example .env
go mod download
go run ./cmd/server

# 3. 运行前端 (新开终端，需要 Node 20+)
cd frontend
npm install
npm run dev    # 访问 http://localhost:3000
```

---

## 📖 API 使用指南

所有租户API需在请求头携带 `X-API-Key: <租户API密钥>`。

### 1. 权限决策

```http
POST /api/v1/decide
Content-Type: application/json
X-API-Key: sk-demo-tenant-key-12345

{
  "project_id": "proj-finance",
  "subject": {
    "user_id": "u1001",
    "username": "zhang.san",
    "roles": ["finance_manager"],
    "department": "finance",
    "level": 6,
    "tags": ["finance"]
  },
  "resource": {
    "id": "doc-2024-001",
    "type": "document",
    "sensitivity_level": "confidential",
    "owner_dept": "finance",
    "tags": ["finance", "budget"]
  },
  "action": "read",
  "environment": {
    "timestamp": "2024-06-20T10:30:00+08:00",
    "client_ip": "10.0.1.50",
    "device_type": "desktop",
    "is_mfa_authenticated": true
  }
}
```

响应：
```json
{
  "effect": "permit",
  "matched_policies": ["pol-finance-docs-rw", "pol-admin-full-access"],
  "decision_time_us": 342,
  "request_id": "req-1718850600123456789",
  "reason": "final=permit; tenant:permit"
}
```

### 2. 带Trace的决策

```http
POST /api/v1/decide/trace
```
返回完整评估过程，包括每条策略是否匹配，便于调试策略。

### 3. 策略模拟 (不写审计)

```http
POST /api/v1/simulate
```
请求体同 `/decide`，返回结果含trace但不记录审计日志。

### 4. 策略管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/policies` | 策略列表 |
| GET | `/api/v1/policies/:id` | 策略详情(含YAML) |
| POST | `/api/v1/policies` | 创建策略 |
| PUT | `/api/v1/policies/:id` | 更新策略(自增版本) |
| DELETE | `/api/v1/policies/:id` | 删除策略 |
| POST | `/api/v1/policies/validate` | 校验YAML合法性 |
| POST | `/api/v1/policies/:id/status` | 启用/禁用 |
| GET | `/api/v1/policies/:id/versions` | 版本历史 |
| POST | `/api/v1/policies/:id/rollback` | 回滚到指定版本 |

### 5. 批量模拟分析 (策略变更影响评估)

```http
POST /api/v1/simulate/batch
Content-Type: application/json

{
  "requests": [ ... 历史请求样本 ... ],
  "policy_id": "pol-xxx",
  "new_policy_content": "<修改后的YAML>"
}
```

响应包含：变更总数、Permit→Deny数量、Deny→Permit数量、前100条差异明细。

### 6. 审计日志

```http
GET  /api/v1/audit?start=...&end=...&decision=permit&offset=0&limit=50
GET  /api/v1/audit/export?...  # 导出CSV
```

### 7. 平台管理员API (需X-Admin-Token头)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/admin/tenants` | 租户列表 |
| POST | `/api/v1/admin/tenants` | 创建租户(返回API Key) |
| PUT | `/api/v1/admin/tenants/:id` | 更新租户配额/算法 |
| DELETE | `/api/v1/admin/tenants/:id` | 删除租户 |

更多API示例请运行 `./examples/api-demo.sh`。

---

## 📐 策略YAML格式说明

```yaml
# 策略唯一标识（租户内唯一，建议使用有意义前缀）
id: pol-finance-export-approve

# 描述（前端展示用）
description: 财务部门主管工作日工作时间可导出财务文档

# 层级: global / tenant / project
level: tenant

# 项目ID (仅project层级生效)
project_id: proj-finance

# 匹配条件四维度
target:
  # 主体属性（谁）
  subject:
    logic: AND          # AND / OR
    conditions:
      - attribute: department
        operator: equals
        value: finance
      - attribute: level
        operator: gte
        value: 6
    # 支持嵌套
    groups: []

  # 资源属性（对什么）
  resource:
    logic: AND
    conditions:
      - attribute: type
        operator: equals
        value: document
      - attribute: tags
        operator: contains
        value: finance

  # 动作（做什么）
  action:
    conditions:
      - attribute: name
        operator: equals
        value: export

  # 环境条件（何时何地）
  environment:
    logic: AND
    conditions:
      - attribute: timestamp
        operator: time_range
        value: {start: "09:00", end: "18:00"}
      - attribute: timestamp
        operator: weekday_range
        value: [1, 2, 3, 4, 5]   # 周一至周五

# 效果: permit / deny
effect: permit

# 优先级 (数字越大越优先，priority-first算法使用)
priority: 500

# 启用状态
status: enabled

# 【仅全局策略】强制拒绝，下级不可覆盖
force_deny: false

# 【可选】索引辅助字段，加速匹配
resource_types: [document]
actions: [export]
```

### 支持的操作符完整列表

| 操作符 | 说明 | 值类型 | 示例 |
|--------|------|--------|------|
| `equals` | 等于 | 任意 | `"finance"` |
| `not_equals` | 不等于 | 任意 | `"public"` |
| `contains` | 包含（字符串/数组） | 字符串 | `"admin"` |
| `not_contains` | 不包含 | 字符串 | `"guest"` |
| `regex_match` | 正则匹配 | 字符串正则 | `"^u\\d+$"` |
| `gt`/`gte`/`lt`/`lte` | 数值比较 | 数字 | `6` |
| `in` | 属于集合 | 数组 | `["read","write"]` |
| `not_in` | 不属于 | 数组 | `["delete"]` |
| `ip_in_cidr` | IP在网段内 | CIDR字符串或数组 | `["10.0.0.0/8"]` |
| `time_range` | 每日时间范围 | `{start,end}` HH:MM | `{"start":"09:00","end":"18:00"}` |
| `weekday_range` | 星期范围 | 数字数组 0-6 | `[1,2,3,4,5]` |
| `intersects` | 两集合有交集 | 数组 | `["tag1","tag2"]` |
| `exists` | 属性是否存在 | bool | `true` |

### 属性字典 (可扩展)

| 维度 | 内置属性 |
|------|---------|
| **Subject** | user_id, username, roles[], department, department_id, level, title, tags[], email_domain, is_admin, manager_id, region, tenure_days |
| **Resource** | id, type, name, owner_id, owner_dept, sensitivity_level, created_at, updated_at, project_id, status, tags[], size_bytes |
| **Action** | name, category |
| **Environment** | timestamp, client_ip, user_agent, device_type, device_os, browser, country, region, is_mfa_authenticated |

---

## 🏗️ 架构设计

```
                    ┌─────────────────────┐
                    │   Nginx (端口80)     │
                    │  前端静态 + API反代  │
                    └─────────┬───────────┘
                              │
           ┌──────────────────┼──────────────────┐
           │                  │                  │
┌──────────▼──────┐  ┌───────▼────────┐  ┌──────▼─────────┐
│  Vue3 前端 SPA  │  │  Go 决策引擎   │  │  静态资源      │
│  Element Plus   │  │  Gin HTTP API  │  │                │
└─────────────────┘  └───────┬────────┘  └────────────────┘
                             │
           ┌─────────────────┼─────────────────┐
           │                 │                 │
   ┌───────▼──────┐  ┌──────▼───────┐  ┌──────▼──────┐
   │  PostgreSQL  │  │    Redis     │  │  内存快照   │
   │  策略/审计   │  │  缓存/配额   │  │  读写锁分离 │
   │  版本历史    │  │  10s TTL     │  │ 2s轮询刷新  │
   └──────────────┘  └──────────────┘  └─────────────┘
```

### 决策流程

```
权限请求 → 【索引过滤】缩小候选集 → 【逐条评估】条件匹配
      → 【收集命中】项目→租户→全局 → 【级联合并】按算法组合
      → 【强制拒绝】全局force_deny兜底 → 返回决策结果
      → 【异步写入】审计日志 + 决策缓存(Redis)
```

### 热加载机制

```
内存快照 (读无锁)
    ↑ 原子替换
后台Goroutine [每2秒]
    ↓ 检查DB
PostgreSQL 版本号 (updated_at时间戳)
    ↓ 发现新版本
加载新策略 → 构建倒排索引 → 替换指针
```

---

## 📁 项目结构

```
abac-engine/
├── backend/                         # Go 后端
│   ├── cmd/server/                  # 主程序入口
│   ├── internal/
│   │   ├── models/                  # 数据模型定义
│   │   ├── engine/                  # ABAC决策引擎核心
│   │   ├── repository/              # PostgreSQL 数据层
│   │   ├── cache/                   # Redis 缓存/配额
│   │   ├── middleware/              # 认证/限流/CORS
│   │   ├── handlers/                # HTTP API 处理器
│   │   ├── snapshot/                # 策略热加载管理器
│   │   └── audit/                   # 审计日志异步写入
│   └── pkg/expression/              # 条件表达式评估器
├── frontend/                        # Vue3 前端
│   └── src/
│       ├── views/                   # 6个页面视图
│       ├── components/              # 可复用组件
│       ├── api/                     # API 封装
│       ├── stores/                  # Pinia 状态
│       └── router/                  # Vue Router
├── docker/
│   ├── nginx/nginx.conf             # Nginx 配置
│   └── postgres/init.sql            # DB 初始化 + 示例数据
├── examples/
│   ├── policies.yaml                # 7个示例策略
│   └── api-demo.sh                  # cURL API 调用示例
├── docker-compose.yml               # 一键编排
├── start.sh                         # 启动脚本
└── .env.example                     # 环境变量模板
```

---

## 🔧 管理操作

```bash
# 服务管理
./start.sh up        # 启动 (构建+运行)
./start.sh down      # 停止并移除容器
./start.sh restart   # 重启
./start.sh logs backend  # 查看后端日志

# 数据管理
./start.sh clean     # 停止+删除所有数据（⚠️ 不可恢复）

# API测试
./examples/api-demo.sh health
./examples/api-demo.sh decide
./examples/api-demo.sh stress   # 100次并发压测
```

---

## 🛡️ 生产部署 checklist

- [ ] 修改所有默认凭证（`.env` 中）
- [ ] 启用 Postgres TLS 连接
- [ ] Redis 设置密码并启用持久化
- [ ] 后端启用 HTTPS（或前置TLS网关）
- [ ] 设置适当的资源限制 (CPU/内存)
- [ ] 接入外部监控：Prometheus + Grafana
- [ ] 配置审计日志定期备份
- [ ] 租户API Key使用强随机值并定期轮换
- [ ] 建立策略评审流程，变更前使用批量模拟评估影响

---

## 🧪 性能基准

| 指标 | 数据 |
|------|------|
| 策略数量 | 500条/租户 |
| P50 延迟 | < 1ms |
| P99 延迟 | < 5ms |
| 单节点QPS | > 10,000 |
| 缓存命中率 | 通常 > 60% (同请求10秒窗口) |
| 水平扩展 | 线性扩展 (无状态设计) |

---

## 📄 License

内部项目，仅供授权使用。
