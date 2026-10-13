import {afterEach,expect,it,vi} from 'vitest'
import {flushPromises,mount} from '@vue/test-utils'
import ReleaseLabel from './ReleaseLabel.vue'

afterEach(()=>{delete (window as any).go})

it.each([['0.2.0','v0.2.0'],['1.0.0','v1.0.0'],['early preview','Development build']])('identifies build %s from backend metadata',async(release,expected)=>{
 (window as any).go={main:{App:{GetSupportReport:vi.fn().mockResolvedValue({release})}}}
 const wrapper=mount(ReleaseLabel)
 await flushPromises()
 expect(wrapper.text()).toBe(`HOSTLIGHT · ${expected}`)
 wrapper.unmount()
})

it('reports unavailable metadata without leaking the error',async()=>{
 (window as any).go={main:{App:{GetSupportReport:vi.fn().mockRejectedValue('PRIVATE-PATH')}}}
 const wrapper=mount(ReleaseLabel)
 await flushPromises()
 expect(wrapper.text()).toBe('HOSTLIGHT · Version unavailable')
 wrapper.unmount()
})
