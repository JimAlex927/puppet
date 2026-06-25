# Frontend README

Vue 3 管理台，覆盖 Dashboard、Project、Task、Pipeline 编辑、执行记录、实时日志和 Agent 页面。

默认登录：

```text
puppetadmin / puppetadmin
```

## 启动

```powershell
npm install
npm run dev
```

访问 `http://localhost:5173`。

## 构建

```powershell
npm run build
```

## 目录

```text
src/api        Axios API 客户端
src/router     Vue Router
src/pages      页面
src/components Pipeline 编辑、日志、状态组件
src/types      前后端共享类型
src/utils      格式化工具
src/stores     状态管理预留目录
```

## Pipeline 编辑

编辑器仍采用列表式管理，不做拖拽画布。每个节点可以配置：

- 成功出口 `nextNodeId`
- 失败出口 `fallbackNodeId`
- 超时、重试和节点参数
- Git Checkout 节点通过 Credential 下拉选择凭据，并显式选择 HTTPS/SSH transport 和 workspace policy

执行顺序由出口字段决定，列表顺序只用于阅读和维护。

## Credentials

Credentials 页面支持维护：

- Username / Password
- Token
- SSH Private Key

编辑已有凭据时，secret 字段留空表示保留原值。前端不会读取或展示已保存的 secret。

## Agents

Agents 页面可创建远程 Agent，并显示一次性 token。启动目标机器上的 Agent 后，页面会通过 heartbeat 显示在线状态。Pipeline 编辑器中的 `Agent Labels` 用于选择执行节点的 Agent。

## 实时日志

执行详情页加载历史 TaskRun、NodeRun、RunLog 后，会连接：

```text
GET /api/task-runs/:id/events
```

并监听 `log`、`node_status`、`task_status` 三类 SSE 事件。
