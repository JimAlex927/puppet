<template>
  <div class="lp">
    <!-- Atmospheric background -->
    <div class="lp-bg" />

    <!-- Left: brand + pipeline steps -->
    <div class="lp-left">
      <div class="lp-left-inner">
        <div class="lp-eyebrow">Pipeline Automation Platform</div>
        <h1 class="lp-headline">
          以<em class="lp-em">傀儡</em>之力<br>驱动自动化任务
        </h1>
        <p class="lp-tagline">灵偶执令，万事皆可自动化</p>

        <div class="lp-steps">
          <div class="lp-steps-track" />
          <div
            v-for="(s, i) in steps"
            :key="s.name"
            class="lp-step"
            :style="`animation-delay:${0.5 + i * 0.22}s`"
          >
            <div class="lp-step-node">
              <div class="lp-step-pulse" />
              <svg width="10" height="8" viewBox="0 0 10 8" fill="none">
                <path d="M1 4l3 3 5-6" stroke="#0d1628" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </div>
            <span class="lp-step-name">{{ s.name }}</span>
          </div>
        </div>

        <blockquote class="lp-quote">
          <span class="lp-quote-mark">"</span>
          真正的自动化，是让灵偶替你执掌繁琐，<br>你专注于创造与成长。
        </blockquote>
      </div>

      <div class="lp-statusbar">
        <span class="lp-statusbar-dot" />
        <span>所有流水线运行正常</span>
        <span class="lp-statusbar-sep">·</span>
        <span>最近成功：2 分钟前</span>
        <span class="lp-statusbar-brand">
          <svg width="14" height="14" viewBox="0 0 36 36" fill="none">
            <circle cx="18" cy="18" r="14" stroke="#2dd4bf" stroke-width="1.5" opacity="0.5"/>
            <circle cx="18" cy="18" r="7" stroke="#2dd4bf" stroke-width="1.5"/>
            <circle cx="18" cy="18" r="2" fill="#2dd4bf"/>
            <line x1="18" y1="4" x2="18" y2="11" stroke="#2dd4bf" stroke-width="1.5"/>
            <line x1="18" y1="25" x2="18" y2="32" stroke="#2dd4bf" stroke-width="1.5"/>
            <line x1="4" y1="18" x2="11" y2="18" stroke="#2dd4bf" stroke-width="1.5"/>
            <line x1="25" y1="18" x2="32" y2="18" stroke="#2dd4bf" stroke-width="1.5"/>
          </svg>
          Puppet
        </span>
      </div>
    </div>

    <!-- Right: login card -->
    <div class="lp-right">
      <div class="lp-card">
        <!-- Animated compass logo -->
        <div class="lp-logo">
          <svg class="lp-logo-ring" width="72" height="72" viewBox="0 0 72 72" fill="none">
            <circle cx="36" cy="36" r="32" stroke="#2dd4bf" stroke-width="1" stroke-dasharray="6 4" opacity="0.4"/>
            <circle cx="36" cy="36" r="24" stroke="#2dd4bf" stroke-width="1" opacity="0.25"/>
          </svg>
          <svg class="lp-logo-core" width="72" height="72" viewBox="0 0 72 72" fill="none">
            <circle cx="36" cy="36" r="16" stroke="#2dd4bf" stroke-width="1.5" opacity="0.7"/>
            <circle cx="36" cy="36" r="5" fill="#2dd4bf" opacity="0.9"/>
            <line x1="36" y1="12" x2="36" y2="22" stroke="#2dd4bf" stroke-width="1.5"/>
            <line x1="36" y1="50" x2="36" y2="60" stroke="#2dd4bf" stroke-width="1.5"/>
            <line x1="12" y1="36" x2="22" y2="36" stroke="#2dd4bf" stroke-width="1.5"/>
            <line x1="50" y1="36" x2="60" y2="36" stroke="#2dd4bf" stroke-width="1.5"/>
            <polygon points="36,20 38.5,28 36,26 33.5,28" fill="#2dd4bf" opacity="0.8"/>
          </svg>
        </div>

        <h2 class="lp-card-title">Puppet</h2>
        <p class="lp-card-sub">以傀儡之力，驱动自动化任务</p>

        <form class="lp-form" @submit.prevent="submit">
          <div class="lp-field" :class="{ 'lp-field--focus': focus === 'user' }">
            <svg class="lp-field-icon" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>
            </svg>
            <input
              v-model="form.username"
              class="lp-input"
              placeholder="账号"
              autocomplete="username"
              @focus="focus = 'user'"
              @blur="focus = ''"
            />
          </div>

          <div class="lp-field" :class="{ 'lp-field--focus': focus === 'pwd' }">
            <svg class="lp-field-icon" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>
            </svg>
            <input
              v-model="form.password"
              :type="showPwd ? 'text' : 'password'"
              class="lp-input"
              placeholder="密码"
              autocomplete="current-password"
              @focus="focus = 'pwd'"
              @blur="focus = ''"
              @keyup.enter="submit"
            />
            <button type="button" class="lp-eye" @click="showPwd = !showPwd">
              <svg v-if="!showPwd" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/>
              </svg>
              <svg v-else width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/>
                <line x1="1" y1="1" x2="23" y2="23"/>
              </svg>
            </button>
          </div>

          <button type="submit" class="lp-submit" :disabled="loading">
            <span v-if="!loading">登 录</span>
            <span v-else class="lp-spin" />
          </button>
        </form>

        <div v-if="errorMsg" class="lp-error">{{ errorMsg }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api'
const router = useRouter()
const loading = ref(false)
const showPwd = ref(false)
const focus = ref('')
const errorMsg = ref('')
const form = reactive({ username: '', password: '' })

const steps = [
  { name: 'Clone' },
  { name: 'Build' },
  { name: 'Deploy' },
  { name: 'Success' },
]

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
/* ── Reset & root ──────────────────────────────────────── */
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

.lp {
  position: fixed;
  inset: 0;
  display: flex;
  overflow: hidden;
  font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
}

/* ── Background ────────────────────────────────────────── */
.lp-bg {
  position: absolute;
  inset: 0;
  background-image: url('/bg.png');
  background-size: cover;
  background-position: center top;
  z-index: 0;
}
/* Dark atmospheric overlay */
.lp-bg::after {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse 60% 70% at 38% 55%, rgba(45,212,191,0.05) 0%, transparent 65%),
    linear-gradient(135deg, rgba(5,10,25,0.62) 0%, rgba(8,16,38,0.58) 50%, rgba(5,10,22,0.68) 100%);
}

/* ── Left panel ─────────────────────────────────────────── */
.lp-left {
  position: relative;
  z-index: 1;
  width: 58%;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 52px 56px 36px;
}

.lp-left-inner {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding-bottom: 40px;
}

.lp-eyebrow {
  display: inline-block;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: #2dd4bf;
  border: 1px solid rgba(45,212,191,0.3);
  padding: 4px 12px;
  border-radius: 20px;
  margin-bottom: 28px;
  width: fit-content;
}

.lp-headline {
  font-size: clamp(32px, 4vw, 52px);
  font-weight: 700;
  line-height: 1.18;
  color: #e8f0fe;
  letter-spacing: -0.01em;
  margin-bottom: 16px;
}

.lp-em {
  font-style: normal;
  color: #2dd4bf;
  text-shadow: 0 0 32px rgba(45,212,191,0.4);
}

.lp-tagline {
  font-size: 15px;
  color: #5d7490;
  margin-bottom: 48px;
  letter-spacing: 0.02em;
}

/* Pipeline steps */
.lp-steps {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 20px;
  margin-bottom: 48px;
  padding-left: 2px;
}

.lp-steps-track {
  position: absolute;
  left: 17px;
  top: 20px;
  bottom: 20px;
  width: 1px;
  background: linear-gradient(to bottom, rgba(45,212,191,0.5), rgba(45,212,191,0.1));
}

.lp-step {
  display: flex;
  align-items: center;
  gap: 16px;
  opacity: 0;
  transform: translateX(-12px);
  animation: stepIn 0.5s cubic-bezier(0.22, 1, 0.36, 1) forwards;
}

@keyframes stepIn {
  to { opacity: 1; transform: translateX(0); }
}

.lp-step-node {
  position: relative;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: #2dd4bf;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  box-shadow: 0 0 0 4px rgba(45,212,191,0.12), 0 0 12px rgba(45,212,191,0.3);
  z-index: 1;
}

.lp-step-pulse {
  position: absolute;
  inset: -4px;
  border-radius: 50%;
  border: 1px solid rgba(45,212,191,0.4);
  animation: stepPulse 2.5s ease-in-out infinite;
}

.lp-step:nth-child(2) .lp-step-pulse { animation-delay: 0.3s; }
.lp-step:nth-child(3) .lp-step-pulse { animation-delay: 0.6s; }
.lp-step:nth-child(4) .lp-step-pulse { animation-delay: 0.9s; }

@keyframes stepPulse {
  0%, 100% { opacity: 0.4; transform: scale(1); }
  50% { opacity: 0; transform: scale(1.5); }
}

.lp-step-name {
  font-size: 14px;
  font-weight: 500;
  color: #a0b4cc;
  letter-spacing: 0.04em;
}

/* Quote */
.lp-quote {
  position: relative;
  font-size: 13px;
  line-height: 1.8;
  color: #4d6278;
  padding-left: 20px;
  border-left: 2px solid rgba(45,212,191,0.25);
  font-style: normal;
  max-width: 360px;
}

.lp-quote-mark {
  font-size: 28px;
  line-height: 0;
  vertical-align: -10px;
  color: rgba(45,212,191,0.3);
  margin-right: 4px;
  font-family: Georgia, serif;
}

/* Status bar */
.lp-statusbar {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: #3d5068;
}

.lp-statusbar-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #2dd4bf;
  box-shadow: 0 0 6px #2dd4bf;
  flex-shrink: 0;
  animation: blink 2.4s ease-in-out infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.lp-statusbar-sep { color: #253545; }

.lp-statusbar-brand {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 7px;
  color: #2d4058;
  font-weight: 600;
  font-size: 13px;
  letter-spacing: 0.06em;
}

/* ── Right panel ────────────────────────────────────────── */
.lp-right {
  position: relative;
  z-index: 1;
  width: 42%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 52px 40px 32px;
}

/* Glassmorphism card */
.lp-card {
  width: 100%;
  max-width: 380px;
  background: rgba(6, 12, 30, 0.72);
  backdrop-filter: blur(24px) saturate(140%);
  -webkit-backdrop-filter: blur(24px) saturate(140%);
  border: 1px solid rgba(45,212,191,0.14);
  border-radius: 18px;
  padding: 44px 40px 40px;
  box-shadow:
    0 0 0 1px rgba(0,0,0,0.3),
    0 24px 64px rgba(0,0,0,0.5),
    0 0 80px rgba(45,212,191,0.04) inset;
  display: flex;
  flex-direction: column;
  align-items: center;
}

/* Rotating compass logo */
.lp-logo {
  position: relative;
  width: 72px;
  height: 72px;
  margin-bottom: 22px;
}

.lp-logo-ring {
  position: absolute;
  inset: 0;
  animation: rotateSlow 18s linear infinite;
}

.lp-logo-core {
  position: absolute;
  inset: 0;
}

@keyframes rotateSlow {
  from { transform: rotate(0deg); }
  to   { transform: rotate(360deg); }
}

.lp-card-title {
  font-size: 26px;
  font-weight: 700;
  color: #ddeeff;
  letter-spacing: 0.08em;
  margin-bottom: 7px;
}

.lp-card-sub {
  font-size: 12px;
  color: #3d5878;
  letter-spacing: 0.04em;
  margin-bottom: 36px;
  text-align: center;
}

/* Form */
.lp-form {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.lp-field {
  position: relative;
  display: flex;
  align-items: center;
  background: rgba(255,255,255,0.035);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 9px;
  transition: border-color 0.2s, box-shadow 0.2s, background 0.2s;
}

.lp-field--focus {
  background: rgba(45,212,191,0.04);
  border-color: rgba(45,212,191,0.35);
  box-shadow: 0 0 0 3px rgba(45,212,191,0.08);
}

.lp-field-icon {
  padding: 0 0 0 14px;
  color: #3d5878;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  transition: color 0.2s;
}

.lp-field--focus .lp-field-icon { color: #2dd4bf; }

.lp-input {
  flex: 1;
  background: none;
  border: none;
  outline: none;
  padding: 13px 12px;
  font-size: 14px;
  color: #c8daf0;
  font-family: inherit;
  width: 100%;
}

.lp-input::placeholder { color: #2e4560; }

.lp-input:-webkit-autofill,
.lp-input:-webkit-autofill:hover,
.lp-input:-webkit-autofill:focus {
  -webkit-box-shadow: 0 0 0 1000px rgba(6, 14, 36, 0.95) inset !important;
  -webkit-text-fill-color: #c8daf0 !important;
  caret-color: #c8daf0;
  transition: background-color 9999s ease-in-out 0s;
}

.lp-eye {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0 14px;
  color: #2e4560;
  display: flex;
  align-items: center;
  transition: color 0.2s;
  flex-shrink: 0;
}
.lp-eye:hover { color: #2dd4bf; }

/* Submit button */
.lp-submit {
  width: 100%;
  margin-top: 6px;
  padding: 13px;
  background: linear-gradient(135deg, #1fbeac 0%, #2dd4bf 50%, #22c4b0 100%);
  border: none;
  border-radius: 9px;
  color: #051015;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.12em;
  cursor: pointer;
  transition: opacity 0.2s, transform 0.15s, box-shadow 0.2s;
  box-shadow: 0 4px 20px rgba(45,212,191,0.25), 0 0 0 1px rgba(45,212,191,0.2);
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 46px;
}

.lp-submit:hover:not(:disabled) {
  opacity: 0.92;
  transform: translateY(-1px);
  box-shadow: 0 8px 28px rgba(45,212,191,0.35), 0 0 0 1px rgba(45,212,191,0.3);
}

.lp-submit:active:not(:disabled) { transform: translateY(0); }
.lp-submit:disabled { opacity: 0.45; cursor: not-allowed; }

/* Spinner */
.lp-spin {
  width: 18px;
  height: 18px;
  border: 2px solid rgba(5,16,20,0.3);
  border-top-color: rgba(5,16,20,0.9);
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
  display: inline-block;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Error */
.lp-error {
  margin-top: 14px;
  font-size: 12px;
  color: #f87171;
  text-align: center;
  padding: 8px 12px;
  background: rgba(248,113,113,0.08);
  border: 1px solid rgba(248,113,113,0.18);
  border-radius: 7px;
  width: 100%;
}

/* ── Responsive ─────────────────────────────────────────── */
@media (max-width: 768px) {
  .lp-left { display: none; }
  .lp-right { width: 100%; padding: 24px; }
}

@media (prefers-reduced-motion: reduce) {
  .lp-step,
  .lp-step-pulse,
  .lp-logo-ring,
  .lp-statusbar-dot,
  .lp-spin { animation: none; }
  .lp-step { opacity: 1; transform: none; }
}
</style>
