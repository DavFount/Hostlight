import {afterEach,describe,expect,it,vi} from 'vitest'
import {flushPromises,mount} from '@vue/test-utils'
import VisualAlerts from './VisualAlerts.vue'
afterEach(()=>{vi.useRealTimers();localStorage.clear();delete (window as any).go})
describe('visual alerts',()=>{
 it('shows outage details and recovery duration without rendering markup',async()=>{
  vi.useFakeTimers();const get=vi.fn().mockResolvedValueOnce({cursor:0,items:[],dropped:false}).mockResolvedValue({cursor:2,items:[{sequence:1,name:'Router',host:'192.0.2.10',reason:'<b>Connection refused</b>',event:'down',time:new Date().toISOString()},{sequence:2,name:'Router',host:'192.0.2.10',durationSeconds:3665,event:'up',time:new Date().toISOString()}],dropped:false});(window as any).go={main:{App:{GetVisualAlerts:get}}};const w=mount(VisualAlerts);await flushPromises();await vi.advanceTimersByTimeAsync(1000);expect(w.text()).toContain('192.0.2.10');expect(w.text()).toContain('<b>Connection refused</b>');expect(w.find('.alert-detail b').exists()).toBe(false);expect(w.text()).toContain('Observed outage: 1h 1m 5s');w.unmount()
 })
 it('baselines, displays transitions, and auto closes',async()=>{
  vi.useFakeTimers();const get=vi.fn().mockResolvedValueOnce({cursor:0,items:[],dropped:false}).mockResolvedValueOnce({cursor:1,items:[{sequence:1,name:'Router',event:'down',time:new Date().toISOString()}],dropped:false}).mockResolvedValue({cursor:1,items:[],dropped:false});(window as any).go={main:{App:{GetVisualAlerts:get}}}
  const w=mount(VisualAlerts);await flushPromises();expect(get).toHaveBeenCalledWith(0,true);await vi.advanceTimersByTimeAsync(1000);expect(w.text()).toContain('Monitor unavailable');await vi.advanceTimersByTimeAsync(10000);expect(w.find('.visual-popup').exists()).toBe(false);w.unmount()
 })
 it('supports persistent previews and dismissal',async()=>{
  vi.useFakeTimers();localStorage.setItem('hostlight.visual.seconds','0');const w=mount(VisualAlerts);await w.get('.visual-controls>button').trigger('click');const preview=w.findAll('button').find(b=>b.text()==='Preview popup')!;await preview.trigger('click');await vi.advanceTimersByTimeAsync(60000);expect(w.find('.visual-popup').exists()).toBe(true);await w.get('.visual-popup button').trigger('click');expect(w.find('.visual-popup').exists()).toBe(false);w.unmount()
 })
})
