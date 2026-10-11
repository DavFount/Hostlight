<script setup lang="ts">
import {onMounted,ref} from 'vue'
const api=()=> (window as unknown as {go?:{main:{App:{GetIncidents:(search:string,status:string,days:number,offset:number)=>Promise<{retentionDays:number;error:string}>;SetHistoryRetention:(days:number)=>Promise<void>}}}}).go?.main.App
const retention=ref(90),saved=ref(90),busy=ref(false),loaded=ref(false),error=ref('')
async function refresh(){if(!api())return;try{const page=await api()!.GetIncidents('','all',30,0);if(page.error)throw new Error(page.error);saved.value=page.retentionDays;retention.value=saved.value;loaded.value=true}catch(e){error.value=String(e)}}
async function save(){const value=retention.value;if(value<saved.value&&!confirm(`Keep only ${value} days of completed history? Older completed incidents will be removed.`)){retention.value=saved.value;return}busy.value=true;error.value='';try{await api()!.SetHistoryRetention(value);saved.value=value}catch(e){error.value=String(e);retention.value=saved.value}finally{busy.value=false}}
onMounted(refresh)
</script>
<template><section class="history-settings"><h2>History retention</h2><p>Older completed incidents are deleted automatically. Active incidents are kept. Completed history is also capped at approximately 50,000 records.</p><label>Keep completed history for <select v-model.number="retention" :disabled="busy||!loaded" @change="save"><option :value="7">7 days</option><option :value="30">30 days</option><option :value="90">90 days</option><option :value="365">1 year</option><option v-if="![7,30,90,365].includes(retention)" :value="retention">{{retention}} days</option></select></label><p v-if="!api()">Open Hostlight through Wails to change retention.</p><p v-if="error" role="alert">{{error}}</p></section></template>
