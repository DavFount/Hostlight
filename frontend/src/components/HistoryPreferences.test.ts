import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {flushPromises,mount,type VueWrapper} from '@vue/test-utils'
import HistoryPreferences from './HistoryPreferences.vue'
const api={GetIncidents:vi.fn(),SetHistoryRetention:vi.fn()}
let w:VueWrapper
beforeEach(()=>{vi.resetAllMocks();api.GetIncidents.mockResolvedValue({retentionDays:90,error:''});api.SetHistoryRetention.mockResolvedValue(undefined);Object.assign(window,{go:{main:{App:api}}})})
afterEach(()=>w?.unmount())
describe('history retention confirmation',()=>{
 it('cancels reduction without deleting records and restores the selected value',async()=>{
  w=mount(HistoryPreferences);await flushPromises();await w.get('select').setValue('7');await flushPromises()
  expect(w.get('dialog').text()).toContain('permanently removed')
  expect(api.SetHistoryRetention).not.toHaveBeenCalled()
  await w.get('.confirmation-actions button').trigger('click');await flushPromises()
  expect((w.get('select').element as HTMLSelectElement).value).toBe('90')
  expect(api.SetHistoryRetention).not.toHaveBeenCalled()
 })
 it('reduces retention only once after confirmation',async()=>{
  w=mount(HistoryPreferences);await flushPromises();await w.get('select').setValue('7');await flushPromises()
  await w.get('.confirm-destructive').trigger('click');await flushPromises()
  expect(api.SetHistoryRetention).toHaveBeenCalledExactlyOnceWith(7)
 })
 it('increases retention without a destructive confirmation',async()=>{
  w=mount(HistoryPreferences);await flushPromises();await w.get('select').setValue('365');await flushPromises()
  expect(api.SetHistoryRetention).toHaveBeenCalledExactlyOnceWith(365)
  expect((w.get('dialog').element as HTMLDialogElement).open).toBe(false)
 })
 it('cancels a pending request when navigating away',async()=>{
  w=mount(HistoryPreferences);await flushPromises();await w.get('select').setValue('7');await flushPromises();w.unmount();await flushPromises()
  expect(api.SetHistoryRetention).not.toHaveBeenCalled()
 })
 it('restores retention and exposes API failures after confirmation',async()=>{
  api.SetHistoryRetention.mockRejectedValueOnce(new Error('History write failed'))
  w=mount(HistoryPreferences);await flushPromises();await w.get('select').setValue('7');await flushPromises()
  await w.get('.confirm-destructive').trigger('click');await flushPromises()
  expect(w.get('[role="alert"]').text()).toContain('History write failed')
  expect((w.get('select').element as HTMLSelectElement).value).toBe('90')
  expect(w.get('select').attributes('disabled')).toBeUndefined()
 })
})
