<script setup lang="ts">
import {onMounted,onUnmounted,ref} from 'vue'
type Settings={enabled:boolean;configured:boolean;url:string;bearer:string;signingKey:string;allowHTTP:boolean;clearBearer:boolean;clearSigningKey:boolean;hasBearer:boolean;hasSigningKey:boolean}
type Delivery={id:string;name:string;host:string;event:string;created:string;attempts:number;status:string;result:string}
type API={GetWebhookSettings:()=>Promise<Settings>;SaveWebhookSettings:(s:Settings)=>Promise<void>;TestWebhook:()=>Promise<void>;GetWebhookDeliveries:()=>Promise<Delivery[]>}
const api=()=>(window as unknown as {go?:{main:{App:API}}}).go?.main.App
const draft=ref<Settings>({enabled:false,configured:false,url:'',bearer:'',signingKey:'',allowHTTP:false,clearBearer:false,clearSigningKey:false,hasBearer:false,hasSigningKey:false})
const items=ref<Delivery[]>([]),error=ref(''),notice=ref(''),busy=ref(false),loaded=ref(false)
let poll:ReturnType<typeof setInterval>,active=true
async function refresh(){if(!api())return;try{const result=await api()!.GetWebhookDeliveries();if(active)items.value=result}catch(e){if(active)error.value=String(e)}}
async function load(){if(!api())return;try{draft.value={...await api()!.GetWebhookSettings()};loaded.value=true}catch(e){error.value=String(e);loaded.value=true}}
async function save(){busy.value=true;error.value='';notice.value='';try{await api()!.SaveWebhookSettings({...draft.value});await load();notice.value='Settings saved. Unsent deliveries were cancelled.'}catch(e){error.value=String(e)}finally{busy.value=false;refresh()}}
async function test(){busy.value=true;error.value='';notice.value='';try{await api()!.TestWebhook();notice.value='Test queued using the saved settings. Check delivery history below.'}catch(e){error.value=String(e)}finally{busy.value=false;refresh()}}
onMounted(()=>{load();refresh();poll=setInterval(refresh,2000)})
onUnmounted(()=>{active=false;clearInterval(poll)})
</script>

<template>
 <section class="webhook-alerts">
  <div v-if="error" class="error" role="alert">{{error}}</div>
  <p v-if="notice" role="status">{{notice}}</p>
  <form class="webhook-settings" @submit.prevent="save">
   <h2>Webhook destination</h2>
   <p>Send a JSON POST when a previously available monitor goes down, and when that outage recovers. Existing failure and recovery thresholds apply. Initial discovery, pauses, check errors, and restarts do not send alerts.</p>
   <label class="check-label"><input v-model="draft.enabled" type="checkbox"> Enable webhook alerts for all monitors</label>
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
   <div class="header-actions"><button class="primary" :disabled="busy||!loaded||!api()">Save settings</button><button type="button" :disabled="busy||!draft.configured||!api()" @click="test">Send test using saved settings</button></div>
   <small>Saving cancels queued deliveries. A request already in flight may still complete. Tests send real requests to the saved endpoint.</small>
  </form>
  <h2>Recent deliveries</h2>
  <p>Latest 100 events. Automatic retries: network errors, HTTP 408, 429, and 5xx; up to 8 attempts within 24 hours. Alerts send only while Hostlight is open.</p>
  <div class="history-table-wrap"><table class="history-table"><thead><tr><th>Created</th><th>Monitor</th><th>Event</th><th>Status</th><th>Attempts</th><th>Result</th></tr></thead><tbody><tr v-for="item in items" :key="item.id"><td>{{new Date(item.created).toLocaleString()}}</td><td>{{item.name}}<small>{{item.host}}</small></td><td>{{item.event==='down'?'Outage':item.event==='up'?'Recovery':'Test'}}<small class="event-id">{{item.id}}</small></td><td>{{item.status}}</td><td>{{item.attempts}}</td><td>{{item.result}}</td></tr><tr v-if="!items.length"><td colspan="6">No webhook deliveries yet. Save your endpoint and send a test to check connectivity.</td></tr></tbody></table></div>
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
</style>
