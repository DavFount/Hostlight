import {describe,it,expect} from 'vitest'
import {readSavedViews,sameFilters} from './savedViews'
const item={id:'1',name:'Office',search:'router',status:'down',group:'Office'}
const encode=(items:unknown[],version=1)=>JSON.stringify({version,items})
describe('saved view validation',()=>{
 it('handles a new workspace and preserves all-groups versus ungrouped',()=>{expect(readSavedViews(null)).toEqual([]);expect(readSavedViews(encode([{...item,group:null}]))[0]?.group).toBe(null);expect(readSavedViews(encode([{...item,group:''}]))[0]?.group).toBe('');expect(sameFilters({...item,group:null},{...item,group:''})).toBe(false)})
 it('rejects corrupt, future, duplicate, oversized, and invalid data',()=>{for(const raw of ['{','null',encode([item],2),encode([item,{...item,id:'2',name:' OFFICE '}]),encode([item,{...item,name:'Other'}]),encode([{...item,status:'broken'}]),encode([{...item,search:'x'.repeat(501)}]),encode(Array.from({length:21},(_,i)=>({...item,id:String(i),name:String(i)})))])expect(()=>readSavedViews(raw)).toThrow()})
 it('keeps only filter metadata',()=>{expect(readSavedViews(encode([{...item,password:'secret'}]))).toEqual([item])})
})
