import {afterEach,describe,expect,it,vi} from 'vitest'
import {flushPromises,mount} from '@vue/test-utils'
import ProbeHistory from './ProbeHistory.vue'
const bucket={time:'2026-10-12T12:00:00Z',checks:4,failed:1,meanMS:20,minMS:10,maxMS:30,types:'tcp'}
const page={items:[bucket],bucketMinutes:12,error:'',from:'2026-10-11T12:00:00Z',until:'2026-10-12T12:12:00Z'}
function setup(){const api={GetProbeHistoryMonitors:vi.fn().mockResolvedValue([{id:'1',name:'Router'},{id:'2',name:'Server'}]),GetProbeHistory:vi.fn().mockResolvedValue(page)};vi.stubGlobal('go',{main:{App:api}});return api}
async function expand(w:ReturnType<typeof mount>){(w.get('details').element as HTMLDetailsElement).open=true;await w.get('details').trigger('toggle');await flushPromises()}
afterEach(()=>{vi.useRealTimers();vi.unstubAllGlobals()})
describe('saved probe history',()=>{
 it('keeps tiny nonzero failure percentages visible',async()=>{
  const api=setup();api.GetProbeHistory.mockResolvedValue({...page,items:[{...bucket,checks:1000000,failed:1}]})
  const w=mount(ProbeHistory);await expand(w)
  expect(w.get('.probe-history-summary').text()).toContain('<0.01% failed')
  expect(w.get('tbody').text()).toContain('<0.01')
  w.unmount()
 })
 it('loads only while expanded, labels aggregate semantics and separates latency from failures',async()=>{
  const api=setup();const w=mount(ProbeHistory)
  expect(api.GetProbeHistoryMonitors).not.toHaveBeenCalled()
  await expand(w)
  expect(api.GetProbeHistory).toHaveBeenCalledWith('1',24)
  expect(w.get('.probe-history-summary').text()).toContain('4 completed checks · 1 failed · 25.00% failed')
  expect(w.findAll('.duration-range')).toHaveLength(1)
  expect(w.findAll('.failed-bar')).toHaveLength(1)
  expect(w.get('svg').attributes('aria-label')).toContain('4 completed checks, 1 failed')
  expect(w.text()).toContain('Not uptime or packet loss')
  expect(w.text()).toContain('50,000')
  w.unmount()
 })
 it('does not fabricate latency for all-failed intervals and shows storage warnings',async()=>{
  const api=setup();api.GetProbeHistory.mockResolvedValue({...page,error:'Storage queue full',items:[{...bucket,checks:1,failed:1,meanMS:0,minMS:0,maxMS:0}]})
  const w=mount(ProbeHistory);await expand(w)
  expect(w.find('.duration-range').exists()).toBe(false)
  expect(w.find('.duration-mean').exists()).toBe(false)
  expect(w.get('tbody').text()).toContain('———')
  expect(w.get('[role="alert"]').text()).toContain('Storage queue full')
  w.unmount()
 })
 it('ignores old requests when changing monitor/range and stops polling when closed',async()=>{
  vi.useFakeTimers();const api=setup();const w=mount(ProbeHistory);await expand(w)
  let finish!:(value:typeof page)=>void
  api.GetProbeHistory.mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve}))
  await w.findAll('select')[0]!.setValue('2');await flushPromises()
  await w.findAll('select')[1]!.setValue('1');await flushPromises()
  expect(api.GetProbeHistory).toHaveBeenLastCalledWith('2',1)
  finish({...page,items:[{...bucket,checks:999}]});await flushPromises()
  expect(w.text()).not.toContain('999 completed')
  ;(w.get('details').element as HTMLDetailsElement).open=false
  await w.get('details').trigger('toggle');await flushPromises()
  const calls=api.GetProbeHistory.mock.calls.length
  await vi.advanceTimersByTimeAsync(90000)
  expect(api.GetProbeHistory).toHaveBeenCalledTimes(calls)
  w.unmount()
 })
 it('distinguishes empty observations from unavailable storage',async()=>{
  const api=setup();api.GetProbeHistoryMonitors.mockResolvedValue([])
  const w=mount(ProbeHistory);await expand(w)
  expect(w.text()).toContain('No saved checks in this range')
  api.GetProbeHistoryMonitors.mockRejectedValue(new Error('disk'))
  await w.get('button').trigger('click');await flushPromises()
  expect(w.get('[role="alert"]').text()).toContain('Probe history unavailable')
  w.unmount()
 })
})
