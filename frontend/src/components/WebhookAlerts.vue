<script setup lang="ts">
import {computed,onMounted,onUnmounted,ref} from 'vue'
type Settings={id:string;name:string;scope:'global'|'selected';groups:string[];monitorIds:string[];enabled:boolean;configured:boolean;url:string;bearer:string;signingKey:string;allowHTTP:boolean;clearBearer:boolean;clearSigningKey:boolean;hasBearer:boolean;hasSigningKey:boolean;error?:string}
type Target={id:string;name:string;host:string;group:string}
type Delivery={id:string;name:string;host:string;destinationName:string;event:string;created:string;attempts:number;status:string;result:string}
type API={GetWebhookDestinations:()=>Promise<Settings[]>;SaveWebhookDestination:(s:Settings)=>Promise<void>;DeleteWebhookDestination:(id:string)=>Promise<void>;TestWebhookDestination:(id:string)=>Promise<void>;GetWebhookDeliveries:()=>Promise<Delivery[]>;GetTargets:()=>Promise<Target[]>}
const api=()=>(window as unknown as {go?:{main:{App:API}}}).go?.main.App
const blank=():Settings=>({id:'',name:'',scope:'global',groups:[],monitorIds:[],enabled:true,configured:false,url:'',bearer:'',signingKey:'',allowHTTP:false,clearBearer:false,clearSigningKey:false,hasBearer:false,hasSigningKey:false})
const draft=ref(blank()),destinations=ref<Settings[]>([]),targets=ref<Target[]>([]),editing=ref(false),monitorSearch=ref('')
const items=ref<Delivery[]>([]),error=ref(''),notice=ref(''),busy=ref(false),loaded=ref(false)
const groups=computed(()=>[...new Set([...targets.value.map(t=>t.group),...draft.value.groups])].sort())
const monitorOptions=computed(()=>{const present=new Set(targets.value.map(t=>t.id));return [...targets.value,...draft.value.monitorIds.filter(id=>!present.has(id)).map(id=>({id,name:`Removed monitor (ID ${id})`,host:'No longer monitored',group:''}))].filter(t=>`${t.name} ${t.host} ${t.group}`.toLowerCase().includes(monitorSearch.value.toLowerCase())).sort((a,b)=>a.name.localeCompare(b.name))})
const scopeLabel=(d:Settings)=>d.scope==='global'?'All monitors':[`${d.groups.length} group(s)`,`${d.monitorIds.length} monitor(s)`].join(' · ')
let poll:ReturnType<typeof setInterval>,active=true
async function refresh(){if(!api())return;try{const [result,currentTargets]=await Promise.all([api()!.GetWebhookDeliveries(),api()!.GetTargets()]);if(active){items.value=result;targets.value=currentTargets}}catch(e){if(active)error.value=String(e)}}
async function load(){if(!api())return;const result=await api()!.GetWebhookDestinations();if(active){destinations.value=result;loaded.value=true}}
function edit(d?:Settings){draft.value=d?{...d,groups:[...d.groups],monitorIds:[...d.monitorIds]}:blank();editing.value=true;monitorSearch.value='';error.value='';notice.value=''}
function cancel(){draft.value=blank();editing.value=false}
async function save(){busy.value=true;error.value='';notice.value='';try{await api()!.SaveWebhookDestination({...draft.value});cancel();await load();notice.value='Settings saved. Unsent deliveries for this destination were cancelled.'}catch(e){error.value=String(e)}finally{busy.value=false;refresh()}}
async function test(d:Settings){busy.value=true;error.value='';notice.value='';try{await api()!.TestWebhookDestination(d.id);notice.value=`Test queued for ${d.name} using saved settings. Check delivery history below.`}catch(e){error.value=String(e)}finally{busy.value=false;refresh()}}
async function remove(d:Settings){if(!confirm(`Remove webhook destination "${d.name}"? Its pending deliveries will be cancelled. Delivery history is kept.`))return;busy.value=true;error.value='';try{await api()!.DeleteWebhookDestination(d.id);if(draft.value.id===d.id)cancel();await load();notice.value='Destination removed; pending deliveries cancelled.'}catch(e){error.value=String(e)}finally{busy.value=false;refresh()}}
onMounted(async()=>{try{await load()}catch(e){if(active)error.value=String(e)};if(!active)return;refresh();poll=setInterval(refresh,2000)})
onUnmounted(()=>{active=false;clearInterval(poll)})
</script>

<template>
 <section class="webhook-alerts">
  <div v-if="error" class="error" role="alert">{{error}}</div>
  <p v-if="notice" role="status">{{notice}}</p>
  <div class="section-heading"><h2>Webhook destinations</h2><button class="primary" :disabled="busy||!loaded||destinations.length>=50" @click="edit()">Add destination</button></div>
  <p>Global and scoped destinations are additive. Each matching destination receives one event, even when both its group and individual monitor selections match. Separate destinations are independent, even if they use the same URL.</p>
  <div class="destination-list">
   <article v-for="d in destinations" :key="d.id" class="destination-card">
    <div><h3>{{d.name}}</h3><p>{{d.enabled?'Enabled':'Disabled'}} · {{scopeLabel(d)}}</p><small v-if="d.scope==='selected'">Groups: {{d.groups.map(g=>g||'Ungrouped').join(', ')||'None'}}</small><p v-if="d.error" class="error">{{d.error}}</p></div>
    <div class="header-actions"><button :disabled="busy" @click="edit(d)">Edit</button><button :disabled="busy||!d.enabled||!!d.error" @click="test(d)">Send test</button><button class="danger" :disabled="busy" @click="remove(d)">Remove</button></div>
   </article>
   <p v-if="loaded&&!destinations.length">No destinations yet. Add one for all monitors, a team’s groups, or specific critical hosts.</p>
  </div>
  <form v-if="editing" class="webhook-settings" @submit.prevent="save">
   <h2>{{draft.id?'Edit destination':'New destination'}}</h2>
   <p>Send a JSON POST when a previously available monitor goes down, and when that outage recovers. Existing failure and recovery thresholds apply. Initial discovery, pauses, check errors, and restarts do not send alerts.</p>
   <label for="webhook-name">Destination name</label><input id="webhook-name" v-model="draft.name" required maxlength="120" placeholder="For example: Infrastructure on-call">
   <label class="check-label"><input v-model="draft.enabled" type="checkbox"> Enable this destination</label>
   <label for="webhook-scope">Send alerts for</label><select id="webhook-scope" v-model="draft.scope"><option value="global">All monitors (global)</option><option value="selected">Selected groups and/or individual monitors</option></select>
   <div v-if="draft.scope==='selected'" class="routing-picker">
    <fieldset><legend>Groups</legend><small>Includes current and future monitors in each selected group.</small><div class="selection-list"><label v-for="g in groups" :key="g" class="check-label"><input v-model="draft.groups" type="checkbox" :value="g">{{g||'Ungrouped'}}</label><p v-if="!groups.length">Create a monitor group first, or select individual monitors below.</p></div></fieldset>
    <fieldset><legend>Individual monitors</legend><input v-model="monitorSearch" aria-label="Search monitors for webhook" placeholder="Search by name, host, or group"><small>{{draft.monitorIds.length}} selected. Selections remain when you search.</small><div class="selection-list"><label v-for="t in monitorOptions" :key="t.id" class="check-label"><input v-model="draft.monitorIds" type="checkbox" :value="t.id"><span>{{t.name}}<small>{{t.host}}</small></span></label><p v-if="!monitorOptions.length">No matching monitors.</p></div></fieldset>
    <small>Choose at least one group or monitor. These selections are combined, not overrides. Renaming a group does not rename its routing rule.</small>
   </div>
   <label for="webhook-url">Endpoint URL</label>
   <input id="webhook-url" v-model="draft.url" type="password" autocomplete="off" :placeholder="draft.configured?'Endpoint saved securely — leave blank to keep':'https://your-receiver.example/webhook'" :required="!draft.configured">
   <small>Saved URLs and tokens are never displayed again. Protected using your Windows account; not portable to another user or computer.</small>
   <details><summary>Authentication and local HTTP endpoints</summary>
    <label for="webhook-bearer">Bearer token (optional)</label>
    <input id="webhook-bearer" v-model="draft.bearer" type="password" autocomplete="off" :placeholder="draft.hasBearer?'Token saved — leave blank to keep':'Optional token'">
    <label v-if="draft.hasBearer" class="check-label"><input v-model="draft.clearBearer" type="checkbox"> Remove saved bearer token</label>
    <label for="webhook-signing">HMAC signing key (optional)</label>
    <input id="webhook-signing" v-model="draft.signingKey" type="password" autocomplete="off" :placeholder="draft.hasSigningKey?'Key saved — leave blank to keep':'Optional shared secret'">
    <label v-if="draft.hasSigningKey" class="check-label"><input v-model="draft.clearSigningKey" type="checkbox"> Remove saved signing key</label>
    <small>Signs the exact JSON body with HMAC-SHA256 in X-Hostlight-Signature. Receivers should deduplicate X-Hostlight-Event-ID.</small>
    <label class="check-label"><input v-model="draft.allowHTTP" type="checkbox"> Allow unencrypted HTTP (trusted internal endpoints only)</label>
    <small>HTTP exposes the payload and credentials on the network. HTTPS certificate verification is always enabled.</small>
   </details>
   <div class="header-actions"><button class="primary" :disabled="busy||!loaded||!api()">Save settings</button><button type="button" :disabled="busy" @click="cancel">Cancel</button></div>
   <small>Saving cancels queued deliveries for this destination only. A request already in flight may still complete. Send tests from the saved destination card.</small>
  </form>
  <h2>Recent deliveries</h2>
  <p>Latest 100 events. Automatic retries: network errors, HTTP 408, 429, and 5xx; up to 8 attempts within 24 hours. Alerts send only while Hostlight is open.</p>
  <div class="history-table-wrap"><table class="history-table"><thead><tr><th>Created</th><th>Destination</th><th>Monitor</th><th>Event</th><th>Status</th><th>Attempts</th><th>Result</th></tr></thead><tbody><tr v-for="item in items" :key="item.id"><td>{{new Date(item.created).toLocaleString()}}</td><td>{{item.destinationName}}</td><td>{{item.name}}<small>{{item.host}}</small></td><td>{{item.event==='down'?'Outage':item.event==='up'?'Recovery':'Test'}}<small class="event-id">{{item.id}}</small></td><td>{{item.status}}</td><td>{{item.attempts}}</td><td>{{item.result}}</td></tr><tr v-if="!items.length"><td colspan="7">No webhook deliveries yet. Add a destination and send a test to check connectivity.</td></tr></tbody></table></div>
 </section>
</template>

<style scoped>
.webhook-settings{max-width:800px;display:grid;gap:12px;margin-bottom:32px;padding:24px;border:1px solid var(--border);border-radius:14px}
.webhook-settings p,.webhook-alerts>p{line-height:1.6;opacity:.8}
.webhook-settings details{display:grid;padding:12px 0}
.webhook-settings details label,.webhook-settings details small{display:block;margin:14px 0 8px}
.webhook-settings details input:not([type=checkbox]){width:100%;box-sizing:border-box}
.check-label{display:flex;align-items:center;gap:10px}.check-label input{width:auto}
.event-id{display:block;font-size:10px;opacity:.65;margin-top:4px}
.destination-list{display:grid;gap:12px;margin-bottom:24px}.destination-card{padding:20px;border:1px solid var(--border);border-radius:12px;background:var(--panel);display:flex;align-items:center;justify-content:space-between;gap:16px}.destination-card h3{font-size:16px;overflow-wrap:anywhere}.destination-card p{margin:8px 0;font-size:12px}.routing-picker{display:grid;gap:16px}.routing-picker fieldset{border:1px solid var(--border);border-radius:8px;padding:14px;min-width:0}.routing-picker legend{font-size:13px}.routing-picker fieldset>input{width:100%;margin-bottom:10px}.selection-list{max-height:220px;overflow:auto;display:grid;gap:12px;padding:12px 0}.selection-list small{display:block}.selection-list label{font-size:13px}.header-actions{flex-wrap:wrap}@media(max-width:900px){.destination-card{align-items:flex-start;flex-direction:column}}
</style>
