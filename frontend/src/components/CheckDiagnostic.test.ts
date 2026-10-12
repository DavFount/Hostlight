import {describe,expect,it} from 'vitest'
import {mount} from '@vue/test-utils'
import CheckDiagnostic from './CheckDiagnostic.vue'

describe('check diagnostics',()=>{
 it.each([
  ['dial tcp 192.0.2.1:443: connectex: No connection could be made because the target machine actively refused it.','connection was refused'],
  ['context deadline exceeded','before the timeout'],
  ['Destination unreachable (ICMP status 11003)','could not be reached'],
  ['Ping TTL expired','hop limit'],
  ['dial tcp: lookup missing.example: no such host','hostname could not be resolved'],
  ['connectex: An attempt was made to access a socket in a way forbidden by its access permissions.','local permissions'],
  ['Windows could not open a ping handle: error 123','Ping could not run'],
  ['dial tcp: an unfamiliar platform error','network check failed'],
 ])('explains %s and preserves the original diagnostic', (message,expected)=>{
  const w=mount(CheckDiagnostic,{props:{message}})
  expect(w.get('.diagnostic').text()).toContain(expected)
  expect(w.get('details').attributes('open')).toBeUndefined()
  expect(w.get('summary').text()).toBe('Technical details')
  expect(w.get('details p').text()).toBe(message)
 })
 it.each(['TCP connection successful','Ping reply received','Monitoring paused','Waiting for checks','HTTP 503 · Expected 200','HTTPS certificate is not trusted','A new diagnostic'])('preserves existing readable messages: %s',message=>{
  const w=mount(CheckDiagnostic,{props:{message}})
  expect(w.get('.diagnostic').text()).toBe(message)
  expect(w.find('details').exists()).toBe(false)
 })
 it('renders untrusted diagnostics as text and keeps elapsed timing in the summary',()=>{
  const message='dial tcp: <img src=x onerror=alert(1)>'
  const w=mount(CheckDiagnostic,{props:{message,prefix:'Connection attempt failed after 12.9 ms · '}})
  expect(w.get('.diagnostic').text()).toContain('failed after 12.9 ms')
  expect(w.get('details p').text()).toBe(message)
  expect(w.find('img').exists()).toBe(false)
 })
})
