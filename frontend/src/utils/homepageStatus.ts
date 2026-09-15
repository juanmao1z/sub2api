export interface UptimeParts {
  years: number
  months: number
  days: number
  hours: number
  minutes: number
  seconds: number
}

const BEIJING_OFFSET_MS = 8 * 60 * 60 * 1000
const DAY_MS = 24 * 60 * 60 * 1000

function shiftedToBeijing(date: Date): Date {
  return new Date(date.getTime() + BEIJING_OFFSET_MS)
}

function daysInUTCMonth(year: number, month: number): number {
  return new Date(Date.UTC(year, month + 1, 0)).getUTCDate()
}

function addCalendar(date: Date, years: number, months: number): Date {
  const targetMonthIndex = date.getUTCMonth() + months
  const targetYear = date.getUTCFullYear() + years + Math.floor(targetMonthIndex / 12)
  const targetMonth = ((targetMonthIndex % 12) + 12) % 12
  const targetDay = Math.min(date.getUTCDate(), daysInUTCMonth(targetYear, targetMonth))

  return new Date(Date.UTC(
    targetYear,
    targetMonth,
    targetDay,
    date.getUTCHours(),
    date.getUTCMinutes(),
    date.getUTCSeconds(),
    date.getUTCMilliseconds(),
  ))
}

function zeroUptime(): UptimeParts {
  return { years: 0, months: 0, days: 0, hours: 0, minutes: 0, seconds: 0 }
}

export function calculateBeijingUptime(
  startedAt: string | null,
  now = new Date(),
): UptimeParts | null {
  if (startedAt === null) return null
  const parsedStart = new Date(startedAt)
  if (!Number.isFinite(parsedStart.getTime())) return null

  const start = shiftedToBeijing(parsedStart)
  const end = shiftedToBeijing(now)
  if (end.getTime() <= start.getTime()) return zeroUptime()

  let years = end.getUTCFullYear() - start.getUTCFullYear()
  let cursor = addCalendar(start, years, 0)
  if (cursor > end) {
    years -= 1
    cursor = addCalendar(start, years, 0)
  }

  let months = (
    (end.getUTCFullYear() - cursor.getUTCFullYear()) * 12
    + end.getUTCMonth()
    - cursor.getUTCMonth()
  )
  let monthCursor = addCalendar(cursor, 0, months)
  if (monthCursor > end) {
    months -= 1
    monthCursor = addCalendar(cursor, 0, months)
  }

  let remaining = end.getTime() - monthCursor.getTime()
  const days = Math.floor(remaining / DAY_MS)
  remaining -= days * DAY_MS
  const hours = Math.floor(remaining / (60 * 60 * 1000))
  remaining -= hours * 60 * 60 * 1000
  const minutes = Math.floor(remaining / (60 * 1000))
  remaining -= minutes * 60 * 1000
  const seconds = Math.floor(remaining / 1000)

  return { years, months, days, hours, minutes, seconds }
}

export function formatBeijingStartTime(startedAt: string | null, _locale: string): string {
  if (startedAt === null) return '--'
  const parsed = new Date(startedAt)
  if (!Number.isFinite(parsed.getTime())) return '--'
  const date = shiftedToBeijing(parsed)
  const pad = (value: number) => String(value).padStart(2, '0')
  return [
    `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}`,
    `${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}`,
  ].join(' ')
}

export function formatCompactTokens(value: number | null, locale: string): string {
  if (value === null || !Number.isFinite(value) || value < 0) return '--'
  const units = [
    { threshold: 1_000_000_000_000, suffix: 'T' },
    { threshold: 1_000_000_000, suffix: 'B' },
    { threshold: 1_000_000, suffix: 'M' },
    { threshold: 1_000, suffix: 'K' },
  ]
  const unit = units.find((candidate) => value >= candidate.threshold)
  if (!unit) return new Intl.NumberFormat(locale, { maximumFractionDigits: 0 }).format(value)

  const formatted = new Intl.NumberFormat(locale, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  }).format(value / unit.threshold)
  return `${formatted}${unit.suffix}`
}

export function formatSuccessRate(value: number | null, locale: string): string {
  if (value === null || !Number.isFinite(value) || value < 0 || value > 1) return '--'
  return new Intl.NumberFormat(locale, {
    style: 'percent',
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  }).format(value)
}
