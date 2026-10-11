<script setup lang="ts">
import {onMounted,ref} from 'vue'
type Settings={concurrency:number;spreadMilliseconds:number}
type Status={saved:Settings;active:Settings;restartRequired:boolean}
const api=()=> (window as unknown as {go?:{main:{App:{GetSchedulingSettings?:()=>Promise<Status>;SaveSchedulingSettings:(s:Settings)=>Promise<void>}}}}).go?.main.App
const status=ref<Status>(),concurrency=ref(32),spread=ref(250),busy=ref(false),error=ref('')
async function refresh(){const s=await api()!.GetSchedulingSettings!();status.value=s;concurrency.value=s.saved.concurrency;spread.value=s.saved.spreadMilliseconds}
async function save(){busy.value=true;error.value='';try{await api()!.SaveSchedulingSettings({concurrency:concurrency.value,spreadMilliseconds:spread.value});await refresh()}catch(e){error.value=String(e)}finally{busy.value=false}}
onMounted(async()=>{if(api()?.GetSchedulingSettings)try{await refresh()}catch{error.value='Scheduling settings unavailable.'}})
</script>
<template><section><h2>Check scheduling</h2><p v-if="!api()?.GetSchedulingSettings">Open Hostlight through Wails to change scheduling.</p><div v-else><p>Limits include HTTP request previews. Higher limits can increase network and CPU load.</p><form @submit.prevent="save"><label>Simultaneous checks <input v-model.number="concurrency" type="number" min="1" max="128" required :disabled="busy"></label><label>Start spread (milliseconds) <input v-model.number="spread" type="number" min="0" max="5000" required :disabled="busy"></label><p>Spreads first checks after start or resume. Zero starts immediately. Each interval begins after the previous check finishes; busy checks wait for a slot.</p><button :disabled="busy">Save scheduling</button></form><p v-if="status">Active limit: {{status.active.concurrency}} · Spread: {{status.active.spreadMilliseconds}} ms</p><p v-if="status?.restartRequired" role="status">Saved. Quit and reopen Hostlight to apply. Hiding to tray is not a restart.</p><p v-if="error" role="alert">{{error}}</p></div></section></template>
<style scoped>label{display:block;margin:16px 0}input{display:block;width:100%;max-width:320px;margin-top:6px}</style>
