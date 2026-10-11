import {afterEach,expect,it,vi} from 'vitest'
import {flushPromises,mount} from '@vue/test-utils'
import SchedulingControls from './SchedulingControls.vue'
afterEach(()=>{delete (window as any).go})
it('saves settings and explains restart without claiming active changes',async()=>{
 const initial={saved:{concurrency:32,spreadMilliseconds:250},active:{concurrency:32,spreadMilliseconds:250},restartRequired:false}
 const get=vi.fn().mockResolvedValueOnce(initial).mockResolvedValue({...initial,saved:{concurrency:8,spreadMilliseconds:500},restartRequired:true})
 const save=vi.fn().mockResolvedValue(undefined)
 ;(window as any).go={main:{App:{GetSchedulingSettings:get,SaveSchedulingSettings:save}}}
 const w=mount(SchedulingControls);await flushPromises()
 const inputs=w.findAll('input');await inputs[0]!.setValue(8);await inputs[1]!.setValue(500)
 await w.get('form').trigger('submit');await flushPromises()
 expect(save).toHaveBeenCalledWith({concurrency:8,spreadMilliseconds:500})
 expect(w.text()).toContain('Active limit: 32');expect(w.get('[role="status"]').text()).toContain('Quit and reopen')
 w.unmount()
})
it('does not report success when saving fails',async()=>{
 ;(window as any).go={main:{App:{GetSchedulingSettings:vi.fn().mockResolvedValue({saved:{concurrency:32,spreadMilliseconds:250},active:{concurrency:32,spreadMilliseconds:250},restartRequired:false}),SaveSchedulingSettings:vi.fn().mockRejectedValue('Unable to save')}}}
 const w=mount(SchedulingControls);await flushPromises();await w.get('form').trigger('submit');await flushPromises()
 expect(w.find('[role="status"]').exists()).toBe(false);expect(w.get('[role="alert"]').text()).toContain('Unable to save');w.unmount()
})
