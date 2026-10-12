import {describe,expect,it} from 'vitest'
import {mount} from '@vue/test-utils'
import CheckStatistics from './CheckStatistics.vue'

const empty={started:'2026-10-12T00:00:00Z',sessionChecks:0,sessionPassed:0,sessionFailed:0,setupErrors:0};
describe('recent check statistics',()=>{
 it('collapses supporting detail and keeps setup errors visible in its summary',async()=>{
  const w=mount(CheckStatistics,{props:{statistics:{...empty,checks:1,failed:0,sessionChecks:1,sessionPassed:1,setupErrors:3}}})
  expect(w.get('details').attributes('open')).toBeUndefined()
  expect(w.get('summary').text()).toBe('Session details · 3 setup errors')
  ;(w.get('details').element as HTMLDetailsElement).open=true
  await w.setProps({statistics:{...empty,checks:2,failed:0,sessionChecks:2,sessionPassed:2,setupErrors:3}})
  expect((w.get('details').element as HTMLDetailsElement).open).toBe(true)
 })
 it('shows overnight session totals independently of the recent window and labels resets',()=>{
  const w=mount(CheckStatistics,{props:{statistics:{...empty,checks:100,failed:0,sessionChecks:20020,sessionPassed:20000,sessionFailed:20,setupErrors:3}}})
  const session=w.get('.session-statistics').text()
  expect(session).toContain((20020).toLocaleString()+' checks')
  expect(w.get('.statistics-values').text()).toContain((20000).toLocaleString())
  expect(w.findAll('dd')[1]!.text()).toBe('20')
  expect(session).toContain('0.10%')
  expect(w.get('.statistics-details').text()).toContain('3 DNS / setup errors')
  expect(w.get('.statistics-details').text()).toContain('Resets on edit or app restart')
  expect(w.get('time').attributes('datetime')).toBe(empty.started)
  expect(w.get('.recent-statistics').text()).toContain('0 / 100 failed · 0.0%')
 })
 it('shows setup errors with no availability observations without inventing a failure percentage',()=>{
  const w=mount(CheckStatistics,{props:{statistics:{...empty,checks:0,failed:0,setupErrors:5}}})
  expect(w.get('.statistics-details').text()).toContain('5 DNS / setup errors')
  expect(w.text()).toContain('No completed availability checks yet')
  expect(w.text()).not.toContain('0.00%')
 })
 it('does not round a nonzero session failure rate to zero',()=>{
  const w=mount(CheckStatistics,{props:{statistics:{...empty,checks:100,failed:0,sessionChecks:1000000,sessionPassed:999999,sessionFailed:1}}})
  expect(w.get('.session-statistics').text()).toContain('<0.01%')
 })
 it('distinguishes unavailable statistics from a fresh monitor',async()=>{
  const w=mount(CheckStatistics)
  expect(w.text()).toContain('unavailable')
  expect(w.text()).not.toContain('0.0%')
  await w.setProps({statistics:{...empty,checks:0,failed:0}})
  expect(w.text()).toContain('No completed availability checks yet')
 })
 it('labels failed checks and the actual session window without claiming packet loss or uptime',()=>{
  const w=mount(CheckStatistics,{props:{statistics:{...empty,checks:3,failed:1}}})
  expect(w.text()).toContain('1 / 3 failed · 33.3%')
  expect(w.get('.recent-statistics').attributes('title')).toBe('Last 3 checks · this session')
  expect(w.attributes('title')).toContain('Local setup errors')
 })
 it('updates after recovery and uses singular check wording',async()=>{
  const w=mount(CheckStatistics,{props:{statistics:{...empty,checks:1,failed:1}}})
  expect(w.get('.recent-statistics').attributes('title')).toBe('Last 1 check · this session')
  await w.setProps({statistics:{...empty,checks:2,failed:1}})
  expect(w.text()).toContain('1 / 2 failed · 50.0%')
 })
})
