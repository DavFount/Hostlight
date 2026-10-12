<script setup lang="ts">
import {computed,onUnmounted,ref,watch} from 'vue'
type Monitor={id:string;name:string}
type Bucket={time:string;checks:number;failed:number;meanMS:number;minMS:number;maxMS:number;types:string}
type Page={items:Bucket[];bucketMinutes:number;error:string;from:string;until:string}
type API={GetProbeHistoryMonitors:()=>Promise<Monitor[]>;GetProbeHistory:(id:string,hours:number)=>Promise<Page>}
const open=ref(false),monitors=ref<Monitor[]>([]),selected=ref(''),hours=ref(24),page=ref<Page>(),error=ref(''),loading=ref(false)
let revision=0,timer:ReturnType<typeof setInterval>|undefined
const api=()=>(window as unknown as {go?:{main:{App:API}}}).go?.main.App
async function refresh(clear=false){
 if(!open.value)return
 const request=++revision;loading.value=true;if(clear)page.value=undefined
 try{
  if(!api()?.GetProbeHistoryMonitors)throw new Error('Probe history unavailable')
  const list=await api()!.GetProbeHistoryMonitors()
  if(request!==revision)return
  monitors.value=list
  if(!list.some(m=>m.id===selected.value))selected.value=list[0]?.id??''
  const result=selected.value?await api()!.GetProbeHistory(selected.value,hours.value):undefined
  if(request!==revision)return
  page.value=result;error.value=''
 }catch{if(request===revision){page.value=undefined;error.value='Probe history unavailable. Try Refresh; check storage health if this continues.'}}
 finally{if(request===revision)loading.value=false}
}
watch(open,value=>{revision++;if(timer)clearInterval(timer);if(value){void refresh(true);timer=setInterval(()=>void refresh(),30000)}})
onUnmounted(()=>{revision++;if(timer)clearInterval(timer)})
const checks=computed(()=>page.value?.items.reduce((sum,b)=>sum+b.checks,0)??0)
const failed=computed(()=>page.value?.items.reduce((sum,b)=>sum+b.failed,0)??0)
const peak=computed(()=>Math.max(1,...(page.value?.items.filter(b=>b.checks>b.failed).map(b=>b.maxMS)??[])))
function x(bucket:Bucket){const p=page.value!;return 45+(new Date(bucket.time).getTime()-new Date(p.from).getTime())/(new Date(p.until).getTime()-new Date(p.from).getTime())*510}
function y(ms:number){return 95-ms/peak.value*75}
const localTime=(value:string)=>new Date(value).toLocaleString()
const percent=(b:Bucket)=>100*b.failed/b.checks
function failurePercent(failures:number,total:number){const value=100*failures/total;return value>0&&value<0.01?'<0.01':value.toFixed(2)}
</script>
<template>
 <details class="probe-history" @toggle="open=($event.target as HTMLDetailsElement).open">
  <summary>Probe history · saved duration and failed checks</summary>
  <div v-if="open" class="probe-history-content">
   <div class="probe-history-tools"><label>Monitor<select v-model="selected" :disabled="!monitors.length" @change="refresh(true)"><option v-if="!monitors.length" value="">No saved observations</option><option v-for="m in monitors" :key="m.id" :value="m.id">{{m.name}} · #{{m.id}}</option></select></label><label>Range<select v-model.number="hours" @change="refresh(true)"><option :value="1">Last hour</option><option :value="24">Last 24 hours</option><option :value="168">Last 7 days</option></select></label><button :disabled="loading" @click="refresh()">Refresh</button><span v-if="loading" role="status">Updating…</span></div>
   <p v-if="error||page?.error" class="error" role="alert">{{error||page?.error}}</p>
   <template v-if="page?.items.length">
    <p class="probe-history-summary">{{checks.toLocaleString()}} completed checks · {{failed.toLocaleString()}} failed · {{failurePercent(failed,checks)}}% failed</p>
    <svg viewBox="0 0 600 190" role="img" :aria-label="`Saved probe history: ${checks} completed checks, ${failed} failed. Successful duration range and mean above; failed-check percentage below. Table contains exact values.`">
     <path d="M45 20H560 M45 95H560 M45 120H560 M45 165H560" class="plot-grid" />
     <text x="4" y="24">{{peak.toFixed(1)}} ms</text><text x="4" y="98">0 ms</text><text x="4" y="124">100%</text><text x="4" y="168">0%</text>
     <g v-for="(b,i) in page.items" :key="i"><title>{{localTime(b.time)}} · {{b.checks}} checks, {{b.failed}} failed · {{b.types}}</title><template v-if="b.checks>b.failed"><path :d="`M${x(b)} ${y(b.minMS)}V${y(b.maxMS)}`" class="duration-range" /><circle :cx="x(b)" :cy="y(b.meanMS)" r="2" class="duration-mean" /></template><path v-if="b.failed" :d="`M${x(b)} 165V${165-percent(b)*0.45}`" class="failed-bar" /></g>
    </svg>
    <div class="probe-history-times"><time :datetime="page.from">{{localTime(page.from)}}</time><time :datetime="page.until">{{localTime(page.until)}}</time></div>
    <p class="probe-history-legend">Upper plot: successful duration min–max with mean dots. Lower plot: failed-check %. Empty intervals have no observations. No line joins separate intervals.</p>
    <details class="probe-history-values"><summary>Interval values · {{page.bucketMinutes}} minute summaries</summary><div class="probe-history-table" tabindex="0" aria-label="Saved probe interval values"><table><thead><tr><th>Interval starts</th><th>Checks</th><th>Failed</th><th>Failure %</th><th>Mean ms</th><th>Min ms</th><th>Max ms</th><th>Check types</th></tr></thead><tbody><tr v-for="b in page.items" :key="b.time"><td>{{localTime(b.time)}}</td><td>{{b.checks}}</td><td>{{b.failed}}</td><td>{{failurePercent(b.failed,b.checks)}}</td><td>{{b.checks>b.failed?b.meanMS.toFixed(1):'—'}}</td><td>{{b.checks>b.failed?b.minMS.toFixed(1):'—'}}</td><td>{{b.checks>b.failed?b.maxMS.toFixed(1):'—'}}</td><td>{{b.types}}</td></tr></tbody></table></div></details>
   </template>
   <p v-else-if="!error&&!loading" role="status">No saved checks in this range. New completed checks appear after the background writer saves them.</p>
   <p class="probe-history-help">Saved across restarts; includes previous edits and sessions for this monitor ID. Counts exclude DNS/setup errors, cancelled probes, pauses, and gaps. Successful duration measures echo reply, TCP connection, or HTTP header/body timing; changed check types can share an interval. Not uptime or packet loss. Minute summaries are retained for seven days, capped globally at 50,000 (oldest saved summaries removed first). Range edges have minute precision. Refreshes every 30 seconds while open.</p>
  </div>
 </details>
</template>
<style scoped>
.probe-history{border:1px solid var(--border);border-radius:10px;padding:16px;margin:0 0 22px;background:var(--panel);color:var(--muted)}.probe-history>summary{font-size:13px;color:var(--text)}.probe-history-content{padding-top:16px}.probe-history-tools{display:flex;gap:12px;align-items:end;flex-wrap:wrap}.probe-history-tools label{display:grid;gap:6px;font-size:11px;min-width:0;max-width:100%}.probe-history-tools select{max-width:100%;font-size:12px}.probe-history-tools button{font-size:12px}.probe-history-summary{color:var(--text);font-size:12px;margin:16px 0}.probe-history svg{display:block;width:100%;height:auto;max-height:300px}.plot-grid{stroke:var(--border);fill:none}.duration-range{stroke:var(--accent);stroke-width:1.5}.duration-mean{fill:var(--accent)}.failed-bar{stroke:#ef7989;stroke-width:3}svg text{fill:var(--muted);font-size:8px}.probe-history-times{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap;font-size:10px}.probe-history-legend,.probe-history-help{font-size:11px;line-height:1.6}.probe-history-values{margin:12px 0;padding:10px 0}.probe-history-table{overflow:auto;max-height:320px;margin-top:10px}table{border-collapse:collapse;width:100%;font-size:11px}th,td{text-align:left;padding:7px;border-bottom:1px solid var(--border);white-space:nowrap}:global([data-theme="light"]) .duration-range{stroke:#126780}:global([data-theme="light"]) .duration-mean{fill:#126780}:global([data-theme="light"]) .failed-bar{stroke:#b12242}
</style>
