<script setup lang="ts">
import {onMounted,onUnmounted,ref,watch} from 'vue'
type Incident={id:number;name:string;host:string;group:string;type:string;port:number;opened:string;ended:string;lastSeen:string;status:string;reason:string;durationSeconds:number}
type HistoryPage={items:Incident[];total:number;error:string;path:string;retentionDays:number}
type API={GetIncidents:(search:string,status:string,days:number,offset:number)=>Promise<HistoryPage>;ExportIncidents:(search:string,status:string,days:number)=>Promise<string>}
const api=()=>(window as unknown as {go?:{main:{App:API}}}).go?.main.App
const search=ref(''),status=ref('all'),days=ref(30),offset=ref(0)
const page=ref<HistoryPage>({items:[],total:0,error:'',path:'',retentionDays:90})
const error=ref(''),exported=ref(''),loading=ref(false),saving=ref(false)
let poll:ReturnType<typeof setInterval>,debounce:ReturnType<typeof setTimeout>,generation=0
const localTime=(value:string)=>value?new Date(value).toLocaleString():'—'
function duration(seconds:number){const d=Math.floor(seconds/86400),h=Math.floor(seconds%86400/3600),m=Math.floor(seconds%3600/60),s=seconds%60;return d?`${d}d ${h}h`:h?`${h}h ${m}m`:m?`${m}m ${s}s`:`${s}s`}
function statusLabel(value:string){return value==='open'?'Unavailable':value==='resolved'?'Recovered':'Interrupted'}
async function refresh(){
 if(!api())return
 const request=++generation;loading.value=true
 try{const result=await api()!.GetIncidents(search.value,status.value,days.value,offset.value);if(request!==generation)return;page.value=result;error.value=''}catch(e){if(request===generation)error.value=String(e)}finally{if(request===generation)loading.value=false}
}
watch([search,status,days],()=>{offset.value=0;generation++;clearTimeout(debounce);debounce=setTimeout(refresh,200)})
watch(offset,refresh)
async function exportCSV(){saving.value=true;exported.value='';try{exported.value=await api()!.ExportIncidents(search.value,status.value,days.value)}catch(e){error.value=String(e)}finally{saving.value=false}}
onMounted(()=>{refresh();poll=setInterval(refresh,5000)})
onUnmounted(()=>{generation++;clearInterval(poll);clearTimeout(debounce)})
</script>

<template>
 <section class="incident-history">
  <div class="history-toolbar">
   <input v-model="search" aria-label="Search incident history" placeholder="Search monitor, host, or group…">
   <select v-model="status" aria-label="Incident status"><option value="all">All outcomes</option><option value="open">Unavailable</option><option value="resolved">Recovered</option><option value="interrupted">Interrupted</option></select>
   <select v-model.number="days" aria-label="History date range"><option :value="1">Last 24 hours</option><option :value="7">Last 7 days</option><option :value="30">Last 30 days</option><option :value="90">Last 90 days</option><option :value="0">All retained history</option></select>
   <button :disabled="saving||!api()||page.total===0" @click="exportCSV">Export CSV</button>
  </div>
  <div v-if="error||page.error" class="error" role="alert">{{error||page.error}}</div>
  <p v-if="exported" class="notice" role="status">Exported to {{exported}}</p>
  <p class="history-caption">{{page.total}} incidents matching your filters <span v-if="loading">· Updating…</span></p>
  <div v-if="page.items.length" class="history-table-wrap">
   <table class="history-table"><thead><tr><th scope="col">Monitor</th><th scope="col">Outcome</th><th scope="col">Started</th><th scope="col">Ended</th><th scope="col">Observed duration</th></tr></thead>
    <tbody><tr v-for="item in page.items" :key="item.id"><td><strong>{{item.name}}</strong><small>{{item.host}} · {{item.type==='icmp'?'Ping':item.type==='http'?'HTTP GET':`TCP ${item.port}`}}</small><small v-if="item.group">{{item.group}}</small></td><td><span class="incident-status" :class="item.status">{{statusLabel(item.status)}}</span><small class="incident-reason">{{item.reason}}</small></td><td>{{localTime(item.opened)}}</td><td>{{item.ended?localTime(item.ended):'Still being observed'}}</td><td><strong>{{duration(item.durationSeconds)}}</strong><small>Last observation: {{localTime(item.lastSeen)}}</small></td></tr></tbody>
   </table>
  </div>
  <div v-else class="empty"><div>◷</div><h2>{{loading?'Loading history…':'No incidents to show'}}</h2><p>Outages appear after a monitor reaches its failure threshold.<br>Try another filter if you are looking for an older incident.</p></div>
  <div class="history-pagination"><button :disabled="offset===0||loading" @click="offset=Math.max(0,offset-100)">Previous</button><span v-if="page.total">{{offset+1}}–{{Math.min(offset+100,page.total)}} of {{page.total}}</span><button :disabled="offset+100>=page.total||loading" @click="offset+=100">Next</button></div>
  <div class="history-settings"><p>Interrupted means monitoring stopped or observations were missing. It does not confirm recovery. Duration ends at the last observation, so time while Hostlight is closed is excluded.</p><p class="bottom-note">Completed history is also capped at approximately 50,000 records. Active incidents are kept.</p><p v-if="page.path" class="bottom-note">History: {{page.path}}</p></div>
 </section>
</template>
