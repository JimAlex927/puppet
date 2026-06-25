# Backend README

后端代码位于项目根目录的 `cmd/` 和 `internal/` 下。

## 启动

```powershell
go run ./cmd/server
```

启动时会自动：

- 创建 SQLite 数据库和数据表
- 注册 `shell`、`sleep`、`http`、`git` 四个内置节点
- 创建或更新 `local-agent`
- 创建默认管理员 `puppetadmin / puppetadmin`
- 初始化一个 Demo Project 和 Demo Task

## 核心接口

- `GET /api/projects`
- `POST /api/projects`
- `GET /api/projects/:id/tasks`
- `POST /api/projects/:id/tasks`
- `GET /api/tasks/:id/pipeline`
- `PUT /api/tasks/:id/pipeline`
- `GET /api/config-node-types`
- `GET /api/tasks/:id/run-config`
- `POST /api/tasks/:id/run`
- `GET /api/task-runs/:id/events`
- `GET /api/node-types`
- `GET /api/agents`
- `GET /api/credentials`
- `POST /api/credentials`
- `PUT /api/credentials/:id`
- `DELETE /api/credentials/:id`
- `POST /api/auth/login`
- `GET /api/auth/me`
- `GET /api/users`
- `POST /api/users`
- `POST /api/agent-callback/heartbeat`
- `POST /api/agent-callback/node-runs/:id/logs`

统一响应格式：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

## 执行链路

`internal/engine` 会读取 Task 的 `pipeline_json`，创建 TaskRun，复制 Pipeline 快照，然后从 `startNodeId` 开始执行。节点成功后跳到 `nextNodeId`，失败后跳到 `fallbackNodeId`；如果旧 Pipeline 没有这些字段，则兼容按数组顺序执行。

Task 可以配置运行输入。`GET /api/tasks/:id/run-config` 会解析 `pipeline.inputs`，并按需执行被绑定的 `pipeline.configNodes` 来生成动态候选项。`POST /api/tasks/:id/run` 会做默认值补齐、必填校验和下拉候选校验，然后把输入写入 TaskRun。

节点参数支持运行输入占位符：

```text
${input.version}
${version}
```

执行前会替换占位符，NodeRun 的 `params_snapshot_json` 保存替换后的参数。

节点执行中的日志会写入 `run_logs`，同时发布到 `internal/logstream`，供 SSE 连接实时消费。

## 配置节点

配置节点位于 `internal/confignode` 和 `internal/confignodes`，用于运行前生成参数候选项，不参与 Pipeline 执行顺序。当前内置：

- `git_branches`: 使用 `git ls-remote --heads` 获取分支列表，输出 `options` 和 `branches`

配置节点可以通过凭据中心解析 `credentialId`。HTTPS username/password、HTTPS token 和 SSH key 的注入方式与 Git Checkout 节点保持一致，secret 不会写入 Pipeline JSON。

## 凭据

`internal/credential` 提供凭据 CRUD 和执行期解析。API 只返回凭据元数据和 `hasSecret`，不会返回 secret。Git Checkout 节点通过 `credentialId` 获取凭据：

- HTTPS username/password 和 token 使用临时 `GIT_ASKPASS`
- SSH private key 使用临时 key 文件和 `GIT_SSH_COMMAND`
- 日志会展示执行计划、workspace 检查、脱敏命令和 checkout 结果

Git Checkout 节点只做代码检出。它显式区分：

- `transport=https`: 仅接受 `https://...` 仓库和 `token` / `username_password` 凭据
- `transport=ssh`: 仅接受 `git@...` / `ssh://...` 仓库和 `ssh_key` 凭据

工作区策略：

- `fail_if_dirty`: 发现脏工作区即失败
- `reset_and_clean`: reset + clean 后更新
- `wipe_and_clone`: 删除目录重新 clone
- `reuse`: 不清理，直接尝试更新

secret 使用 `PUPPET_SECRET_KEY` 派生 AES-GCM key 加密。开发默认密钥只适合本地试用。

## Remote Agent

Server 通过 HTTP 主动调用 Agent：

```text
POST {agent.endpointUrl}/api/agent/execute-node
Authorization: Bearer <agent_token>
```

Agent 执行时回调 Server：

```text
POST /api/agent-callback/node-runs/:id/logs
POST /api/agent-callback/heartbeat
```

该模式要求 Server 能访问 Agent 的 endpoint。跨 NAT 或大量 Agent 场景后续可以再替换为 MQ dispatcher。
