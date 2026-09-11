import { computed, ref } from 'vue'

export type ThemeMode = 'dark' | 'light'

const STORAGE_KEY = 'puppet_theme'
const theme = ref<ThemeMode>(readTheme())

function readTheme(): ThemeMode {
  if (typeof window === 'undefined') return 'dark'
  return localStorage.getItem(STORAGE_KEY) === 'light' ? 'light' : 'dark'
}

function applyTheme(mode: ThemeMode) {
  if (typeof document === 'undefined') return
  document.documentElement.dataset.theme = mode
}

applyTheme(theme.value)

export function useTheme() {
  function setTheme(mode: ThemeMode) {
    theme.value = mode
    localStorage.setItem(STORAGE_KEY, mode)
    applyTheme(mode)
  }

  function toggleTheme() {
    setTheme(theme.value === 'dark' ? 'light' : 'dark')
  }

  return {
    theme,
    isLightTheme: computed(() => theme.value === 'light'),
    setTheme,
    toggleTheme,
  }
}
