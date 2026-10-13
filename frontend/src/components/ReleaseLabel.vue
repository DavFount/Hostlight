<script setup lang="ts">
import {onMounted,onUnmounted,ref} from 'vue'

const label=ref('')
let active=true
onUnmounted(()=>{active=false})
onMounted(async()=>{
 const app=(window as unknown as {go?:{main?:{App?:{GetSupportReport?:()=>Promise<{release:string}>}}}}).go?.main?.App
 if(!app?.GetSupportReport){label.value='Browser preview';return}
 try{
  const report=await app.GetSupportReport()
  if(active)label.value=report.release==='early preview'?'Development build':report.release?`v${report.release}`:'Version unavailable'
 }catch{if(active)label.value='Version unavailable'}
})
</script>

<template><footer>HOSTLIGHT<span v-if="label"> · {{label}}</span></footer></template>
