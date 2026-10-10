import {afterEach,describe,expect,it,vi} from 'vitest'
import {mount} from '@vue/test-utils'
import SavedViews from './SavedViews.vue'
import {savedViewsKey} from '../savedViews'
const filters={search:'router',status:'down',group:'Office'}
afterEach(()=>{vi.unstubAllGlobals();localStorage.clear();vi.restoreAllMocks()})
describe('saved monitoring views',()=>{
 it('saves, reloads, and applies filters without altering monitors',async()=>{
  const w=mount(SavedViews,{props:{filters}});await w.get('input').setValue('Office outages');await w.findAll('button').find(b=>b.text()==='Save as new')!.trigger('click');const stored=JSON.parse(localStorage.getItem(savedViewsKey)!);expect(stored.items[0]).toMatchObject({...filters,name:'Office outages'});w.unmount();const next=mount(SavedViews,{props:{filters:{search:'',status:'all',group:null}}});await next.get('select').setValue(stored.items[0].id);expect(next.emitted('apply')?.[0]).toEqual([filters]);next.unmount()
 })
 it('marks modified filters and explicitly updates and renames a view',async()=>{
  const w=mount(SavedViews,{props:{filters}});await w.get('input').setValue('Office');await w.findAll('button').find(b=>b.text()==='Save as new')!.trigger('click');await w.setProps({filters:{...filters,status:'up'}});expect(w.text()).toContain('Filters changed');await w.get('input').setValue('Office healthy');await w.findAll('button').find(b=>b.text()==='Update selected')!.trigger('click');expect(w.text()).not.toContain('Filters changed');expect(JSON.parse(localStorage.getItem(savedViewsKey)!).items).toHaveLength(1);expect(JSON.parse(localStorage.getItem(savedViewsKey)!).items[0].name).toBe('Office healthy');w.unmount()
 })
 it('validates names, confirms deletion, and leaves active filters unchanged',async()=>{
  const w=mount(SavedViews,{props:{filters}});const click=async(text:string)=>w.findAll('button').find(b=>b.text()===text)!.trigger('click');await click('Save as new');expect(w.get('[role="alert"]').text()).toContain('Enter a view name');await w.get('input').setValue('Office');await click('Save as new');await click('Save as new');expect(w.get('[role="alert"]').text()).toContain('already exists');await click('Delete view');expect(JSON.parse(localStorage.getItem(savedViewsKey)!).items).toHaveLength(1);await click('Confirm delete view');expect(JSON.parse(localStorage.getItem(savedViewsKey)!).items).toHaveLength(0);expect(w.emitted('apply')).toBeUndefined();w.unmount()
 })
 it('reports corrupt storage and write failure without claiming success',async()=>{
  localStorage.setItem(savedViewsKey,'invalid');const w=mount(SavedViews,{props:{filters}});expect(w.get('[role="alert"]').text()).toContain('could not be loaded');vi.stubGlobal('localStorage',{setItem:()=>{throw Error('quota')}});await w.get('input').setValue('Office');await w.findAll('button').find(b=>b.text()==='Save as new')!.trigger('click');expect(w.get('[role="alert"]').text()).toContain('Unable to save');expect(w.findAll('option')).toHaveLength(1);w.unmount()
 })
})
