<template>
  <div class="auth-page">
    <div class="auth-bg" />

    <header class="auth-top">
      <div class="auth-brand">
        <span class="auth-brand-mark">
          <svg width="34" height="34" viewBox="0 0 72 72" fill="none">
            <circle cx="36" cy="36" r="30" stroke="currentColor" stroke-width="2" opacity="0.5" />
            <circle cx="36" cy="36" r="16" stroke="currentColor" stroke-width="2" />
            <line x1="36" y1="5" x2="36" y2="67" stroke="currentColor" stroke-width="2" />
            <line x1="5" y1="36" x2="67" y2="36" stroke="currentColor" stroke-width="2" />
          </svg>
        </span>
        <strong>Puppet</strong>
      </div>
      <nav class="auth-nav">
        <span>产品特性</span>
        <span>文档</span>
        <span>社区</span>
      </nav>
    </header>

    <main class="auth-main">
      <section class="hero-copy">
        <div class="eyebrow">Pipeline Automation Platform</div>
        <h1>以<span>傀儡</span>之力<br />驱动自动化任务</h1>
        <p>灵偶执令，万事皆可自动化</p>

        <div class="pipeline-steps">
          <div class="step-line" />
          <div v-for="(step, index) in steps" :key="step.name" class="step" :style="{ animationDelay: `${index * 120}ms` }">
            <span class="step-node">✓</span>
            <span class="step-symbol">{{ step.symbol }}</span>
            <span class="step-name">{{ step.name }}</span>
            <span class="step-check">✓</span>
          </div>
        </div>

        <blockquote>真正的自动化，是让灵偶替你执掌繁琐，你专注于创造与成长。</blockquote>
      </section>

      <div class="login-column">
        <div class="status-bar">
          <span class="status-dot" />
          <span>{{ statusText }}</span>
          <span>最近成功：{{ latestSuccessText }}</span>
          <strong>Puppet</strong>
        </div>

        <section class="login-card" aria-label="登录">
          <div class="card-mark">
            <svg class="mark-ring" width="76" height="76" viewBox="0 0 76 76" fill="none">
              <circle cx="38" cy="38" r="33" stroke="currentColor" stroke-dasharray="7 5" opacity="0.5" />
              <circle cx="38" cy="38" r="20" stroke="currentColor" opacity="0.68" />
              <circle cx="38" cy="38" r="5" fill="currentColor" />
              <line x1="38" y1="9" x2="38" y2="27" stroke="currentColor" />
              <line x1="38" y1="49" x2="38" y2="67" stroke="currentColor" />
              <line x1="9" y1="38" x2="27" y2="38" stroke="currentColor" />
              <line x1="49" y1="38" x2="67" y2="38" stroke="currentColor" />
            </svg>
          </div>

          <h2>Puppet</h2>
          <p class="card-subtitle">以傀儡之力，驱动自动化任务</p>

          <form class="login-form" @submit.prevent="submit">
            <label class="field" :class="{ active: focus === 'user' }">
              <span class="field-icon">⌾</span>
              <input
                v-model="form.username"
                autocomplete="username"
                placeholder="账号 / 邮箱"
                @focus="focus = 'user'"
                @blur="focus = ''"
              />
            </label>

            <label class="field" :class="{ active: focus === 'pwd' }">
              <span class="field-icon">▣</span>
              <input
                v-model="form.password"
                :type="showPwd ? 'text' : 'password'"
                autocomplete="current-password"
                placeholder="密码"
                @focus="focus = 'pwd'"
                @blur="focus = ''"
              />
              <button type="button" class="eye" @click="showPwd = !showPwd">{{ showPwd ? '隐藏' : '显示' }}</button>
            </label>

            <button class="submit" type="submit" :disabled="loading">
              <span v-if="!loading">登 录</span>
              <span v-else class="spinner" />
            </button>
          </form>

          <div v-if="errorMsg" class="error">{{ errorMsg }}</div>
        </section>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api'
import type { PublicStatus } from '@/types'

const router = useRouter()
const loading = ref(false)
const showPwd = ref(false)
const focus = ref('')
const errorMsg = ref('')
const form = reactive({ username: '', password: '' })
const publicStatus = ref<PublicStatus | null>(null)
let statusTimer: number | undefined

const steps = [
  { name: 'Build', symbol: '⚙' },
  { name: 'Deploy', symbol: '◇' },
  { name: 'Run', symbol: '□' },
  { name: 'Success', symbol: '✓' },
]

const statusText = computed(() => {
  if (!publicStatus.value) return '流水线状态加载中'
  if (publicStatus.value.runningCount > 0) return `${publicStatus.value.runningCount} 条流水线正在运行`
  return '所有流水线运行正常'
})

const latestSuccessText = computed(() => {
  const value = publicStatus.value?.latestSuccessAt
  if (!value) return '暂无成功记录'
  return formatRelativeTime(value)
})

onMounted(() => {
  loadPublicStatus()
  statusTimer = window.setInterval(loadPublicStatus, 30000)
})

onBeforeUnmount(() => {
  if (statusTimer) window.clearInterval(statusTimer)
})

async function loadPublicStatus() {
  try {
    publicStatus.value = await api.publicStatus()
  } catch {
    publicStatus.value = null
  }
}

function formatRelativeTime(value: string) {
  const elapsed = Date.now() - new Date(value).getTime()
  if (!Number.isFinite(elapsed) || elapsed < 0) return '刚刚'
  const seconds = Math.floor(elapsed / 1000)
  if (seconds < 60) return '刚刚'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} 小时前`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days} 天前`
  return new Date(value).toLocaleString()
}

async function submit() {
  if (!form.username || !form.password) return
  errorMsg.value = ''
  loading.value = true
  try {
    const resp = await api.login(form)
    localStorage.setItem('puppet_token', resp.token)
    router.push('/dashboard')
  } catch {
    errorMsg.value = '账号或密码错误，请重试'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page {
  position: fixed;
  inset: 0;
  overflow: hidden;
  min-width: 320px;
  background: #07101f;
  color: #eef6ff;
  display: block;
  place-items: initial;
}

.auth-bg {
  position: absolute;
  inset: 0;
  background-image: url('/login-bg-rich.png');
  background-size: cover;
  background-position: center center;
}

.auth-bg::after {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(circle at 31% 43%, rgba(45, 212, 191, 0.08), transparent 34%),
    linear-gradient(90deg, rgba(4, 10, 22, 0.08), rgba(5, 12, 26, 0.04) 48%, rgba(5, 12, 26, 0.42) 74%, rgba(4, 9, 18, 0.74)),
    linear-gradient(180deg, rgba(3, 8, 18, 0.38), rgba(3, 8, 18, 0.02) 42%, rgba(3, 8, 18, 0.44));
}

.auth-top {
  position: relative;
  z-index: 2;
  height: 72px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 clamp(42px, 5vw, 72px);
  border-bottom: 1px solid rgba(222, 238, 244, 0.14);
  background: linear-gradient(180deg, rgba(3, 10, 22, 0.76), rgba(3, 10, 22, 0.18));
}

.auth-brand,
.auth-nav,
.status-bar,
.step,
.field,
.submit {
  display: flex;
  align-items: center;
}

.auth-brand {
  gap: 12px;
  color: rgba(238, 229, 208, 0.95);
}

.auth-brand-mark {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: transparent;
  color: #65f4e3;
  font-weight: inherit;
  filter: drop-shadow(0 0 12px rgba(45, 212, 191, 0.42));
}

.auth-brand strong {
  font-family: Georgia, 'Times New Roman', serif;
  font-size: 27px;
  letter-spacing: 0.02em;
}

.auth-nav {
  margin-left: auto;
  gap: 44px;
  color: rgba(225, 235, 244, 0.76);
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 0.08em;
}

.auth-main {
  position: relative;
  z-index: 1;
  min-height: calc(100vh - 72px);
  display: grid;
  grid-template-columns: minmax(300px, 35%) minmax(620px, 65%);
}

.hero-copy {
  width: min(300px, 25vw);
  padding: clamp(104px, 15vh, 148px) 0 96px clamp(36px, 4vw, 68px);
}

.eyebrow {
  width: fit-content;
  margin-bottom: 22px;
  padding: 5px 13px;
  border: 1px solid rgba(45, 212, 191, 0.42);
  border-radius: 999px;
  background: rgba(3, 14, 27, 0.42);
  color: #40f0dd;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.hero-copy h1 {
  margin: 0 0 14px;
  color: #edf6ff;
  font-size: clamp(28px, 2.45vw, 38px);
  line-height: 1.18;
  font-weight: 900;
  letter-spacing: 0;
  text-shadow: 0 12px 34px rgba(0, 0, 0, 0.35);
}

.hero-copy h1 span {
  color: #2dd4bf;
  text-shadow: 0 0 32px rgba(45, 212, 191, 0.48);
}

.hero-copy p {
  margin: 0 0 30px;
  color: rgba(218, 229, 240, 0.82);
  font-size: 15px;
}

.pipeline-steps {
  position: relative;
  width: 170px;
  display: grid;
  gap: 10px;
  margin-bottom: 22px;
}

.step-line {
  position: absolute;
  top: 24px;
  bottom: 24px;
  left: 23px;
  width: 1px;
  background: linear-gradient(#2dd4bf, rgba(45, 212, 191, 0.08));
}

.step {
  position: relative;
  min-height: 36px;
  gap: 8px;
  padding: 5px 9px 5px 5px;
  border: 1px solid rgba(45, 212, 191, 0.3);
  border-radius: 12px;
  background: linear-gradient(90deg, rgba(5, 18, 31, 0.74), rgba(21, 51, 58, 0.34));
  box-shadow: inset 0 0 18px rgba(45, 212, 191, 0.05), 0 8px 24px rgba(0, 0, 0, 0.18);
  opacity: 0;
  transform: translateX(-10px);
  animation: step-in 420ms ease forwards;
}

@keyframes step-in {
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

.step-node {
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 50%;
  background: #2dd4bf;
  color: #06131b;
  font-weight: 900;
  box-shadow: 0 0 0 5px rgba(45, 212, 191, 0.12);
}

.step-symbol {
  width: 16px;
  text-align: center;
  color: rgba(238, 229, 208, 0.88);
}

.step-name {
  color: rgba(238, 229, 208, 0.94);
  font-size: 12px;
  font-weight: 700;
}

.step-check {
  margin-left: auto;
  width: 18px;
  height: 18px;
  display: grid;
  place-items: center;
  border: 1px solid rgba(45, 212, 191, 0.58);
  border-radius: 50%;
  color: #65f4e3;
  font-size: 12px;
}

blockquote {
  max-width: 270px;
  margin: 0;
  padding: 14px 0 14px 20px;
  border-left: 2px solid rgba(45, 212, 191, 0.42);
  color: rgba(220, 231, 240, 0.76);
  font-size: 13px;
  line-height: 1.8;
}

.login-column {
  align-self: start;
  justify-self: end;
  width: min(390px, calc(100vw - 48px));
  margin: clamp(46px, 8vh, 82px) clamp(36px, 5vw, 96px) 36px 24px;
  display: grid;
  gap: 14px;
}

.login-card {
  width: 100%;
  padding: 34px 38px 34px;
  border: 1px solid rgba(221, 236, 238, 0.3);
  border-radius: 12px;
  background:
    linear-gradient(180deg, rgba(190, 213, 226, 0.22), rgba(17, 37, 54, 0.46)),
    rgba(9, 22, 36, 0.5);
  box-shadow: 0 34px 90px rgba(0, 0, 0, 0.45), inset 0 0 0 1px rgba(45, 212, 191, 0.08);
  backdrop-filter: blur(22px) saturate(128%);
}

.card-mark {
  width: 68px;
  height: 68px;
  margin: 0 auto 18px;
  color: #66f5e4;
  filter: drop-shadow(0 0 18px rgba(45, 212, 191, 0.42));
}

.mark-ring {
  animation: spin-slow 18s linear infinite;
}

@keyframes spin-slow {
  to {
    transform: rotate(360deg);
  }
}

.login-card h2 {
  margin: 0;
  color: rgba(238, 229, 208, 0.96);
  font-family: Georgia, 'Times New Roman', serif;
  font-size: 36px;
  font-weight: 600;
  text-align: center;
  letter-spacing: 0.08em;
}

.card-subtitle {
  margin: 10px 0 30px;
  color: rgba(223, 234, 242, 0.58);
  font-size: 14px;
  text-align: center;
}

.login-form {
  display: grid;
  gap: 16px;
}

.field {
  min-height: 54px;
  border: 1px solid rgba(255, 255, 255, 0.36);
  border-radius: 8px;
  background: rgba(226, 235, 244, 0.72);
  transition: border-color 160ms, box-shadow 160ms, background 160ms;
}

.field.active {
  border-color: rgba(45, 212, 191, 0.52);
  background: rgba(238, 246, 250, 0.86);
  box-shadow: 0 0 0 3px rgba(45, 212, 191, 0.12);
}

.field-icon {
  width: 48px;
  text-align: center;
  color: #536b7e;
  font-weight: 800;
}

.field input {
  width: 100%;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: #24394c;
  font: inherit;
  font-size: 15px;
}

.field input::placeholder {
  color: rgba(55, 74, 91, 0.74);
}

.eye {
  flex: 0 0 auto;
  margin-right: 12px;
  border: 0;
  background: transparent;
  color: #536b7e;
  cursor: pointer;
  font-size: 12px;
}

.submit {
  min-height: 56px;
  justify-content: center;
  margin-top: 8px;
  border: 0;
  border-radius: 8px;
  background: linear-gradient(135deg, #0f817a, #21c7b8 48%, #0b6f69);
  color: #eefdfb;
  cursor: pointer;
  font-size: 16px;
  font-weight: 900;
  letter-spacing: 0.16em;
  box-shadow: 0 14px 30px rgba(20, 188, 174, 0.26);
}

.submit:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.spinner {
  width: 18px;
  height: 18px;
  border: 2px solid rgba(255, 255, 255, 0.38);
  border-top-color: white;
  border-radius: 50%;
  animation: spin-slow 800ms linear infinite;
}

.error {
  margin-top: 14px;
  padding: 10px 12px;
  border: 1px solid rgba(248, 113, 113, 0.28);
  border-radius: 8px;
  background: rgba(248, 113, 113, 0.1);
  color: #fecaca;
  text-align: center;
  font-size: 13px;
}

.status-bar {
  position: relative;
  z-index: 1;
  min-height: 48px;
  justify-content: center;
  gap: 16px;
  padding: 12px 16px;
  border: 1px solid rgba(222, 238, 244, 0.12);
  border-radius: 8px;
  background: rgba(4, 13, 24, 0.72);
  color: rgba(222, 234, 245, 0.72);
  font-size: 12px;
  box-shadow: 0 20px 44px rgba(0, 0, 0, 0.32);
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #40f0dd;
  box-shadow: 0 0 14px rgba(45, 212, 191, 0.84);
}

.status-bar strong {
  color: rgba(238, 229, 208, 0.78);
}

@media (max-width: 980px) {
  .auth-top {
    padding: 0 24px;
  }

  .auth-nav,
  .hero-copy {
    display: none;
  }

  .auth-main {
    display: grid;
    grid-template-columns: 1fr;
    min-height: calc(100vh - 72px);
  }

  .login-column {
    align-self: start;
    margin: clamp(24px, 6vh, 46px) 24px 24px;
    justify-self: center;
  }

  .status-bar {
    flex-wrap: wrap;
    gap: 10px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .step,
  .mark-ring,
  .spinner {
    animation: none;
  }

  .step {
    opacity: 1;
    transform: none;
  }
}
</style>
