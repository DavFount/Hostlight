<script setup lang="ts">
import {ref} from 'vue'
type Monitor={name:string;host:string;type:string;port:number;group:string;interval:number;timeout:number;failures:number;recovery:number;paused:boolean;expectedStatus?:number;bodyMode?:string;bodyExpected?:string;jsonPointer?:string;jsonType?:string;tlsMinDays?:number}
type Preview={token:string;source:string;rows:{row:number;monitor:Monitor;status:string;detail:string}[];ready:number;skipped:number;invalid:number}
type Result={added:number;skipped:number;paused:number}
type API={ExportMonitorBackup:()=>Promise<string>;ChooseMonitorImport:()=>Promise<Preview>;CommitMonitorImport:(token:string,start:boolean)=>Promise<Result>;DismissMonitorImport:(token:string)=>Promise<void>}
const emit=defineEmits<{changed:[]}>()
const api=()=>(window as unknown as {go?:{main:{App:API}}}).go?.main.App
const preview=ref<Preview|null>(null),start=ref(false),busy=ref(false),error=ref(''),notice=ref('')
async function exportBackup(){busy.value=true;error.value='';notice.value='';try{const path=await api()!.ExportMonitorBackup();if(path)notice.value=`Monitor backup saved to ${path}`}catch(e){error.value=String(e)}finally{busy.value=false}}
async function choose(){busy.value=true;error.value='';notice.value='';start.value=false;try{if(preview.value)await api()!.DismissMonitorImport(preview.value.token);preview.value=null;const result=await api()!.ChooseMonitorImport();if(result.token)preview.value=result}catch(e){error.value=String(e)}finally{busy.value=false}}
async function cancel(){busy.value=true;try{if(preview.value)await api()!.DismissMonitorImport(preview.value.token);preview.value=null;start.value=false;error.value=''}catch(e){error.value=String(e)}finally{busy.value=false}}
async function commit(){if(!preview.value)return;busy.value=true;error.value='';notice.value='';try{const result=await api()!.CommitMonitorImport(preview.value.token,start.value);preview.value=null;start.value=false;notice.value=`Imported ${result.added} ${result.added===1?'monitor':'monitors'}. ${result.paused} paused; ${result.skipped} ${result.skipped===1?'duplicate':'duplicates'} skipped. Existing monitors were not changed.`;emit('changed')}catch(e){error.value=String(e)}finally{busy.value=false}}
</script>
<template>
 <section class="monitor-backups">
  <div v-if="error" class="error" role="alert">{{error}}</div><p v-if="notice" role="status" class="backup-notice">{{notice}}</p>
  <div class="backup-actions">
   <article><h2>Back up your monitors</h2><p>Save names, hosts, groups, ICMP/TCP/HTTP settings, timing rules, and pause preferences to a portable JSON file.</p><p>Webhook destinations and credentials, maintenance windows, incident history, and appearance settings are not included. Hostnames and IP addresses are included—store the file appropriately.</p><button :disabled="busy||!api()" @click="exportBackup">Export monitor backup</button></article>
   <article><h2>Bring monitors into Hostlight</h2><p>Choose a Hostlight monitor backup and review every entry before importing. Existing monitors are never replaced. Duplicate host/check combinations are skipped.</p><p>Imports start paused by default and receive new monitor IDs. Fix invalid entries before importing. Maximum: 1,000 monitors and a 1 MiB file.</p><button class="primary" :disabled="busy||!api()" @click="choose">Choose backup to review</button></article>
  </div>
  <section v-if="preview" class="import-review" aria-label="Monitor import preview">
   <h2>Review {{preview.source}}</h2><p>{{preview.ready}} ready to add · {{preview.skipped}} {{preview.skipped===1?'duplicate':'duplicates'}} skipped · {{preview.invalid}} invalid</p>
   <div v-if="preview.invalid" class="error" role="alert">Fix invalid settings in the backup and choose the file again. Nothing will be imported until all entries are valid.</div>
   <div class="history-table-wrap"><table class="history-table"><thead><tr><th>Entry</th><th>Monitor</th><th>Group</th><th>Check settings</th><th>Result</th></tr></thead><tbody><tr v-for="row in preview.rows" :key="row.row"><td>{{row.row}}</td><td>{{row.monitor.name}}<small>{{row.monitor.host}}</small></td><td>{{row.monitor.group||'Ungrouped'}}</td><td>{{row.monitor.type==='icmp'?'Ping / ICMP':row.monitor.type==='http'?`HTTP GET / expected ${row.monitor.expectedStatus||200}`:`TCP / ${row.monitor.port}`}}<small v-if="row.monitor.tlsMinDays">Certificate requires at least {{row.monitor.tlsMinDays}} days remaining</small><small v-if="row.monitor.bodyMode==='contains'">Contains text: {{row.monitor.bodyExpected}}</small><small v-if="row.monitor.bodyMode==='json'">JSON {{row.monitor.jsonPointer||'(root)'}} equals {{row.monitor.jsonType==='null'?'null':row.monitor.bodyExpected}} ({{row.monitor.jsonType||'string'}})</small><small>Every {{row.monitor.interval}}s · timeout {{row.monitor.timeout}}s</small><small>{{row.monitor.failures}} failures / {{row.monitor.recovery}} successes</small><small>{{row.status==='skipped'?'Existing monitor unchanged':row.status==='invalid'?'Will not import':row.monitor.paused?'Paused in backup':start?'Will start monitoring':'Will import paused'}}</small></td><td><strong>{{row.status==='ready'?'Ready':row.status==='skipped'?'Skipped':'Invalid'}}</strong><small>{{row.detail}}</small></td></tr></tbody></table></div>
   <label class="start-imported"><input v-model="start" type="checkbox" :disabled="busy"> Start imported monitors that were not paused in the backup</label>
   <p class="check-help">Starting sends network checks immediately. Existing global or group webhook rules apply to future availability changes. Individual monitor assignments and maintenance windows are not imported.</p>
   <p class="check-help">Preview expires after 15 minutes. If monitor settings change, choose the file again to refresh the review.</p>
   <div class="header-actions"><button :disabled="busy" @click="cancel">Cancel review</button><button class="primary" :disabled="busy||preview.invalid>0||preview.ready===0" @click="commit">Import {{preview.ready}} {{start?(preview.ready===1?'monitor':'monitors'):(preview.ready===1?'paused monitor':'paused monitors')}}</button></div>
  </section>
 </section>
</template>
<style scoped>
.backup-actions{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:20px;margin:24px 0}.backup-actions article{padding:24px;border:1px solid var(--border);border-radius:12px;background:var(--panel)}.backup-actions p{font-size:13px;line-height:1.7}.backup-actions button{margin-top:10px}.import-review{margin-top:30px;display:grid;gap:16px}.import-review>p{margin:0;font-size:13px}.import-review .history-table-wrap{max-height:440px;overflow:auto}.start-imported{display:flex;align-items:center;gap:10px;font-size:13px}.start-imported input{flex-shrink:0}.backup-notice{overflow-wrap:anywhere}.header-actions{flex-wrap:wrap}@media(max-width:1000px){.backup-actions{grid-template-columns:1fr}}
</style>
