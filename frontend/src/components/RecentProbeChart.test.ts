import {afterEach,describe,expect,it,vi} from 'vitest'
import {flushPromises,mount} from '@vue/test-utils'
import RecentProbeChart from './RecentProbeChart.vue'
const sample=(latencyMS:number,failed=false,breakBefore=false)=>({time:'2026-10-12T12:00:00Z',latencyMS,failed,breakBefore})
async function expand(w:ReturnType<typeof mount>){(w.get('details').element as HTMLDetailsElement).open=true;await w.get('details').trigger('toggle');await flushPromises()}
afterEach(()=>{vi.useRealTimers();vi.unstubAllGlobals()})
function api(result:ReturnType<typeof sample>[]){const get=vi.fn().mockResolvedValue(result);vi.stubGlobal('go',{main:{App:{GetRecentProbeSamples:get}}});return get}
describe('recent probe chart',()=>{
 it('fetches only when expanded and separates failed checks from successful latency',async()=>{
  const get=api([sample(10),sample(999,true),sample(30),sample(20,false,true)])
  const w=mount(RecentProbeChart,{props:{id:'1'}})
  expect(get).not.toHaveBeenCalled()
  await expand(w)
  expect(get).toHaveBeenCalledWith('1')
  expect(w.get('.chart-summary').text()).toContain('1 / 4 failed · 25.0%')
  expect(w.get('.chart-summary').text()).toContain('Mean 20.0 ms · max 30.0 ms')
  expect(w.findAll('polyline')).toHaveLength(3)
  expect(w.findAll('.failure-mark')).toHaveLength(1)
  expect(w.get('svg').attributes('aria-label')).toContain('4 recent checks; 1 failed')
  expect(w.findAll('tbody tr')[1]!.text()).toContain('Failed—')
  w.unmount()
 })
 it('distinguishes empty, all-failed and unavailable windows',async()=>{
  const get=api([]);const w=mount(RecentProbeChart,{props:{id:'1'}})
  await expand(w);expect(w.text()).toContain('No completed availability checks yet')
  get.mockResolvedValue([sample(0,true)])
  await w.setProps({sessionStart:'new'})
  await flushPromises()
  expect(w.get('svg').attributes('aria-label')).toContain('No successful latency observations')
  expect(w.text()).not.toContain('Mean')
  get.mockRejectedValue(new Error('storage'))
  await w.setProps({sessionStart:'newer'});await flushPromises()
  expect(w.text()).toContain('Recent chart unavailable')
  expect(w.find('svg').exists()).toBe(false)
  w.unmount()
 })
 it('stops polling when closed and ignores a stale response after an edit',async()=>{
  vi.useFakeTimers();const get=api([sample(10)])
  const w=mount(RecentProbeChart,{props:{id:'1',sessionStart:'old'}})
  await expand(w)
  let resolve!:(value:ReturnType<typeof sample>[])=>void
  get.mockImplementationOnce(()=>new Promise(done=>{resolve=done}))
  await vi.advanceTimersByTimeAsync(5000)
  await w.setProps({sessionStart:'new'});await flushPromises()
  resolve([sample(999)]);await flushPromises()
  expect(w.text()).not.toContain('999.0')
  ;(w.get('details').element as HTMLDetailsElement).open=false
  await w.get('details').trigger('toggle');await flushPromises()
  const calls=get.mock.calls.length
  await vi.advanceTimersByTimeAsync(15000)
  expect(get).toHaveBeenCalledTimes(calls)
  w.unmount()
 })
})
