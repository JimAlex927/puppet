# Puppet Pipeline MVP

一个轻量级 Pipeline 执行平台 MVP，主链路覆盖：

1. 创建 Project
2. 创建 Task
3. 配置带成功/失败出口的 Pipeline
4. 配置 Task 运行参数
5. 执行 Task
6. 生成 TaskRun / NodeRun
7. 实时写入日志并通过 SSE 推送
8. 查看历史执行记录和日志

## 技术栈

- Backend: Go, Gin, GORM, SQLite
- Frontend: Vue 3, TypeScript, Vite, Element Plus, Axios
- MVP Agent: local-agent

SQLite 驱动使用纯 Go 实现，Windows 下不依赖 cgo / gcc。

## 启动

开发模式可以分别启动后端和 Vite：

```powershell
go run ./cmd/server
```

默认 API 监听 `http://localhost:8080`，内嵌前端监听 `http://localhost:5173`，数据库文件位于 `data/puppet.db`。

默认管理员：

```text
username: puppetadmin
password: puppetadmin
```

开发前端：

```powershell
cd frontend
npm install
npm run dev
```

默认访问 `http://localhost:5173`，Vite 会把 `/api` 代理到后端。

## 打包为单个 exe

后端 exe 会嵌入 `frontend/dist`，并在运行时同时启动两个端口：

- `PUPPET_HTTP_ADDR`: API 端口，默认 `:8080`
- `PUPPET_FRONTEND_ADDR`: 前端页面端口，默认 `:5173`

构建顺序：

```powershell
cd frontend
npm install
npm run build
cd ..
go build -o puppet-server.exe ./cmd/server
```

启动：

```powershell
.\puppet-server.exe
```

访问 `http://localhost:5173`。这个前端端口内置 `/api` 反向代理，会转发到 `PUPPET_SERVER_URL`，默认 `http://localhost:8080`。

## 验证

```powershell
go test ./...
cd frontend
npm run build
```

## 目录结构

```text
cmd/server         后端入口
internal/api       HTTP 路由和 handler
internal/db        数据库迁移和种子数据
internal/model     数据模型
internal/project   Project 业务服务
internal/task      Task 业务服务和默认 Pipeline
internal/engine    线性 Pipeline 执行引擎
internal/node      节点接口、元数据和注册表
internal/nodes     内置 shell/http/sleep 节点
internal/confignode 运行配置节点接口和注册表
internal/confignodes 内置动态配置节点
internal/logstream SSE 实时事件 Hub
internal/agent     local-agent 查询服务
internal/variable  变量能力预留
internal/credential 凭据能力预留
internal/artifact  制品能力预留
internal/event     事件能力预留
frontend           Vue 管理台
```

## MVP 内置节点

- `shell`: 执行本机 shell 脚本，实时写入 stdout/stderr
- `sleep`: 等待指定秒数
- `http`: 发送 HTTP 请求，2xx 判定成功
- `git`: Git Checkout，只负责把指定仓库和 ref 检出到 workspace；支持 HTTPS 和 SSH 两种 transport
- `process`: 启动或停止进程；启动时记录 PID 和进程身份 metadata，停止时校验身份后再停止，降低 PID 复用误杀风险

## Task 运行配置

Task 支持类似 Jenkins `Build with Parameters` 的运行配置。点击执行时前端会先请求 `GET /api/tasks/:id/run-config`，弹出运行配置窗口，用户确认后再调用 `POST /api/tasks/:id/run`。

当前参数类型：

- `string`: 静态可编辑文本
- `select`: 下拉选择，可以配置静态选项，也可以绑定动态配置节点

动态参数通过 `configNodes` 实现，和真正执行 Pipeline 的 `nodes` 分离。比如给 Task 增加一个 `version` 下拉参数，然后绑定 `Git Branches` 配置节点；每次打开运行窗口时都会重新执行该配置节点，实时读取仓库分支，输出到 `version` 的候选列表。

节点参数可以引用运行输入：

```text
${input.version}
${version}
```

节点参数也可以引用前面节点的输出：

```text
${node.startProcess.metadataPath}
${node.startProcess.pid}
```

执行前引擎会把这些占位符替换成用户在运行弹窗里选择或填写的值，并把替换后的参数写入 NodeRun 快照。

内置配置节点：

- `git_branches`: 执行 `git ls-remote --heads` 获取远程分支列表，支持凭据中心里的 HTTPS username/password、HTTPS token 和 SSH key

## Pipeline 跳转模型

节点现在支持两个出口：

- `nextNodeId`: 当前节点成功后跳转的节点 ID
- `fallbackNodeId`: 当前节点失败后跳转的节点 ID

Pipeline 从 `startNodeId` 开始执行。成功且没有 `nextNodeId` 时 Pipeline 结束为成功；失败且没有 `fallbackNodeId` 时 Pipeline 结束为失败。执行器会校验引用的节点是否存在，并检测循环，避免无限执行。

旧版没有出口字段的 Pipeline 会继续按数组顺序执行，便于兼容历史数据。

## Git Checkout 节点

Git 节点的边界是 checkout，不做 `commit`、`push`、`tag`、`merge`、`rebase` 等操作。其它 Git 操作可以先用 `shell` 节点，后续如有必要再做单独节点。

节点必须显式配置 `transport`：

- `https`: 仓库地址必须是 `https://...`，凭据可选 `token` 或 `username_password`
- `ssh`: 仓库地址必须是 `git@host:path` 或 `ssh://...`，凭据可选 `ssh_key`

Git 节点不把账号、密码、token 放在 Pipeline JSON 里，而是引用凭据中心里的 `credentialId`。

凭据类型：

- `username_password`: HTTPS username/password
- `token`: HTTPS token，默认 username 为 `x-access-token`
- `ssh_key`: SSH private key

Git 节点执行时会打印 checkout plan、脱敏后的 git 命令、checkout 结果，包括 commit、branch、author、message。HTTPS 凭据通过临时 `GIT_ASKPASS` 注入，SSH key 通过临时 key 文件和 `GIT_SSH_COMMAND` 注入，执行结束后会清理临时文件。

工作区策略由 `workspacePolicy` 控制：

- `fail_if_dirty`: 默认策略。已有仓库且存在未提交或未跟踪文件时直接失败
- `reset_and_clean`: 对已有仓库执行 `git reset --hard` 和 `git clean -fdx`，再 fetch/checkout
- `wipe_and_clone`: 删除 checkoutDir 后重新 clone
- `reuse`: 不主动清理，直接尝试 fetch/checkout，可能因为脏工作区失败

凭据 secret 会以 AES-GCM 加密 blob 存入 SQLite。开发环境如果未设置 `PUPPET_SECRET_KEY` 会使用本地默认密钥；正式使用请设置稳定且保密的 `PUPPET_SECRET_KEY`，否则更换密钥后历史凭据无法解密。

## Process 节点

Process 节点用于管理长期运行的本机进程，第一版支持 Windows 和 Linux。节点拆成两个：

- `Process Start`: 使用 Go `exec.Command` 直接启动进程，不通过 `cmd start`
- `Process Stop`: 停止进程
- `workdir`: 只表示进程启动时的工作目录，默认 `${workspace}`
- 启动后会在当前 TaskRun 的 `${workspace}/processes` 下写入 metadata，例如 `data/workspaces/taskrun-1/processes/app.json`
- 真实系统进程名由平台从 `executable` 推导并写入 metadata，不需要用户配置
- NodeRun 输出包含 `pid`、`metadataPath`、`stdoutLog`、`stderrLog`
- `Process Stop` 通过 `Stop By` 下拉框选择一种停止方式：`metadata` 或 `port`
- `metadata` 方式会先校验 PID 当前对应的进程名、可执行路径、启动时间，再停止进程

Windows 停止进程使用 `taskkill`，进程身份来自 `Get-CimInstance Win32_Process`。

Linux 停止进程使用 `SIGTERM` / `SIGKILL`，进程身份来自 `/proc/<pid>/comm`、`/proc/<pid>/exe`、`/proc/<pid>/cmdline`、`/proc/<pid>/stat` 和 kernel `boot_id`。

推荐 Pipeline 用法：

1. 添加 `Process Start` 节点，ID 例如 `startApp`
2. 添加 `Process Stop` 节点，`Stop By` 选择 `metadata`，`metadataPath` 填：

```text
${node.startApp.metadataPath}
```

如果是同一个 TaskRun 里停止前面启动的应用，可以让 `metadataPath` 留空，并保持 `name` 一致；节点会默认读取：

```text
${workspace}/processes/<name>.json
```

如果是跨 TaskRun 停止同一个应用，`${workspace}` 每次都不同，应该显式填写一个固定 `metadataPath`，例如：

```text
C:\apps\my-app\.puppet\app.json
```

`metadataPath` 也可以填写目录；如果填的是目录，节点会自动使用：

```text
<metadataPath>/<name>.json
```

例如 `metadataPath=C:\apps\my-app\.puppet` 且 `name=nats-server`，实际文件是 `C:\apps\my-app\.puppet\nats-server.json`。

启动前如果发现同名进程或端口已被占用，`ifAlreadyRunning` 支持：

- `fail`: 直接失败
- `stop`: 先停止匹配进程再启动
- `allow`: 不处理，继续启动

如果没有 metadata，也可以把 `Stop By` 选择为 `port`，按端口停止正在监听该端口的进程；这种方式适合清理场景，但没有 metadata 校验那么安全。

`Process Stop` 的停止方式：

- `metadata`: 推荐方式。配置 `name` 和 `metadataPath`。可以校验 PID 身份，避免 PID 复用误杀
- `port`: 清理方式。停止监听指定端口的进程，没有 metadata 身份校验

## 环境变量

- `PUPPET_HTTP_ADDR`: 后端监听地址，默认 `:8080`
- `PUPPET_SERVER_URL`: Agent 回调 Server 的地址，默认 `http://localhost:8080`
- `PUPPET_DATA_DIR`: 数据目录，默认 `data`
- `PUPPET_DATABASE_DSN`: SQLite 文件路径，默认 `data/puppet.db`
- `PUPPET_WORKSPACE_DIR`: 执行工作区目录，默认 `data/workspaces`
- `PUPPET_SECRET_KEY`: 凭据加密密钥，正式使用必须设置

## 后续扩展位置

第一版实现 local-agent 和 HTTP remote-agent。Server 主动调用 Agent 的 `endpointUrl` 执行节点，所以网络上必须满足 Server 能访问 Agent。节点扩展、变量、凭据、制品、事件总线都已保留独立目录，后续可以在不重写主链路的前提下扩展。

## Remote Agent

在前端 Agents 页面创建 Agent，填写：

- `name`: Agent 名称
- `endpointUrl`: 例如 `http://agent-host:9090`
- `labels`: 例如 `linux,docker`

创建后会显示一次性 token。到目标机器启动 Agent：

```powershell
go run ./cmd/agent --listen :9090 --server http://SERVER_IP:8080 --token <AGENT_TOKEN> --workspace agent-workspaces
```

Agent 会：

- 暴露 `POST /api/agent/execute-node`
- 定期向 Server 发送 heartbeat
- 执行 Server 派发的节点
- 回调 Server 写入实时日志

Pipeline 通过 `agentSelector.labels` 选择 Agent。比如：

```json
{
  "agentSelector": {
    "labels": ["linux", "docker"]
  }
}
```
