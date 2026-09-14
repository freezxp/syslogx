export type LogRow = Record<string, unknown> & {
  _time?: string
  timestamp?: string
  _msg?: string
  message?: string
  hostname?: string
  severity_name?: string
  app_name?: string
  source_ip?: string
  facility_name?: string
  format?: string
}

export interface RecentResponse { data: LogRow[] }
export interface ReadyResponse { status: 'ready' | 'not_ready'; checks: Record<string, unknown> }

async function request<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {signal, headers: {Accept: 'application/json'}})
  if (!response.ok) throw new Error(`${response.status} ${response.statusText}`)
  return response.json() as Promise<T>
}

export const api = {
  recent: (signal?: AbortSignal) => request<RecentResponse>('/api/v1/system/logs/recent?limit=1000', signal),
  ready: async (signal?: AbortSignal): Promise<ReadyResponse> => {
    const response = await fetch('/ready', {signal, headers: {Accept: 'application/json'}})
    const body = await response.json() as ReadyResponse
    if (response.status !== 200 && response.status !== 503) throw new Error(`${response.status} ${response.statusText}`)
    return body
  }
}

export const value = (row: LogRow, key: string): string => {
  const raw = row[key]
  if (raw === null || raw === undefined) return '—'
  if (typeof raw === 'object') return JSON.stringify(raw)
  return String(raw)
}
