export type Condition = 'snow' | 'rain' | 'cold' | 'hot' | 'normal'

export interface AreaRef {
  id: number
  name: string
}

export interface WindowRef {
  from: string
  to: string
}

export interface Expected {
  now: number
  next: number
  gap: number
  ratio: number
}

export interface WeatherEvidence {
  available: boolean
  condition?: Condition
  multiplier: number
  note?: string
}

export interface PastWeek {
  date: string
  now: number
  next: number
  rose: boolean
}

export interface RunnerUp {
  name: string
  gap: number
  ratio: number
}

export interface Evidence {
  weather: WeatherEvidence
  past_weeks: PastWeek[] | null
  weeks_rose: string
  runners_up: RunnerUp[]
  data_window: string
}

export interface Recommendation {
  action: 'move_drivers' | 'hold'
  headline: string
  area: AreaRef | null
  window: WindowRef
  expected: Expected
  evidence: Evidence
  could_be_wrong: string[]
}

export async function fetchRecommendation(at?: string): Promise<Recommendation> {
  const url = at ? `/api/recommendation?at=${encodeURIComponent(at)}` : '/api/recommendation'
  const res = await fetch(url)
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    throw new Error(body?.error ?? `request failed with status ${res.status}`)
  }
  return res.json()
}

// ratio is "next / now" (e.g. 1.29); shown as the change from today, "+29%".
export function formatPercent(ratio: number): string {
  const pct = Math.round((ratio - 1) * 100)
  return `${pct >= 0 ? '+' : ''}${pct}%`
}

export function formatMultiplier(multiplier: number): string {
  return multiplier === 1 ? 'no adjustment' : formatPercent(multiplier)
}
