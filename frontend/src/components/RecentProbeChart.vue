<script setup lang="ts">
import {computed,onUnmounted,ref,watch} from 'vue'
type Sample={time:string;failed:boolean;latencyMS:number;breakBefore:boolean}
const props=defineProps<{id:string;sessionStart?:string}>()
const open=ref(false),samples=ref<Sample[]>([]),error=ref(''),loading=ref(false)
let timer:ReturnType<typeof setInterval>|undefined,revision=0,pending=false
async function refresh(){
 if(pending||!open.value)return
 const current=revision
 pending=true;loading.value=true
 try{
  const api=(window as unknown as {go?:{main:{App:{GetRecentProbeSamples:(id:string)=>Promise<Sample[]>}}}}).go?.main.App
  if(!api?.GetRecentProbeSamples)throw new Error('Unavailable')
  const result=await api.GetRecentProbeSamples(props.id)
  if(current!==revision)return
  samples.value=result.slice(-100);error.value=''
 }catch{if(current===revision){samples.value=[];error.value='Recent chart unavailable. Try closing and reopening it.'}}
 finally{if(current===revision){pending=false;loading.value=false}}
}
watch([open,()=>props.id,()=>props.sessionStart],()=>{
 revision++;pending=false;loading.value=false;samples.value=[];error.value=''
 if(timer)clearInterval(timer)
 if(open.value){void refresh();timer=setInterval(()=>void refresh(),5000)}
})
onUnmounted(()=>{revision++;if(timer)clearInterval(timer)})
const passed=computed(()=>samples.value.filter(s=>!s.failed))
const failed=computed(()=>samples.value.length-passed.value.length)
const peak=computed(()=>Math.max(1,...passed.value.map(s=>s.latencyMS)))
const average=computed(()=>passed.value.reduce((sum,s)=>sum+s.latencyMS,0)/passed.value.length)
function x(index:number){return samples.value.length===1?150:10+index*280/(samples.value.length-1)}
function y(sample:Sample){return 85-sample.latencyMS/peak.value*65}
const lines=computed(()=>{
 const segments:string[]=[];let current=''
 for(const [i,s] of samples.value.entries()){
  if(s.failed||s.breakBefore){if(current)segments.push(current);current=''}
  if(!s.failed)current+=(current?' ':'')+`${x(i)},${y(s)}`
 }
 if(current)segments.push(current)
 return segments
})
function localTime(time:string){return new Date(time).toLocaleTimeString()}
</script>
<template>
 <details class="recent-chart" @toggle="open=($event.target as HTMLDetailsElement).open">
  <summary>Recent chart</summary>
  <div v-if="open" class="chart-content">
   <p v-if="error" role="status">{{error}}</p>
   <p v-else-if="!samples.length" role="status">{{loading?'Loading recent checks…':'No completed availability checks yet.'}}</p>
   <template v-else>
    <div class="chart-heading"><span>Successful latency</span><span>{{peak.toFixed(1)}} ms scale</span></div>
    <svg viewBox="0 0 300 110" role="img" :aria-label="`${samples.length} recent checks; ${failed} failed. ${passed.length?`Successful latency average ${average.toFixed(1)} milliseconds, maximum ${Math.max(...passed.map(s=>s.latencyMS)).toFixed(1)} milliseconds.`:'No successful latency observations.'}`">
     <path d="M10 20H290 M10 85H290" class="grid-line" />
     <polyline v-for="(line,i) in lines" :key="i" :points="line" class="latency-line" />
     <g v-for="(sample,i) in samples" :key="i"><title>{{localTime(sample.time)}} · {{sample.failed?'Failed check':sample.latencyMS.toFixed(1)+' ms'}}</title><path v-if="sample.failed" :d="`M${x(i)-2} 96l4 4m-4 0l4-4`" class="failure-mark" /><circle v-else :cx="x(i)" :cy="y(sample)" r="1.8" class="latency-dot" /></g>
    </svg>
    <div class="chart-times"><time :datetime="samples[0]!.time">{{localTime(samples[0]!.time)}}</time><time :datetime="samples[samples.length-1]!.time">{{localTime(samples[samples.length-1]!.time)}}</time></div>
    <p class="chart-summary">{{failed}} / {{samples.length}} failed · {{(100*failed/samples.length).toFixed(1)}}%<span v-if="passed.length">Mean {{average.toFixed(1)}} ms · max {{Math.max(...passed.map(s=>s.latencyMS)).toFixed(1)}} ms</span></p>
    <details class="sample-details"><summary>Check values</summary><div class="sample-table" tabindex="0" aria-label="Recent check values"><table><thead><tr><th>Time</th><th>Result</th><th>ms</th></tr></thead><tbody><tr v-for="(sample,i) in samples" :key="i"><td>{{localTime(sample.time)}}</td><td>{{sample.failed?'Failed':'Passed'}}{{sample.breakBefore?' · gap':''}}</td><td>{{sample.failed?'—':sample.latencyMS.toFixed(1)}}</td></tr></tbody></table></div></details>
    <p class="chart-help">Last {{samples.length}} completed checks · session only. Checks are equally spaced; elapsed time varies. × marks failures; latency uses successes only. Lines break at failures, pauses, setup errors, and monitoring gaps. Resets on edit or restart.</p>
   </template>
  </div>
 </details>
</template>
<style scoped>
.recent-chart{border:0;padding:0;font-size:11px;margin:10px 0;color:var(--muted)}.recent-chart summary{cursor:pointer;font-size:11px}.chart-content{padding-top:10px}.chart-heading,.chart-times{display:flex;justify-content:space-between;gap:8px;font-size:10px}.chart-content svg{display:block;width:100%;height:auto}.grid-line{stroke:var(--border);fill:none;stroke-width:1}.latency-line{stroke:var(--accent,#43b7d1);fill:none;stroke-width:1.5}.latency-dot{fill:var(--accent,#43b7d1)}.failure-mark{stroke:var(--danger,#e45d73);stroke-width:1.5}.chart-summary{font-size:11px;color:var(--text);line-height:1.6}.chart-summary span{display:block;color:var(--muted);font-size:10px}.chart-help{font-size:10px;line-height:1.5}.sample-details{font-size:10px}.sample-table{max-height:180px;overflow:auto;margin-top:8px}table{width:100%;border-collapse:collapse;font-size:10px}th,td{text-align:left;padding:4px;border-bottom:1px solid var(--border)}:global([data-theme="light"]) .latency-line{stroke:#126780}:global([data-theme="light"]) .latency-dot{fill:#126780}:global([data-theme="light"]) .failure-mark{stroke:#b12242}
</style>
