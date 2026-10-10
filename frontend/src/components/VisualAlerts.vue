<script setup lang="ts">
import {onMounted,onUnmounted,ref,watch} from 'vue'
type Alert={sequence:number;name:string;event:string;time:string}
type Page={cursor:number;items:Alert[];dropped:boolean}
const api=()=> (window as unknown as {go?:{main:{App:{GetVisualAlerts?:(cursor:number,baseline:boolean)=>Promise<Page>}}}}).go?.main.App
const enabled=ref(localStorage.getItem('hostlight.visual.enabled')!=='false')
const savedPosition=localStorage.getItem('hostlight.visual.position')
const position=ref(['top-right','top-left','bottom-right','bottom-left'].includes(savedPosition||'')?savedPosition!:'bottom-right')
const savedSeconds=Number(localStorage.getItem('hostlight.visual.seconds')??10)
const seconds=ref([0,5,10,30,60].includes(savedSeconds)?savedSeconds:10)
const alerts=ref<(Alert&{expires:number})[]>([]),warning=ref(''),settings=ref(false)
watch(enabled,v=>{localStorage.setItem('hostlight.visual.enabled',String(v));if(!v)alerts.value=[]})
watch(position,v=>localStorage.setItem('hostlight.visual.position',v))
watch(seconds,v=>localStorage.setItem('hostlight.visual.seconds',String(v)))
let cursor=0,baseline=true,pending=false,stopped=false,timer:ReturnType<typeof setInterval>
function dismiss(sequence:number){alerts.value=alerts.value.filter(a=>a.sequence!==sequence)}
function show(a:Alert){alerts.value.push({...a,expires:seconds.value?Date.now()+seconds.value*1000:0});if(alerts.value.length>20){alerts.value.shift();warning.value='Older popups were removed. Review incident history for all transitions.'}}
async function poll(){
 alerts.value=alerts.value.filter(a=>!a.expires||a.expires>Date.now())
 const get=api()?.GetVisualAlerts
 if(!get||pending)return
 pending=true
 try{const page=await get(cursor,baseline);if(stopped)return;cursor=page.cursor;baseline=false;warning.value=page.dropped?'Some visual alerts were missed. Review incident history.':'';if(enabled.value)page.items.forEach(show)}
 catch{warning.value='Visual alerts unavailable. Monitoring continues; check incident history.'}
 finally{pending=false}
}
onMounted(()=>{poll();timer=setInterval(poll,1000)})
onUnmounted(()=>{stopped=true;clearInterval(timer)})
</script>
<template>
 <section class="visual-controls"><button @click="settings=!settings" :aria-expanded="settings">Visual alerts</button><div v-if="settings" class="notice"><p>Popups appear inside Hostlight only. They do not appear over other applications. Snoozing and maintenance also silence these alerts.</p><label><input type="checkbox" v-model="enabled"> Enable outage and recovery popups</label><label>Position <select v-model="position"><option value="top-right">Top right</option><option value="top-left">Top left</option><option value="bottom-right">Bottom right</option><option value="bottom-left">Bottom left</option></select></label><label>Close after <select v-model="seconds"><option :value="0">Until dismissed</option><option v-for="n in [5,10,30,60]" :key="n" :value="n">{{n}} seconds</option></select></label><button @click="show({sequence:-Date.now(),name:'Example monitor',event:'down',time:new Date().toISOString()})">Preview popup</button></div><p v-if="warning" role="status">{{warning}}</p></section>
 <section class="visual-stack" :class="position" aria-label="Availability alerts" aria-live="polite" aria-relevant="additions"><article v-for="a in alerts" :key="a.sequence" class="visual-popup" :class="a.event"><div><strong>{{a.event==='down'?'Monitor unavailable':'Monitor recovered'}}</strong><button @click="dismiss(a.sequence)" :aria-label="`Dismiss alert for ${a.name}`">×</button></div><p>{{a.name}}</p><small>{{new Date(a.time).toLocaleTimeString()}}</small></article><button v-if="alerts.length>1" @click="alerts=[]">Dismiss all</button></section>
</template>
<style scoped>
.visual-controls{margin-bottom:16px}.visual-controls label{display:inline-flex;align-items:center;gap:8px;margin:8px 16px 8px 0}.visual-stack{position:fixed;z-index:100;width:min(360px,calc(100vw - 32px));max-height:75vh;overflow:auto;display:grid;gap:10px;pointer-events:none}.top-right{top:16px;right:16px}.top-left{top:16px;left:16px}.bottom-right{bottom:16px;right:16px}.bottom-left{bottom:16px;left:16px}.visual-popup,.visual-stack>button{pointer-events:auto}.visual-popup{background:var(--panel,#18212d);color:var(--text,#eef4ff);border:1px solid #64748b;border-left:5px solid #fb7185;border-radius:12px;padding:16px;box-shadow:0 12px 35px #0005}.visual-popup.up{border-left-color:#6ee7b7}.visual-popup>div{display:flex;align-items:center;justify-content:space-between;gap:16px}.visual-popup p{overflow-wrap:anywhere;margin:10px 0}.visual-popup button{padding:2px 9px;font-size:20px}
</style>
