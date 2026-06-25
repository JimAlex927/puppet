export function fmtDate(value?: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

export function fmtDuration(ms?: number) {
  if (!ms) return '-'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}
