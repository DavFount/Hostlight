<script setup lang="ts">
import {computed,ref} from 'vue'
type Silence={id:string;targetId:string;kind:string;starts:string;ends:string;reason:string}
type API={SnoozeTargetAlerts:(id:string,minutes:number,reason:string)=>Promise<void>;ScheduleTargetMaintenance:(id:string,start:string,end:string,reason:string)=>Promise<void>;CancelAlertSilence:(id:string)=>Promise<void>}
const props=defineProps<{target:{id:string;name:string};silences:Silence[]}>()
const emit=defineEmits<{close:[];changed:[]}>()
const api=()=>(window as unknown as {go?:{main:{App:API}}}).go?.main.App
const minutes=ref(60),snoozeReason=ref(''),reason=ref(''),error=ref(''),notice=ref(''),busy=ref(false)
function localInput(date:Date){return new Date(date.getTime()-date.getTimezoneOffset()*60000).toISOString().slice(0,16)}
const start=ref(localInput(new Date())),end=ref(localInput(new Date(Date.now()+3600000)))
const zone=Intl.DateTimeFormat().resolvedOptions().timeZone
const windows=computed(()=>props.silences.filter(s=>s.targetId===props.target.id))
const localTime=(value:string)=>new Date(value).toLocaleString(undefined,{timeZoneName:'short'})
const active=(s:Silence)=>new Date(s.starts).getTime()<=Date.now()&&new Date(s.ends).getTime()>Date.now()
async function perform(action:()=>Promise<void>,message:string){busy.value=true;error.value='';notice.value='';try{if(!api())throw new Error('Open Hostlight through Wails.');await action();notice.value=message;emit('changed')}catch(e){error.value=String(e)}finally{busy.value=false}}
function snooze(){return perform(()=>api()!.SnoozeTargetAlerts(props.target.id,minutes.value,snoozeReason.value),'Alerts snoozed. Checks continue.')}
function schedule(){return perform(()=>{const from=new Date(start.value),to=new Date(end.value);if(!Number.isFinite(from.getTime())||!Number.isFinite(to.getTime()))throw new Error('Choose valid start and end dates.');return api()!.ScheduleTargetMaintenance(props.target.id,from.toISOString(),to.toISOString(),reason.value)},'Maintenance scheduled. Checks and incident history continue.')}
function cancel(s:Silence){return perform(()=>api()!.CancelAlertSilence(s.id),'Window cancelled. Other overlapping windows still apply.')}
</script>
<template>
 <div class="backdrop" @click.self="!busy&&emit('close')" @keydown.esc="!busy&&emit('close')">
  <section class="modal" role="dialog" aria-modal="true" aria-labelledby="silence-title">
   <div class="modal-heading"><div><small>ALERT CONTROLS</small><h2 id="silence-title">{{target.name}}</h2></div><button :disabled="busy" aria-label="Close alert controls" @click="emit('close')">✕</button></div>
   <p>Silence notifications without pausing checks. Incident history continues. Suppressed notifications are not replayed, and there is no catch-up alert when a window ends—even if the monitor is still down. Test webhooks bypass these controls.</p>
   <div v-if="error" class="error" role="alert">{{error}}</div><p v-if="notice" role="status">{{notice}}</p>
   <form @submit.prevent="snooze"><h3>Snooze now</h3><label for="snooze-minutes">Duration (minutes)<input id="snooze-minutes" v-model.number="minutes" type="number" min="1" max="10080" required autofocus></label><small>1 minute to 7 days. Common choices: 15, 60, or 240 minutes.</small><label for="snooze-reason">Reason (optional)<input id="snooze-reason" v-model="snoozeReason" maxlength="200" placeholder="For example: Troubleshooting with the network team"></label><div class="form-actions"><button class="primary" :disabled="busy||!api()">Snooze alerts</button></div></form>
   <form @submit.prevent="schedule"><h3>Schedule maintenance</h3><p class="check-help">One-time window, up to 30 days. Times use {{zone}}; saved as absolute UTC timestamps. Choose a start within the next year.</p><div class="form-row"><label for="maintenance-start">Starts<input id="maintenance-start" v-model="start" type="datetime-local" required></label><label for="maintenance-end">Ends<input id="maintenance-end" v-model="end" type="datetime-local" required></label></div><label for="maintenance-reason">Reason (optional)<input id="maintenance-reason" v-model="reason" maxlength="200" placeholder="For example: Switch firmware upgrade"></label><div class="form-actions"><button :disabled="busy||!api()">Schedule window</button></div></form>
   <section class="scheduled-windows"><h3>Active and upcoming windows</h3><p v-if="!windows.length">No active or upcoming windows for this monitor.</p><article v-for="s in windows" :key="s.id"><strong>{{s.kind==='snooze'?'Snooze':'Maintenance'}} · {{active(s)?'Active':'Upcoming'}}</strong><p>{{localTime(s.starts)}} → {{localTime(s.ends)}}</p><small>{{s.reason||'No reason provided'}}</small><button :disabled="busy" @click="cancel(s)">{{active(s)?'End now':'Cancel window'}}</button></article><small>Overlapping windows are combined. Ending one does not end the others.</small></section>
  </section>
 </div>
</template>
<style scoped>
.modal{width:660px}.modal>p{line-height:1.7}.modal form,.scheduled-windows{border-top:1px solid var(--border);padding-top:22px;margin-top:22px}.modal h3{font-size:16px}.scheduled-windows article{display:grid;gap:8px;padding:14px;border:1px solid var(--border);border-radius:8px;margin:12px 0;font-size:12px}.scheduled-windows article p{margin:0;font-size:12px}.scheduled-windows article button{justify-self:start}.scheduled-windows>p{font-size:12px}.modal-heading h2{overflow-wrap:anywhere}
</style>
