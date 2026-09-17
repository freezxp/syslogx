import { describe, expect, it } from 'vitest'
import { pageFromPath } from './App'

describe('pageFromPath', () => {
  it.each([
    ['/', 'dashboard'],
    ['/dashboard', 'dashboard'],
    ['/logs', 'logs'],
    ['/logs/live', 'live'],
    ['/sources', 'sources'],
    ['/searches', 'searches'],
    ['/settings/system', 'system']
  ])('opens %s as %s', (path, expected) => {
    expect(pageFromPath(path)).toBe(expected)
  })
})
