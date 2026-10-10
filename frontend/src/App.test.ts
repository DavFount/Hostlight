import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {flushPromises,mount,type VueWrapper} from '@vue/test-utils'
import App from './App.vue'
const target={id:'1',name:'Gateway',host:'192.0.2.1',group:'Office',type:'icmp',port:0,interval:5,timeout:3,failures:3,recovery:2,state:'down',latency:0,message:'Request timed out',checked:'2026-10-09T12:00:00Z'}
const silence={id:'window-1',targetId:'1',kind:'maintenance',starts:'2026-10-09T11:00:00Z',ends:'2026-10-09T14:00:00Z',reason:'Planned upgrade'}
const api={GetTargets:vi.fn(),GetConfigurationStatus:vi.fn(),GetAlertSilences:vi.fn()}
let wrapper:VueWrapper
beforeEach(()=>{vi.clearAllMocks();vi.useFakeTimers({toFake:['Date']});vi.setSystemTime(new Date('2026-10-09T12:00:00Z'));api.GetTargets.mockResolvedValue([target]);api.GetConfigurationStatus.mockResolvedValue({path:'',error:'',historyError:''});api.GetAlertSilences.mockResolvedValue([silence]);Object.assign(window,{go:{main:{App:api}}})})
afterEach(()=>{wrapper?.unmount();vi.useRealTimers()})
describe('monitor alert controls',()=>{
 it('shows suppression separately from availability and opens monitor-specific controls',async()=>{wrapper=mount(App);await flushPromises();const card=wrapper.find('.grid article');expect(card.find('.state').text()).toContain('Unavailable');expect(card.find('.silence-badge').text()).toContain('Alerts silenced until');await card.findAll('button').find(b=>b.text()==='Silence alerts')!.trigger('click');expect(wrapper.find('[role="dialog"]').text()).toContain('Gateway');expect(wrapper.find('[role="dialog"]').text()).toContain('Planned upgrade')})
 it('labels upcoming maintenance without calling it active',async()=>{api.GetAlertSilences.mockResolvedValue([{...silence,starts:'2026-10-09T13:00:00Z'}]);wrapper=mount(App);await flushPromises();expect(wrapper.find('.silence-badge').text()).toContain('Maintenance scheduled:');expect(wrapper.find('.silence-badge').text()).not.toContain('silenced until')})
 it('keeps the monitor overview available if alert storage fails',async()=>{api.GetAlertSilences.mockRejectedValue('Storage unavailable');wrapper=mount(App);await flushPromises();expect(wrapper.find('.grid article').text()).toContain('Gateway');expect(wrapper.find('[role="alert"]').text()).toContain('Alert controls unavailable')})
})
