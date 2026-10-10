<script setup lang="ts">
import {computed,ref} from 'vue'
import {readSavedViews,sameFilters,savedViewsKey,type MonitorFilters,type SavedView} from '../savedViews'
const props=defineProps<{filters:MonitorFilters}>()
const emit=defineEmits<{apply:[filters:MonitorFilters]}>()
const items=ref<SavedView[]>([]),error=ref(''),notice=ref(''),selected=ref(''),name=ref(''),confirmDelete=ref(false)
try{items.value=readSavedViews(localStorage.getItem(savedViewsKey))}catch{error.value='Saved views could not be loaded. Monitor settings are unaffected. Saving a new view will replace the unreadable saved-view list.'}
const current=computed(()=>items.value.find(v=>v.id===selected.value))
const modified=computed(()=>current.value&&!sameFilters(current.value,props.filters))
function persist(next:SavedView[]){try{localStorage.setItem(savedViewsKey,JSON.stringify({version:1,items:next}));items.value=next;error.value='';return true}catch{error.value='Unable to save views on this computer. Your existing views have not been changed.';return false}}
function apply(){confirmDelete.value=false;const v=current.value;if(v){name.value=v.name;emit('apply',{search:v.search,status:v.status,group:v.group});notice.value=`Applied ${v.name}.`}}
function save(update=false){
 confirmDelete.value=false
 const title=name.value.trim()
 if(!title||title.length>60){error.value='Enter a view name between 1 and 60 characters.';return}
 if(props.filters.search.length>500||(props.filters.group?.length??0)>200){error.value='Search must be at most 500 characters and group at most 200 characters to save a view.';return}
 if(items.value.some(v=>v.name.toLowerCase()===title.toLowerCase()&&(!update||v.id!==selected.value))){error.value='A view with that name already exists. Choose another name or update the selected view.';return}
 if(!update&&items.value.length>=20){error.value='You can save up to 20 views. Delete an unused view first.';return}
 if(update&&!current.value)return
 const id=update?selected.value:crypto.randomUUID()
 const v={...props.filters,id,name:title}
 const next=update?items.value.map(old=>old.id===id?v:old):[...items.value,v]
 if(persist(next)){selected.value=id;name.value=title;notice.value=`Saved ${title}.`}
}
function remove(){const v=current.value;if(!v)return;if(persist(items.value.filter(i=>i.id!==v.id))){selected.value='';name.value='';confirmDelete.value=false;notice.value=`Deleted view ${v.name}. Monitors and current filters are unchanged.`}}
function reset(){selected.value='';name.value='';confirmDelete.value=false;emit('apply',{search:'',status:'all',group:null});notice.value='Showing all monitors.'}
</script>
<template>
 <section class="saved-views" aria-label="Saved monitoring views">
  <div class="saved-row"><label>Saved view <select v-model="selected" @change="apply"><option value="">Custom filters</option><option v-for="v in items" :key="v.id" :value="v.id">{{v.name}}</option></select></label><span v-if="modified" class="changed">Filters changed · not saved</span><button v-if="current" @click="apply">Reapply</button><button @click="reset">Clear filters</button></div>
  <details><summary>Save or manage views</summary><p>Save the current search, status, and group filters on this computer. Views never change monitoring or alert rules.</p><div class="saved-row"><label>View name <input v-model="name" maxlength="60" placeholder="e.g. Production outages" @keydown.enter.prevent="save()"></label><button @click="save()">Save as new</button><button v-if="current" @click="save(true)">Update selected</button><button v-if="current" @click="confirmDelete=!confirmDelete">Delete view</button></div><div v-if="confirmDelete" class="saved-row"><span>Delete “{{current?.name}}”? Monitors will not be removed.</span><button class="danger" @click="remove">Confirm delete view</button><button @click="confirmDelete=false">Cancel</button></div></details>
  <p v-if="error" class="error" role="alert">{{error}}</p><p v-else-if="notice" role="status">{{notice}}</p>
 </section>
</template>
<style scoped>
.saved-views{border:1px solid var(--border);border-radius:10px;padding:14px;margin:0 0 20px;background:var(--panel);font-size:12px}.saved-row{display:flex;align-items:center;flex-wrap:wrap;gap:10px}.saved-row label{display:flex;align-items:center;gap:10px}.saved-row input,.saved-row select{max-width:260px}.saved-views details{margin-top:12px;padding-top:12px}.saved-views p{margin:10px 0 0}.changed{color:var(--muted)}
</style>
