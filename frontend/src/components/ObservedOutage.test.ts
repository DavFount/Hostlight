import {describe,it,expect} from 'vitest'
import {mount} from '@vue/test-utils'
import ObservedOutage from './ObservedOutage.vue'
describe('observed outage',()=>{
 it('formats long observations and holds paused durations',async()=>{
  const w=mount(ObservedOutage,{props:{seconds:90061,state:'down'}})
  expect(w.text()).toContain('1d 1h 1m 1s')
  await w.setProps({state:'paused'})
  expect(w.text()).toContain('duration held')
  expect(w.text()).toContain('1d 1h 1m 1s')
  await w.setProps({state:'unknown'})
  expect(w.text()).toContain('Awaiting fresh checks')
 })
 it('distinguishes zero duration from unavailable or ended incidents',async()=>{
  const w=mount(ObservedOutage,{props:{state:'down'}})
  expect(w.text()).toBe('')
  await w.setProps({seconds:0})
  expect(w.text()).toContain('0s')
  for(const state of ['up','error']){await w.setProps({state});expect(w.text()).toBe('')}
 })
})
