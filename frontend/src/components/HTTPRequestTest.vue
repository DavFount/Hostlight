<script setup lang="ts">
import {ref,watch,onUnmounted} from 'vue'
const props=defineProps<{target:Record<string,unknown>}>()
type Result={success:boolean;statusCode:number;latency:number;message:string;timing?:string}
type API={TestHTTPRequest:(target:Record<string,unknown>)=>Promise<Result>}
const api=()=>(window as unknown as {go?:{main:{App:API}}}).go?.main.App
const busy=ref(false),error=ref(''),result=ref<Result|null>(null)
let generation=0
watch(()=>JSON.stringify(props.target),()=>{generation++;result.value=null;error.value='';busy.value=false})
onUnmounted(()=>generation++)
async function test(){const request=++generation;busy.value=true;error.value='';result.value=null;try{const response=await api()!.TestHTTPRequest({...props.target});if(request===generation)result.value=response}catch(e){if(request===generation)error.value=String(e)}finally{if(request===generation)busy.value=false}}
</script>
<template>
 <section class="http-preview" aria-label="HTTP request preview">
  <button type="button" :disabled="busy||!api()" @click="test">{{busy?'Testing request…':'Test HTTP request'}}</button>
  <p class="check-help">Sends one GET with the current URL, response rules, and timeout. Does not save a monitor or send an alert. Redirects are not followed; response bodies are not stored.</p>
  <p v-if="result" role="status" :class="{'http-match':result.success,'http-mismatch':!result.success}"><strong>{{result.success?'Expected response received':'Check failed'}}</strong><br>{{result.message}}<br>{{result.latency.toFixed(1)}} ms · {{result.timing||(result.statusCode?'time to response headers':'elapsed attempt time')}}</p>
  <p v-if="error" class="error" role="alert">{{error}}</p>
 </section>
</template>
<style scoped>
.http-preview{margin-top:18px;padding:14px;border:1px solid var(--border);border-radius:8px}.http-preview p{font-size:12px;overflow-wrap:anywhere}.http-match{color:var(--accent)}.http-mismatch{color:var(--text);border-left:3px solid #c38a29;padding-left:10px}
</style>
