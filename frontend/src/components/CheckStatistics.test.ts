import {describe,expect,it} from 'vitest'
import {mount} from '@vue/test-utils'
import CheckStatistics from './CheckStatistics.vue'

describe('recent check statistics',()=>{
 it('distinguishes unavailable statistics from a fresh monitor',async()=>{
  const w=mount(CheckStatistics)
  expect(w.text()).toContain('unavailable')
  expect(w.text()).not.toContain('0.0%')
  await w.setProps({statistics:{checks:0,failed:0}})
  expect(w.text()).toContain('No completed availability checks yet')
 })
 it('labels failed checks and the actual session window without claiming packet loss or uptime',()=>{
  const w=mount(CheckStatistics,{props:{statistics:{checks:3,failed:1}}})
  expect(w.text()).toContain('1 / 3 failed checks · 33.3%')
  expect(w.text()).toContain('Last 3 checks · this session')
  expect(w.attributes('title')).toContain('Local setup errors')
 })
 it('updates after recovery and uses singular check wording',async()=>{
  const w=mount(CheckStatistics,{props:{statistics:{checks:1,failed:1}}})
  expect(w.text()).toContain('Last 1 check · this session')
  await w.setProps({statistics:{checks:2,failed:1}})
  expect(w.text()).toContain('1 / 2 failed checks · 50.0%')
 })
})
