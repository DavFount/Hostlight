export type MonitorFilters={search:string;status:string;group:string|null}
export type SavedView=MonitorFilters&{id:string;name:string}
export const savedViewsKey='hostlight.savedViews.v1'
export const statuses=['all','up','down','unknown','error','paused']
export function readSavedViews(raw:string|null):SavedView[]{
 if(raw===null)return []
 const data=JSON.parse(raw)
 if(data.version!==1||!Array.isArray(data.items)||data.items.length>20)throw new Error('Unsupported saved views format')
 const ids=new Set<string>(),names=new Set<string>()
 for(const item of data.items){
  if(!item||typeof item.id!=='string'||!item.id||item.id.length>100||typeof item.name!=='string'||!item.name.trim()||item.name.length>60||typeof item.search!=='string'||item.search.length>500||!statuses.includes(item.status)||(item.group!==null&&(typeof item.group!=='string'||item.group.length>200)))throw new Error('Invalid saved view')
  const name=item.name.trim().toLowerCase()
  if(ids.has(item.id)||names.has(name))throw new Error('Duplicate saved view')
  ids.add(item.id);names.add(name)
 }
 return data.items.map((v:SavedView)=>({id:v.id,name:v.name.trim(),search:v.search,status:v.status,group:v.group}))
}
export function sameFilters(a:MonitorFilters,b:MonitorFilters){return a.search===b.search&&a.status===b.status&&a.group===b.group}
