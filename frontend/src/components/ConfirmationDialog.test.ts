import {afterEach,describe,expect,it} from 'vitest'
import {flushPromises,mount,type VueWrapper} from '@vue/test-utils'
import ConfirmationDialog from './ConfirmationDialog.vue'
let w:VueWrapper|undefined
afterEach(()=>{w?.unmount();w=undefined;document.body.innerHTML=''})
describe('custom confirmation dialog',()=>{
 it('uses the modal dialog top layer, names its content, and initially focuses Cancel',async()=>{
  w=mount(ConfirmationDialog,{attachTo:document.body,props:{request:{title:'Remove monitor?',message:'Remove <b>Gateway</b>?',confirmLabel:'Remove monitor',destructive:true}}})
  await flushPromises()
  expect((w.get('dialog').element as HTMLDialogElement).open).toBe(true)
  expect(w.get('dialog').attributes('aria-labelledby')).toBe(w.get('h2').attributes('id'))
  expect(w.get('dialog').attributes('aria-describedby')).toBe(w.get('p').attributes('id'))
  expect(document.activeElement).toBe(w.findAll('button')[0]!.element)
  expect(w.find('b').exists()).toBe(false)
  expect(w.get('p').text()).toContain('<b>Gateway</b>')
 })
 it('cancels with Escape and restores focus when the parent clears the request',async()=>{
  const opener=document.createElement('button');document.body.append(opener);opener.focus()
  w=mount(ConfirmationDialog,{attachTo:document.body})
  await w.setProps({request:{title:'Remove?',message:'Test'}});await flushPromises()
  await w.get('dialog').trigger('cancel')
  expect(w.emitted('resolve')).toEqual([[false]])
  await w.setProps({request:undefined});await flushPromises()
  expect((w.get('dialog').element as HTMLDialogElement).open).toBe(false)
  expect(document.activeElement).toBe(opener)
 })
 it('emits confirmation only once for repeated clicks',async()=>{
  w=mount(ConfirmationDialog,{attachTo:document.body,props:{request:{title:'Remove?',message:'Test'}}})
  await flushPromises()
  await w.findAll('button')[1]!.trigger('click');await w.findAll('button')[1]!.trigger('click')
  expect(w.emitted('resolve')).toEqual([[true]])
 })
 it('supports themed acknowledgement messages without a Cancel button',async()=>{
  w=mount(ConfirmationDialog,{attachTo:document.body,props:{request:{title:'Information',message:'Done',mode:'alert'}}})
  await flushPromises()
  expect(w.findAll('button')).toHaveLength(1)
  expect(w.get('button').text()).toBe('OK')
  expect(document.activeElement).toBe(w.get('button').element)
 })
})
