import { describe, expect, it } from 'vitest'
import { value, type LogRow } from './api'

describe('log field formatting', () => {
  it('preserves scalar values and distinguishes missing fields', () => {
    const row: LogRow = {hostname: 'fw01', priority: 164, enabled: false}
    expect(value(row, 'hostname')).toBe('fw01')
    expect(value(row, 'priority')).toBe('164')
    expect(value(row, 'enabled')).toBe('false')
    expect(value(row, 'missing')).toBe('—')
  })

  it('formats structured values as JSON', () => {
    expect(value({field: {vendor: 'acme'}}, 'field')).toBe('{"vendor":"acme"}')
  })
})
