<script setup lang="ts">
import {computed,onMounted,onUnmounted,ref,watch} from 'vue'
import IncidentHistory from './components/IncidentHistory.vue'
import WebhookAlerts from './components/WebhookAlerts.vue'
import AlertSilences from './components/AlertSilences.vue'
import MonitorBackups from './components/MonitorBackups.vue'
import hostlightLogo from './assets/hostlight.svg'
import HTTPRequestTest from './components/HTTPRequestTest.vue'
import HTTPBodyRule from './components/HTTPBodyRule.vue'
type Silence={id:string;targetId:string;kind:string;starts:string;ends:string;reason:string}
type Target={id:string;name:string;host:string;type:'icmp'|'tcp'|'http';port:number;group:string;interval:number;timeout:number;failures:number;recovery:number;state:string;latency:number;message:string;checked:string;expectedStatus?:number;bodyMode?:string;bodyExpected?:string;jsonPointer?:string;jsonType?:string}
type BulkRow={line:number;target:Target;error:string}
type API={GetAlertSilences:()=>Promise<Silence[]>;GetTargets:()=>Promise<Target[]>;GetConfigurationStatus:()=>Promise<{path:string;error:string;historyError:string}>;SaveTarget:(t:Target)=>Promise<void>;DeleteTarget:(id:string)=>Promise<void>;PauseTarget:(id:string)=>Promise<void>;ResumeTarget:(id:string)=>Promise<void>;PreviewBulk:(input:string,defaults:Target)=>Promise<BulkRow[]>;AddBulk:(input:string,defaults:Target)=>Promise<number>;SetTargetsPaused:(ids:string[],paused:boolean)=>Promise<void>}
const api=()=>(window as unknown as {go?:{main:{App:API}}}).go?.main.App
const targets=ref<Target[]>([]),search=ref(''),filter=ref('all'),error=ref(''),editing=ref(false),busy=ref(false)
const configuration=ref({path:'',error:'',historyError:''}),view=ref('monitors')
const silences=ref<Silence[]>([]),silencing=ref<Target|null>(null),now=ref(Date.now())
const silenceBadges=computed(()=>{const map=new Map<string,Silence>();for(const s of silences.value){if(new Date(s.ends).getTime()<=now.value)continue;const prior=map.get(s.targetId);const active=new Date(s.starts).getTime()<=now.value;const priorActive=prior&&new Date(prior.starts).getTime()<=now.value;if(!prior||(active&&!priorActive)||(active&&priorActive&&s.ends>prior.ends)||(!active&&!priorActive&&s.starts<prior.starts))map.set(s.targetId,s)}return map})
function silenceLabel(id:string){const s=silenceBadges.value.get(id);if(!s)return '';return new Date(s.starts).getTime()<=now.value?`Alerts silenced until ${new Date(s.ends).toLocaleString()}`:`Maintenance scheduled: ${new Date(s.starts).toLocaleString()}`}
const selectedGroup=ref<string|null>(null),bulk=ref(false),bulkInput=ref(''),bulkRows=ref<BulkRow[]>([])
function silenceActive(id:string){const s=silenceBadges.value.get(id);return !!s&&new Date(s.starts).getTime()<=now.value}
function checkSucceeded(t:Target){return t.type==='http'?(t.message.endsWith('· Expected status received')||t.message.endsWith('· Expected status and body matched')):t.message===(t.type==='tcp'?'TCP connection successful':'Ping reply received')}
function checkDiagnostic(t:Target){return t.type==='tcp'&&t.checked&&t.state!=='paused'&&!checkSucceeded(t)?`Connection attempt failed after ${t.latency.toFixed(1)} ms · ${t.message}`:t.message}
const previewSignature=ref('')
const groups=computed(()=>[...new Set(targets.value.map(t=>t.group).filter(Boolean))].sort())
const theme=ref(localStorage.getItem('hostlight.theme')||'dark'),density=ref(localStorage.getItem('hostlight.density')||'comfortable')
watch(theme,v=>{document.documentElement.dataset.theme=v;localStorage.setItem('hostlight.theme',v)},{immediate:true})
watch(density,v=>localStorage.setItem('hostlight.density',v))
const blank=():Target=>({id:'',name:'',host:'',type:'icmp',port:443,group:'',interval:5,timeout:3,failures:3,recovery:2,state:'unknown',latency:0,message:'',checked:'',expectedStatus:200,bodyMode:'',bodyExpected:'',jsonPointer:'/status',jsonType:'string'})
const draft=ref(blank())
watch([bulkInput,draft],()=>{bulkRows.value=[]},{deep:true})
const commonPorts=[
 {port:80,name:'HTTP'}, {port:443,name:'HTTPS'}, {port:22,name:'SSH / SFTP'},
 {port:21,name:'FTP control'}, {port:25,name:'SMTP'}, {port:53,name:'DNS (TCP)'},
 {port:88,name:'Kerberos (TCP)'}, {port:110,name:'POP3'}, {port:135,name:'RPC endpoint mapper'},
 {port:139,name:'NetBIOS session'}, {port:143,name:'IMAP'}, {port:389,name:'LDAP'},
 {port:445,name:'SMB'}, {port:465,name:'SMTP over TLS'}, {port:587,name:'SMTP submission'},
 {port:636,name:'LDAPS'}, {port:993,name:'IMAP over TLS'}, {port:995,name:'POP3 over TLS'},
 {port:1433,name:'Microsoft SQL Server'}, {port:1521,name:'Oracle listener'},
 {port:2049,name:'NFS (TCP)'}, {port:3268,name:'AD global catalog'}, {port:3269,name:'AD global catalog over TLS'},
 {port:3306,name:'MySQL / MariaDB'}, {port:3389,name:'Remote Desktop'}, {port:5432,name:'PostgreSQL'},
 {port:5900,name:'VNC'}, {port:5985,name:'WinRM HTTP'}, {port:5986,name:'WinRM HTTPS'},
 {port:6379,name:'Redis'}, {port:8080,name:'Alternate HTTP'}, {port:8443,name:'Alternate HTTPS'},
]
const customPort=ref(false)
const portPreset=computed({
 get:()=>!customPort.value&&commonPorts.some(service=>service.port===draft.value.port)?String(draft.value.port):'custom',
 set:(value:string)=>{customPort.value=value==='custom';if(value!=='custom')draft.value.port=Number(value)},
})
watch(()=>draft.value.type,type=>{if(type==='tcp' && !draft.value.port)draft.value.port=443;if(type==='http'&&!draft.value.expectedStatus)draft.value.expectedStatus=200})
const visible=computed(()=>targets.value.filter(t=>(selectedGroup.value===null||t.group===selectedGroup.value)&&(filter.value==='all'||t.state===filter.value)&&`${t.name} ${t.host} ${t.group}`.toLowerCase().includes(search.value.toLowerCase())).sort((a,b)=>a.name.localeCompare(b.name)))
const count=(state:string)=>targets.value.filter(t=>t.state===state).length
async function refresh(){if(!api())return;now.value=Date.now();try{[targets.value,configuration.value,silences.value]=await Promise.all([api()!.GetTargets(),api()!.GetConfigurationStatus(),api()!.GetAlertSilences().catch(e=>{error.value='Alert controls unavailable: '+String(e);return silences.value})])}catch(e){error.value=String(e)}}
function edit(t?:Target){bulk.value=false;draft.value=t?{...t}:blank();if(!t)draft.value.group=selectedGroup.value??'';customPort.value=false;error.value='';editing.value=true}
function editBulk(){edit();bulk.value=true;bulkInput.value='';bulkRows.value=[]}
async function save(){
 busy.value=true;error.value=''
 try{
  if(!api())throw new Error('Open Hostlight through Wails for live monitoring.')
  if(bulk.value){
   const signature=JSON.stringify([bulkInput.value,draft.value])
   if(!bulkRows.value.length||previewSignature.value!==signature){
    const rows=await api()!.PreviewBulk(bulkInput.value,{...draft.value})
    if(signature!==JSON.stringify([bulkInput.value,draft.value]))return
    bulkRows.value=rows;previewSignature.value=signature
    if(!rows.length)error.value='Enter at least one host.'
    return
   }
   await api()!.AddBulk(bulkInput.value,{...draft.value})
  }else await api()!.SaveTarget({...draft.value})
  editing.value=false;await refresh()
 }catch(e){error.value=String(e)}finally{busy.value=false}
}
async function pauseVisible(paused:boolean){busy.value=true;error.value='';try{await api()!.SetTargetsPaused(visible.value.map(t=>t.id),paused);await refresh()}catch(e){error.value=String(e)}finally{busy.value=false}}
async function action(t:Target,remove=false){try{if(remove){if(!confirm(`Remove ${t.name}?`))return;await api()!.DeleteTarget(t.id)}else if(t.state==='paused')await api()!.ResumeTarget(t.id);else await api()!.PauseTarget(t.id);await refresh()}catch(e){error.value=String(e)}}
let timer:ReturnType<typeof setInterval>
onMounted(async()=>{if(api())configuration.value=await api()!.GetConfigurationStatus();refresh();timer=setInterval(refresh,1000)})
onUnmounted(()=>clearInterval(timer))
</script>
<template>
<div class="shell" :class="density">
<aside><div class="brand"><img :src="hostlightLogo" alt="" width="36" height="36"> Hostlight</div><div class="workspace"><small>WORKSPACE</small><strong>Local network</strong><span>Desktop monitoring</span></div><button class="nav" :aria-current="view==='monitors'?'page':undefined" @click="view='monitors'">▦ Monitors <span>{{targets.length}}</span></button><button class="nav" :aria-current="view==='history'?'page':undefined" @click="view='history'">◷ Incident history</button><button class="nav" :aria-current="view==='alerts'?'page':undefined" @click="view='alerts'">♧ Webhook alerts</button><button class="nav" :aria-current="view==='backups'?'page':undefined" @click="view='backups'">⇄ Monitor backups</button><p class="sidebar-note">● Checks run on this computer<br>Keep Hostlight open to monitor your services.</p><div class="appearance"><label for="theme">Appearance</label><select id="theme" v-model="theme"><option value="dark">Dark</option><option value="light">Light</option></select><label for="density">Card spacing</label><select id="density" v-model="density"><option value="comfortable">Comfortable</option><option value="compact">Compact</option></select></div><footer>HOSTLIGHT · EARLY PREVIEW</footer></aside>
 <main><header><div><small>{{view==='backups'?'MONITOR BACKUPS':view==='alerts'?'WEBHOOK ALERTS':view==='history'?'INCIDENT HISTORY':'NETWORK OVERVIEW'}}</small><h1>{{view==='backups'?'Take your monitoring setup with you.':view==='alerts'?'Know when availability changes.':view==='history'?'Every outage has a timeline.':'Your network, at a glance.'}}</h1><p>{{view==='backups'?'Back up definitions. Review imports. Keep existing monitors intact.':view==='alerts'?'Connect Hostlight to your incident workflow.':view==='history'?'Review availability changes, recovery, and monitoring gaps.':'Watch availability. Catch changes. Stay in control.'}}</p></div><div v-if="view==='monitors'" class="header-actions"><button @click="editBulk()">Add multiple</button><button class="primary" @click="edit()">＋ Add monitor</button></div></header>
 <div class="notice">Ping, TCP &amp; HTTP monitoring · Targets and pause settings are saved automatically. Checks resume when you reopen Hostlight.</div><div v-if="configuration.error" class="error" role="alert">{{configuration.error}} Settings file: {{configuration.path}}</div><div v-if="!api()" class="notice">Browser preview. Launch through Wails for live checks.</div><div v-if="error&&!editing" class="error" role="alert">{{error}}</div>
 <div v-if="configuration.historyError" class="error" role="alert">{{configuration.historyError}}</div>
 <IncidentHistory v-if="view==='history'" />
 <WebhookAlerts v-else-if="view==='alerts'" />
 <MonitorBackups v-else-if="view==='backups'" @changed="refresh" />
 <template v-else>
 <section class="metrics"><button @click="filter='all'"><span>Total monitors</span><strong>{{targets.length}}</strong><small>Across your workspace</small></button><button @click="filter='up'"><span>● Available</span><strong>{{count('up')}}</strong><small>Services responding</small></button><button @click="filter='down'"><span>● Unavailable</span><strong>{{count('down')}}</strong><small>Failure threshold reached</small></button><button @click="filter='paused'"><span>● Paused</span><strong>{{count('paused')}}</strong><small>Checks temporarily stopped</small></button></section>
 <div class="section-heading"><h2>Monitors <small>{{visible.length}}</small></h2><div class="tools"><input v-model="search" aria-label="Search monitors" placeholder="Search name, host, or group…"><select v-model="filter" aria-label="Filter status"><option value="all">All statuses</option><option value="up">Available</option><option value="down">Unavailable</option><option value="unknown">Checking</option><option value="error">Check error</option><option value="paused">Paused</option></select></div></div>
 <div v-if="targets.length" class="group-toolbar"><label>Group <select v-model="selectedGroup"><option :value="null">All groups</option><option value="">Ungrouped</option><option v-for="group in groups" :key="group" :value="group">{{group}}</option></select></label><span>{{visible.length}} shown</span><button :disabled="busy||!api()||!visible.some(t=>t.state!=='paused')" @click="pauseVisible(true)">Pause shown</button><button :disabled="busy||!api()||!visible.some(t=>t.state==='paused')" @click="pauseVisible(false)">Resume shown</button></div>
 <section v-if="!targets.length" class="empty"><div>⌁</div><h2>A clear view starts with one monitor.</h2><p>Ping a host, check a TCP port, or monitor an HTTP health endpoint.<br>No scripts or configuration files needed.</p><button class="primary" @click="edit()">＋ Add your first monitor</button><p><small>Start with your gateway's IP address. Ping needs no port.</small></p></section>
 <section v-else-if="!visible.length" class="empty"><h2>No matching monitors</h2><p>Try another search or filter.</p></section>
 <section v-else class="grid"><article v-for="t in visible" :key="t.id"><div class="card-heading"><small>{{t.type==='icmp'?'PING / ICMP':t.type==='http'?'HTTP / HTTPS':`TCP / ${t.port}`}}</small><span class="state" :class="t.state">● {{t.state==='up'?'Available':t.state==='down'?'Unavailable':t.state==='paused'?'Paused':t.state==='error'?'Check error':'Checking'}}</span></div><h3>{{t.name}}</h3><p class="address">{{t.host}}<span v-if="t.group"> · {{t.group}}</span></p><p v-if="silenceBadges.has(t.id)" class="silence-badge" :class="{active:silenceActive(t.id)}" :title="silenceBadges.get(t.id)?.reason">{{silenceLabel(t.id)}}</p><div class="reading"><strong>{{t.checked && checkSucceeded(t)?t.latency.toFixed(1):'—'}}</strong><small>ms · {{t.type==='icmp'?'last echo reply':t.type==='http'?(t.bodyMode?'last complete response check':'last matching response headers'):'last successful connection'}}</small></div><p class="diagnostic" :title="checkDiagnostic(t)">{{checkDiagnostic(t)}}</p><div class="card-footer"><small>Every {{t.interval}}s</small><div><button @click="edit(t)">Edit</button><button title="Maintenance and alert snoozing" @click="silencing=t">Silence alerts</button><button @click="action(t)">{{t.state==='paused'?'Resume':'Pause'}}</button><button class="danger" @click="action(t,true)">Remove</button></div></div></article></section>
 <p class="bottom-note">Ping checks echo replies; TCP checks port reachability; HTTP checks response status. A firewall may block ping even when services are available.</p><p v-if="configuration.path" class="bottom-note">Settings: {{configuration.path}}</p></template></main>
 <AlertSilences v-if="silencing" :target="silencing" :silences="silences" @close="silencing=null" @changed="refresh" />
 <div v-if="editing" class="backdrop" @click.self="editing=false">
  <section class="modal" role="dialog" aria-modal="true" aria-labelledby="editor-title" @keydown.esc="editing=false">
   <div class="modal-heading"><div><small>NETWORK MONITOR</small><h2 id="editor-title">{{bulk?'Add multiple monitors':draft.id?'Edit monitor':'Add a monitor'}}</h2></div><button @click="editing=false" aria-label="Close">✕</button></div>
   <form @submit.prevent="save">
    <label>Check type<select v-model="draft.type"><option value="icmp">Ping (ICMP) — host reachability</option><option value="tcp">TCP service — port reachability</option><option value="http">HTTP / HTTPS — response status</option></select></label>
    <p class="check-help">{{draft.type==='icmp'?'Send an echo request to the host. No port is needed.':draft.type==='http'?'Send a GET request and require the expected HTTP status. HTTPS certificates are verified.':'Check whether a service accepts connections on a specific port.'}}</p>
    <template v-if="!bulk"><label>Display name<input v-model="draft.name" required autofocus placeholder="e.g. Office gateway"></label><label>{{draft.type==='http'?'HTTP/HTTPS URL':'Hostname or IP address'}}<input v-model="draft.host" required :placeholder="draft.type==='http'?'https://service.example/health':'e.g. 192.168.1.1'"></label></template>
    <template v-else><label>{{draft.type==='http'?'URLs':'Hosts'}}<textarea v-model="bulkInput" rows="6" required :placeholder="draft.type==='http'?'https://service.example/health\nPortal | https://portal.example/':'192.168.1.1\nserver.example.com\nOffice gateway | 192.168.1.254'"></textarea></label><p class="check-help">One {{draft.type==='http'?'complete HTTP/HTTPS URL':'hostname or IP address'}} per line, up to 200. For a friendly name use: {{draft.type==='http'?'Portal | https://portal.example/':'Office gateway | 192.168.1.1'}}. All monitors use the check type and settings below.</p></template>
    <template v-if="draft.type==='tcp'">
     <label>Common service<select v-model="portPreset"><option v-for="service in commonPorts" :key="service.port" :value="String(service.port)">{{service.name}} — {{service.port}}</option><option value="custom">Custom port…</option></select></label>
     <label v-if="portPreset==='custom'">Custom TCP port<input v-model.number="draft.port" type="number" min="1" max="65535" required></label>
     <p class="check-help">These are common defaults. Choose Custom port if your service uses a different port. This checks the TCP connection only.</p>
    </template>
    <template v-if="draft.type==='http'"><label>Expected HTTP status<input v-model.number="draft.expectedStatus" type="number" min="100" max="599" required></label><p class="check-help">Default: 200 (OK). HTTP is unencrypted. URLs are saved in plain text and included in history, exports, and webhook payloads: do not include passwords, tokens, or secret query parameters. Authentication is not supported yet.</p><HTTPBodyRule v-model:mode="draft.bodyMode" v-model:expected="draft.bodyExpected" v-model:pointer="draft.jsonPointer" v-model:value-type="draft.jsonType" /><HTTPRequestTest v-if="!bulk" :target="draft" /></template>
    <label>Group (optional)<input v-model="draft.group" list="group-names" placeholder="e.g. Production"><datalist id="group-names"><option v-for="group in groups" :key="group" :value="group" /></datalist></label>
    <details><summary>Check timing and status rules</summary><div class="form-row"><label>Check every (seconds)<input v-model.number="draft.interval" type="number" min="1" max="3600" required></label><label>Timeout (seconds)<input v-model.number="draft.timeout" type="number" min="1" max="60" required></label></div><div class="form-row"><label>Failures before unavailable<input v-model.number="draft.failures" type="number" min="1" max="100" required></label><label>Successes before available<input v-model.number="draft.recovery" type="number" min="1" max="100" required></label></div><p>Consecutive checks filter brief interruptions. The interval starts after each check finishes.</p></details>
    <section v-if="bulkRows.length" class="bulk-preview" aria-label="Validation preview"><h3>Review {{bulkRows.length}} monitors</h3><p class="check-help">{{bulkRows.some(row=>row.error)?'Fix the highlighted lines before adding monitors.':'All lines are valid. Confirm to save and start monitoring.'}}</p><div v-for="row in bulkRows" :key="row.line" class="preview-row" :class="{'invalid-row':row.error}"><small>Line {{row.line}}</small><strong>{{row.target.name}}</strong><span>{{row.target.host}}</span><span v-if="row.error" role="alert">{{row.error}}</span><span v-else>Ready</span></div></section>
    <div v-if="error" class="error" role="alert">{{error}}</div><div class="form-actions"><button type="button" @click="editing=false">Cancel</button><button class="primary" :disabled="busy||(bulkRows.length>0&&bulkRows.some(row=>row.error))">{{busy?'Working…':bulk?(bulkRows.length?`Add ${bulkRows.length} monitors`:'Review monitors'):draft.id?'Save changes':'Start monitoring'}}</button></div>
   </form>
  </section>
 </div>
</div>
</template>
