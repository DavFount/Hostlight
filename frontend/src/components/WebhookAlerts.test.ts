import {afterEach,beforeEach,describe,expect,it,vi} from 'vitest'
import {flushPromises,mount,type VueWrapper} from '@vue/test-utils'
import WebhookAlerts from './WebhookAlerts.vue'
const settings=()=>({enabled:true,configured:true,url:'',bearer:'',signingKey:'',allowHTTP:false,clearBearer:false,clearSigningKey:false,hasBearer:true,hasSigningKey:false})
const api={GetWebhookSettings:vi.fn(),SaveWebhookSettings:vi.fn(),TestWebhook:vi.fn(),GetWebhookDeliveries:vi.fn()}
let wrapper:VueWrapper
beforeEach(()=>{vi.clearAllMocks();api.GetWebhookSettings.mockResolvedValue(settings());api.SaveWebhookSettings.mockResolvedValue(undefined);api.TestWebhook.mockResolvedValue(undefined);api.GetWebhookDeliveries.mockResolvedValue([{id:'event-id',event:'down',created:'2026-10-09T12:00:00Z',status:'pending',attempts:2,result:'HTTP 503'}]);Object.assign(window,{go:{main:{App:api}}})})
afterEach(()=>wrapper?.unmount())
describe('webhook alerts',()=>{
 it('shows saved-secret placeholders and delivery outcomes',async()=>{wrapper=mount(WebhookAlerts);await flushPromises();expect(wrapper.find('#webhook-url').attributes('placeholder')).toContain('leave blank');expect(wrapper.find('tbody').text()).toContain('Outage');expect(wrapper.find('tbody').text()).toContain('HTTP 503');expect(wrapper.text()).toContain('Initial discovery')})
 it('saves settings without recovering stored secrets',async()=>{wrapper=mount(WebhookAlerts);await flushPromises();await wrapper.find('#webhook-url').setValue('https://example.com/hook');await wrapper.find('form').trigger('submit');await flushPromises();expect(api.SaveWebhookSettings).toHaveBeenCalledWith({...settings(),url:'https://example.com/hook'});expect((wrapper.find('#webhook-url').element as HTMLInputElement).value).toBe('');expect(wrapper.text()).toContain('Settings saved')})
 it('sends a test using saved settings',async()=>{wrapper=mount(WebhookAlerts);await flushPromises();await wrapper.find('button[type="button"]').trigger('click');await flushPromises();expect(api.TestWebhook).toHaveBeenCalledOnce();expect(wrapper.text()).toContain('Test queued')})
 it('shows save failures and retains the draft',async()=>{api.SaveWebhookSettings.mockRejectedValue('Enter a valid HTTPS webhook URL');wrapper=mount(WebhookAlerts);await flushPromises();await wrapper.find('#webhook-url').setValue('bad');await wrapper.find('form').trigger('submit');await flushPromises();expect(wrapper.find('[role="alert"]').text()).toContain('HTTPS');expect((wrapper.find('#webhook-url').element as HTMLInputElement).value).toBe('bad')})
})
