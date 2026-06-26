export type Status = 'pending' | 'running' | 'success' | 'failed' | 'canceled' | 'timeout' | 'skipped'

export interface Project {
  id: number
  name: string
  description: string
  createdAt: string
  updatedAt: string
}

export interface Task {
  id: number
  projectId: number
  name: string
  description: string
  pipelineJson: string
  allowConcurrent: boolean
  timeoutSeconds: number
  createdAt: string
  updatedAt: string
}

export interface TaskRun {
  id: number
  projectId: number
  taskId: number
  status: Status
  triggerType: string
  triggeredBy: string
  inputJson: string
  pipelineSnapshotJson: string
  startedAt?: string
  finishedAt?: string
  durationMs: number
  errorMessage: string
  createdAt: string
}

export interface NodeRun {
  id: number
  taskRunId: number
  nodeId: string
  nodeName: string
  nodeType: string
  status: Status
  nodeIndex: number
  paramsSnapshotJson: string
  outputJson: string
  startedAt?: string
  finishedAt?: string
  durationMs: number
  errorMessage: string
  retryCount: number
  createdAt: string
}

export interface RunLog {
  id: number
  taskRunId: number
  nodeRunId: number
  sequence: number
  stream: 'stdout' | 'stderr' | 'system'
  content: string
  createdAt: string
}

export interface Agent {
  id: number
  name: string
  endpointUrl: string
  os: string
  arch: string
  hostname: string
  labelsJson: string
  status: string
  lastHeartbeatAt?: string
  createdAt: string
  updatedAt: string
}

export interface AgentInput {
  name: string
  endpointUrl: string
  labels: string[]
  status?: string
}

export interface AgentCreateResponse {
  agent: Agent
  token: string
}

export interface User {
  id: number
  username: string
  displayName: string
  role: string
  status: string
  lastLoginAt?: string
  createdAt: string
  updatedAt: string
}

export interface LoginResponse {
  token: string
  user: User
}

export interface UserInput {
  username: string
  displayName: string
  role: string
  status: string
  password?: string
}

export interface Credential {
  id: number
  name: string
  type: 'username_password' | 'token' | 'ssh_key'
  description: string
  username: string
  hasSecret: boolean
  createdAt: string
  updatedAt: string
}

export interface CredentialInput {
  name: string
  type: Credential['type']
  description: string
  username: string
  password?: string
  token?: string
  privateKey?: string
}

export interface NodeField {
  name: string
  label: string
  type: 'input' | 'textarea' | 'number' | 'select' | 'switch' | 'credential'
  required: boolean
  default?: unknown
  options?: string[]
  secret?: boolean
  showWhen?: {
    field: string
    equals: unknown
  }
}

export interface NodeMetadata {
  type: string
  name: string
  category: string
  description: string
  supportedOS: string[]
  fields: NodeField[]
}

export interface PipelineNode {
  id: string
  name: string
  type: string
  params: Record<string, unknown>
  timeoutSeconds: number
  retryTimes: number
  nextNodeId: string
  fallbackNodeId: string
  continueOnError: boolean
}

export interface InputSource {
  type: string
  params: Record<string, unknown>
}

export interface PipelineInput {
  name: string
  label: string
  type: 'string' | 'select' | 'boolean' | 'number'
  required: boolean
  default?: unknown
  options?: string[]
  source?: InputSource
  // UI-only helper, not sent to backend
  optionsText?: string
}

export interface PipelineDefinition {
  name: string
  startNodeId: string
  agentSelector: {
    labels: string[]
  }
  inputs: PipelineInput[]
  nodes: PipelineNode[]
}

export interface RunConfigInput extends PipelineInput {
  options: string[]
  error?: string
}

export interface RunConfig {
  inputs: RunConfigInput[]
}

export interface DashboardSummary {
  projectCount: number
  taskCount: number
  todayRunCount: number
  runningCount: number
  successCount: number
  failedCount: number
  agentOnlineCount: number
  recentRuns: TaskRun[]
}
