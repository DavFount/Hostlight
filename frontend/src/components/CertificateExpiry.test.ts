import {describe,expect,it} from 'vitest'
import {mount} from '@vue/test-utils'
import CertificateExpiry from './CertificateExpiry.vue'
const now=new Date('2026-10-10T12:00:00Z').getTime()
describe('verified certificate expiry',()=>{
 it('hides absent or invalid metadata',()=>{expect(mount(CertificateExpiry,{props:{now}}).find('p').exists()).toBe(false);expect(mount(CertificateExpiry,{props:{now,expires:'bad'}}).find('p').exists()).toBe(false)})
 it('shows the last verified expiry and remaining days',()=>{const wrapper=mount(CertificateExpiry,{props:{now,expires:'2026-10-12T12:00:00Z',minimumDays:0}});expect(wrapper.text()).toContain('2 days remaining');expect(wrapper.classes()).not.toContain('warning');expect(wrapper.attributes('title')).toContain('Last verified server certificate expiry')})
 it('highlights an enabled minimum-validity failure',()=>{const wrapper=mount(CertificateExpiry,{props:{now,expires:'2026-10-12T12:00:00Z',minimumDays:30}});expect(wrapper.classes()).toContain('warning')})
 it('does not warn at the exact minimum boundary',()=>{const wrapper=mount(CertificateExpiry,{props:{now,expires:'2026-10-12T12:00:00Z',minimumDays:2}});expect(wrapper.classes()).not.toContain('warning')})
 it('distinguishes expired cached information',()=>{const wrapper=mount(CertificateExpiry,{props:{now,expires:'2026-10-09T12:00:00Z'}});expect(wrapper.text()).toContain('expiry reached');expect(wrapper.classes()).toContain('warning')})
})
