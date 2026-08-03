# AGENTS.md

本文件适用于整个仓库，供在本项目中工作的自动化编码代理使用。若子目录以后出现更具体的 `AGENTS.md`，以离目标文件最近的说明为准。

## 项目概览

这是一个 Go workspace，采用 Employer–Worker 集群架构：Wails 桌面端负责账号、任务规划、调度、订单和支付提示，Worker 通过 gRPC + mTLS 执行任务；桌面前端使用 Vue、TypeScript、Vuetify 和 Vite。

主要目录：

- `lib/`：Bilibili API、通用模型、日志、通知、调度及基础工具。
- `cluster/`：领域模型、SQLite 存储、规划、分发、Employer 客户端和 Worker 服务。
- `captcha/`：跨平台验证码动态库加载与封装。
- `cmd/ticket-worker/`：独立 Worker CLI 入口。
- `cmd/gui/`：Wails v3 桌面端 Go 后端及服务层。
- `cmd/gui/frontend/`：Vue 前端。
- `cluster/worker/proto/`：Employer–Worker gRPC 协议及生成代码。
- `docs/`：集群设计和用户文档。

## 工作原则

- 修改前先阅读目标包、相邻测试和调用方，保持现有分层，不把 GUI、存储或网络逻辑混入通用库。
- 只改当前任务需要的文件；仓库可能已有用户未提交的改动，不要覆盖、还原或顺手格式化无关文件。
- 保持文件为 UTF-8。中文文档或文案出现终端乱码时，不要据此批量转码或重写。
- 不提交运行时数据、Cookie、令牌、证书、私钥、SQLite 数据库、日志、构建产物或 `node_modules`。这些内容通常位于 `data/`、`logs/`、`bin/` 和 `cmd/gui/frontend/node_modules/`。
- 网络请求、验证码、通知和真实购票流程不得成为单元测试的外部依赖；使用本地 HTTP/gRPC 测试服务、临时目录或可控替身。
- 错误必须保留上下文并向上传递；面向用户的错误优先沿用 `lib/global` 的 fault 体系，不要静默吞错。
- 涉及 goroutine、调度器或 Worker 生命周期时，明确处理 `context.Context`、停止信号、锁和资源清理，避免泄漏与重复执行。

## Go 约定

- 使用仓库声明的 Go 工具链（根 `go.work` 当前为 Go 1.26.4）；不要无任务依据升级 Go 版本或依赖。
- 所有改动过的 Go 文件必须经过 `gofmt`。遵循现有包名、错误包装、构造函数和测试风格。
- 新行为应在同包或相邻测试中覆盖；优先表驱动测试，使用 `t.TempDir()`、`t.Cleanup()` 和确定性输入。
- 平台相关实现延续现有 `//go:build` 拆分方式。不要为了让当前平台通过而破坏其他平台文件。
- 修改 SQLite schema 时，在 `cluster/storage` 中增加向前迁移并补迁移/仓储测试；不要改变已发布迁移的含义。

根目录本身不是 Go module，因此不要运行 `go test ./...`。从仓库根目录执行：

```powershell
go test ./lib/... ./cluster/... ./captcha/... ./cmd/ticket-worker/...
go test ./cmd/gui ./cmd/gui/bws_service/... ./cmd/gui/cluster_service/... ./cmd/gui/i18n/... ./cmd/gui/notify_service/... ./cmd/gui/payqr/... ./cmd/gui/store/...
```

按改动范围可先运行更小的包，例如：

```powershell
go test ./cluster/dispatcher ./cluster/planner
go test ./cmd/gui/cluster_service
```

GUI 的递归模式可能扫描 `frontend/node_modules` 中碰巧存在的 Go 包，所以不要用 `go test ./cmd/gui/...` 作为默认全量命令。

## 前端约定

- 前端目录为 `cmd/gui/frontend`，使用已提交的 `package-lock.json`；安装依赖优先运行 `npm ci`。
- 使用 Vue Composition API、TypeScript 和现有 Vuetify/Pinia/router 结构，避免引入新的状态或样式体系。
- 新增或修改用户可见文本时，同步维护 `src/locales/zh-CN.json` 与 `src/locales/en-US.json`；Go 侧国际化文案则检查 `cmd/gui/i18n/locales/`。
- 不手工编辑 `frontend/bindings/`；它由 Wails 生成且被忽略。`src/components.d.ts` 也应视为工具维护文件，除非任务明确要求处理生成结果。
- `npm run lint` 带有 `--fix`，会修改文件。只检查时使用 `npx eslint .`。

前端验证命令：

```powershell
Set-Location cmd/gui/frontend
npm run type-check
npx eslint .
npm run build
```

## Wails 与构建

在 `cmd/gui` 中使用 Wails 自带的 Task runner，无需单独安装 `task`：

```powershell
Set-Location cmd/gui
wails3 task dev
wails3 task build
```

- Go 服务新增、删除或更改暴露给前端的方法/模型后，运行 `wails3 task common:generate:bindings`，并确保前端类型检查通过。
- `wails3 task build` 可能执行 `go mod tidy`、生成 bindings、构建前端和平台资源；检查并仅保留任务需要的依赖文件变化。
- 平台打包、签名、Docker 和发布任务成本较高，除非任务直接涉及这些路径，否则不作为日常验证要求。

## Protobuf 协议

- 协议源文件是 `cluster/worker/proto/worker.proto`；不要直接编辑 `worker.pb.go` 或 `worker_grpc.pb.go`。
- 保持 wire compatibility：不要复用或重排已发布字段号；删除字段时保留/标记其编号，跨 Employer–Worker 的行为变化需要同时更新两端和相关测试。
- 在 `cluster/worker/proto` 中使用与生成文件头部一致的工具版本重新生成：

```powershell
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative worker.proto
```

当前生成版本为 `protoc 34.1`、`protoc-gen-go v1.36.11`、`protoc-gen-go-grpc v1.6.2`。生成后运行 `cluster/worker`、`cluster/employer` 及相关集成测试。

## 完成检查

提交结果前至少确认：

1. 改动范围与任务一致，没有覆盖用户已有修改。
2. Go 文件已 `gofmt`，相关 Go 测试通过。
3. 前端改动通过 type-check、非修改式 lint 和 build。
4. 协议、Wails API、数据库 schema 或国际化文案的配套文件已同步。
5. `git diff --check` 无空白错误，`git status --short` 中没有意外生成物或敏感数据。
6. 最终说明列出关键改动、实际运行的验证命令以及任何未验证项。
