# services/api ｜ Go Application API

确定性业务系统（TECH_SPEC §3.2）：auth、project、evidence 元数据、audit run、finding、review、fix task、评分、retest。

## 运行

```bash
go run ./cmd/api        # :8080；需 DATABASE_URL/OIDC_*（见根 .env.example）
go run ./cmd/migrate up # 迁移（ADR-0003）；status / down-all（带守卫）同命令
```

## 探针

- `GET /healthz` — 进程存活，永不触碰基础设施；
- `GET /readyz` — 依赖可用才 200；缺 DATABASE_URL 时 503 并指名变量。

## 认证与租户（B05）

- **身份**：OIDC（RS256 JWT）。后端校验 issuer / audience / expiry / signature（TECH_SPEC §12），签名键来自 JWKS（缓存 15 分钟，未知 kid 触发刷新）；
- **租户来源**：token 只证明身份（sub/email）。organization 与 role 从本库 `users` 表按 email 加载——租户变更无需等 token 刷新，也杜绝了伪造 claim；
- **隔离**：所有项目查询带 `organization_id = actor.OrganizationID`（中间件注入，非请求体）；跨租户读取与不存在同答案（404），防资源枚举（OWASP API1）；
- **审计**：`audit_log` 只追加（actor / action / resource / request_id / before/after），无更新删除代码路径；
- **本地测试身份**：`ENV=dev` 且 `ARRIVAL_ENABLE_TEST_IDENTITY=1` 且密钥 ≥32 字节三者同时满足才可用（HS256，issuer `arrivalready-test`）。生产环境（或 ENV 非 dev）下构造即报错退出——真实数据路径禁止绕过；
- **未配置时 fail closed**：受保护路由 503 并说明原因，绝不静默放行。

## 测试

```bash
go test ./...   # 单测始终跑；集成测试需 TEST_DATABASE_URL 指向一次性数据库
```

集成测试（`internal/store`）对真实 PostgreSQL 执行「空库 up → down 全部 → 再 up」并验证两组织隔离；CI 用 postgres:18 service 容器跑同一套（.github/workflows/ci.yml）。本地无 Docker 时可用本机 PostgreSQL 建 一次性集群：

```powershell
& 'C:\Program Files\PostgreSQL\17\bin\initdb.exe' -D "$env:TEMP\arrival-pg\data" -A trust -U postgres
& 'C:\Program Files\PostgreSQL\17\bin\pg_ctl.exe' -D "$env:TEMP\arrival-pg\data" -o '-p 54329' start
$env:TEST_DATABASE_URL='postgres://postgres@127.0.0.1:54329/postgres?sslmode=disable'
```
