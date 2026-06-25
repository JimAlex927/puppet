import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/dashboard' },
    { path: '/login', component: () => import('@/pages/login/LoginPage.vue'), meta: { title: 'Login', public: true } },
    { path: '/dashboard', component: () => import('@/pages/dashboard/DashboardPage.vue'), meta: { title: 'Dashboard' } },
    { path: '/projects', component: () => import('@/pages/projects/ProjectsPage.vue'), meta: { title: 'Projects' } },
    {
      path: '/projects/:id',
      component: () => import('@/pages/project-detail/ProjectDetailPage.vue'),
      meta: { title: 'Project Detail' },
    },
    {
      path: '/tasks/:id/pipeline',
      component: () => import('@/pages/pipeline-editor/PipelineEditorPage.vue'),
      meta: { title: 'Pipeline Editor' },
    },
    { path: '/tasks/:id/runs', component: () => import('@/pages/runs/RunsPage.vue'), meta: { title: 'Task Runs' } },
    { path: '/runs/:id', component: () => import('@/pages/run-detail/RunDetailPage.vue'), meta: { title: 'Run Detail' } },
    { path: '/agents', component: () => import('@/pages/agents/AgentsPage.vue'), meta: { title: 'Agents' } },
    {
      path: '/credentials',
      component: () => import('@/pages/credentials/CredentialsPage.vue'),
      meta: { title: 'Credentials' },
    },
    { path: '/users', component: () => import('@/pages/users/UsersPage.vue'), meta: { title: 'Users' } },
  ],
})

router.beforeEach((to) => {
  if (to.meta.public) return true
  if (!localStorage.getItem('puppet_token')) return '/login'
  return true
})

export default router
