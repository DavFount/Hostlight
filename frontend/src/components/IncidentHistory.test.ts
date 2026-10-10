import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {flushPromises,mount,type VueWrapper} from '@vue/test-utils'
import IncidentHistory from './IncidentHistory.vue'

const incident={id:1,name:'Office gateway',host:'192.168.1.1',group:'Office',type:'icmp',port:0,opened:'2026-10-09T12:00:00Z',ended:'2026-10-09T12:01:05Z',lastSeen:'2026-10-09T12:01:05Z',status:'interrupted',reason:'Monitoring paused',durationSeconds:65}
const page=()=>({items:[incident],total:1,error:'',path:'C:/settings/history.db',retentionDays:90})
let wrapper:VueWrapper
const api={GetIncidents:vi.fn(),SetHistoryRetention:vi.fn(),ExportIncidents:vi.fn()}
beforeEach(()=>{vi.clearAllMocks();api.GetIncidents.mockResolvedValue(page());api.SetHistoryRetention.mockResolvedValue(undefined);api.ExportIncidents.mockResolvedValue('C:/exports/incidents.csv');Object.assign(window,{go:{main:{App:api}}})})
afterEach(()=>{wrapper?.unmount();vi.restoreAllMocks();vi.unstubAllGlobals();vi.useRealTimers()})

describe('incident history',()=>{
 it('shows interrupted monitoring separately from recovery and labels observed duration',async()=>{
  wrapper=mount(IncidentHistory);await flushPromises()
  expect(wrapper.find('tbody').text()).toContain('Interrupted')
  expect(wrapper.find('tbody').text()).toContain('1m 5s')
  expect(wrapper.text()).toContain('Observed duration')
  expect(wrapper.text()).toContain('It does not confirm recovery')
 })
 it('exports the active filters through the native save workflow',async()=>{
  wrapper=mount(IncidentHistory);await flushPromises()
  await wrapper.findAll('button').find(button=>button.text()==='Export CSV')!.trigger('click');await flushPromises()
  expect(api.ExportIncidents).toHaveBeenCalledWith('','all',30)
  expect(wrapper.text()).toContain('Exported to C:/exports/incidents.csv')
 })
 it('does not remove retained data if the reduction is canceled',async()=>{
  wrapper=mount(IncidentHistory);await flushPromises();vi.stubGlobal('confirm',vi.fn().mockReturnValue(false))
  await wrapper.find('.history-settings select').setValue('7');await flushPromises()
  expect(api.SetHistoryRetention).not.toHaveBeenCalled()
  expect((wrapper.find('.history-settings select').element as HTMLSelectElement).value).toBe('90')
 })
 it('applies outcome and date filters to the first page',async()=>{
  wrapper=mount(IncidentHistory);await flushPromises();vi.useFakeTimers({toFake:['setTimeout','clearTimeout']})
  await wrapper.find('[aria-label="Incident status"]').setValue('resolved')
  await wrapper.find('[aria-label="History date range"]').setValue('7')
  await vi.advanceTimersByTimeAsync(200);await flushPromises()
  expect(api.GetIncidents).toHaveBeenLastCalledWith('','resolved',7,0)
 })
 it('saves a confirmed retention reduction',async()=>{
  wrapper=mount(IncidentHistory);await flushPromises();vi.stubGlobal('confirm',vi.fn().mockReturnValue(true))
  api.GetIncidents.mockResolvedValue({...page(),retentionDays:7})
  await wrapper.find('.history-settings select').setValue('7');await flushPromises()
  expect(api.SetHistoryRetention).toHaveBeenCalledWith(7)
 })
 it('pages history without changing filters',async()=>{
  api.GetIncidents.mockResolvedValue({...page(),total:101})
  wrapper=mount(IncidentHistory);await flushPromises()
  await wrapper.findAll('button').find(button=>button.text()==='Next')!.trigger('click');await flushPromises()
  expect(api.GetIncidents).toHaveBeenLastCalledWith('','all',30,100)
 })
 it('makes database errors visible instead of showing a successful history load',async()=>{
  api.GetIncidents.mockRejectedValue(new Error('History could not be read'))
  wrapper=mount(IncidentHistory);await flushPromises()
  expect(wrapper.find('[role="alert"]').text()).toContain('History could not be read')
 })
})
