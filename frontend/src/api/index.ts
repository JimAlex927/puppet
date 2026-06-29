import { http, request } from './client'
import type {
  Agent,
  AgentCreateResponse,
  AgentInput,
  Credential,
  CredentialInput,
  DashboardSummary,
  LoginResponse,
  NodeMetadata,
  NodeRun,
  PageResult,
  PipelineDefinition,
  Project,
  PublicStatus,
  RunLog,
  RunConfig,
  SharedFile,
  SharedFileShare,
  Task,
  TaskRun,
  User,
  UserInput,
} from '@/types'


export const api = {
  publicStatus: () => request<PublicStatus>({ url: '/public/status' }),

  login: (data: { username: string; password: string }) =>
    request<LoginResponse>({ url: '/auth/login', method: 'POST', data }),
  logout: () => request<{ loggedOut: boolean }>({ url: '/auth/logout', method: 'POST' }),
  me: () => request<User>({ url: '/auth/me' }),

  dashboard: () => request<DashboardSummary>({ url: '/dashboard/summary' }),

  sharedFiles: () => request<SharedFile[]>({ url: '/shared-files' }),
  createSharedFileShare: (id: number, expiresInMinutes: number) =>
    request<SharedFileShare>({
      url: `/shared-files/${id}/share`,
      method: 'POST',
      data: { expiresInMinutes },
    }),
  deleteSharedFile: (id: number) => request<{ deleted: boolean }>({ url: `/shared-files/${id}`, method: 'DELETE' }),
  sharedFileDownloadUrl: (id: number) => {
    const token = localStorage.getItem('puppet_token')
    const query = token ? `?token=${encodeURIComponent(token)}` : ''
    return `/api/shared-files/${id}/download${query}`
  },

  projects: () => request<Project[]>({ url: '/projects' }),
  projectsPage: (page: number, pageSize: number) =>
    request<PageResult<Project>>({ url: '/projects', params: { page, pageSize } }),
  project: (id: number) => request<Project>({ url: `/projects/${id}` }),
  createProject: (data: Pick<Project, 'name' | 'description'>) =>
    request<Project>({ url: '/projects', method: 'POST', data }),
  updateProject: (id: number, data: Pick<Project, 'name' | 'description'>) =>
    request<Project>({ url: `/projects/${id}`, method: 'PUT', data }),
  deleteProject: (id: number) => request<{ deleted: boolean }>({ url: `/projects/${id}`, method: 'DELETE' }),
  exportProject: async (id: number) => {
    const response = await http.request<Blob>({ url: `/projects/${id}/export`, responseType: 'blob' })
    return response.data
  },
  importProject: (file: File) => {
    const data = new FormData()
    data.append('file', file)
    return request<Project>({ url: '/projects/import', method: 'POST', data })
  },

  tasks: (projectId: number) => request<Task[]>({ url: `/projects/${projectId}/tasks` }),
  tasksPage: (projectId: number, page: number, pageSize: number) =>
    request<PageResult<Task>>({ url: `/projects/${projectId}/tasks`, params: { page, pageSize } }),
  task: (id: number) => request<Task>({ url: `/tasks/${id}` }),
  createTask: (projectId: number, data: Partial<Task>) =>
    request<Task>({ url: `/projects/${projectId}/tasks`, method: 'POST', data }),
  updateTask: (id: number, data: Partial<Task>) => request<Task>({ url: `/tasks/${id}`, method: 'PUT', data }),
  deleteTask: (id: number) => request<{ deleted: boolean }>({ url: `/tasks/${id}`, method: 'DELETE' }),

  pipeline: (taskId: number) => request<PipelineDefinition>({ url: `/tasks/${taskId}/pipeline` }),
  savePipeline: (taskId: number, data: PipelineDefinition) =>
    request<PipelineDefinition>({ url: `/tasks/${taskId}/pipeline`, method: 'PUT', data }),
  nodeTypes: () => request<NodeMetadata[]>({ url: '/node-types' }),
  sourceTypes: () => request<NodeMetadata[]>({ url: '/config-node-types' }),
  runConfig: (taskId: number) => request<RunConfig>({ url: `/tasks/${taskId}/run-config` }),

  runTask: (taskId: number, input: Record<string, unknown> = {}) =>
    request<TaskRun>({ url: `/tasks/${taskId}/run`, method: 'POST', data: { input } }),
  prepareTaskRun: (taskId: number, input: Record<string, unknown> = {}) =>
    request<TaskRun>({ url: `/tasks/${taskId}/runs/prepare`, method: 'POST', data: { input } }),
  startTaskRun: (runId: number) => request<TaskRun>({ url: `/task-runs/${runId}/start`, method: 'POST' }),
  taskRuns: (taskId: number) => request<TaskRun[]>({ url: `/tasks/${taskId}/runs` }),
  taskRun: (id: number) => request<TaskRun>({ url: `/task-runs/${id}` }),
  cancelTaskRun: (id: number) => request<TaskRun>({ url: `/task-runs/${id}/cancel`, method: 'POST' }),
  nodeRuns: (runId: number) => request<NodeRun[]>({ url: `/task-runs/${runId}/node-runs` }),
  runLogs: (runId: number) => request<RunLog[]>({ url: `/task-runs/${runId}/logs` }),

  agents: () => request<Agent[]>({ url: '/agents' }),
  agent: (id: number) => request<Agent>({ url: `/agents/${id}` }),
  createAgent: (data: AgentInput) => request<AgentCreateResponse>({ url: '/agents', method: 'POST', data }),
  updateAgent: (id: number, data: AgentInput) => request<Agent>({ url: `/agents/${id}`, method: 'PUT', data }),
  deleteAgent: (id: number) => request<{ deleted: boolean }>({ url: `/agents/${id}`, method: 'DELETE' }),

  credentials: () => request<Credential[]>({ url: '/credentials' }),
  credential: (id: number) => request<Credential>({ url: `/credentials/${id}` }),
  createCredential: (data: CredentialInput) => request<Credential>({ url: '/credentials', method: 'POST', data }),
  updateCredential: (id: number, data: CredentialInput) =>
    request<Credential>({ url: `/credentials/${id}`, method: 'PUT', data }),
  deleteCredential: (id: number) => request<{ deleted: boolean }>({ url: `/credentials/${id}`, method: 'DELETE' }),

  users: () => request<User[]>({ url: '/users' }),
  createUser: (data: UserInput) => request<User>({ url: '/users', method: 'POST', data }),
  updateUser: (id: number, data: UserInput) => request<User>({ url: `/users/${id}`, method: 'PUT', data }),
  deleteUser: (id: number) => request<{ deleted: boolean }>({ url: `/users/${id}`, method: 'DELETE' }),
}
