<script setup lang="ts">
import {onMounted,onUnmounted,ref} from 'vue'
type Status={supported:boolean;available:boolean;closeToTray:boolean}
const api=()=> (window as unknown as {go?:{main:{App:{GetDesktopStatus?:()=>Promise<Status>;SetCloseToTray:(enabled:boolean)=>Promise<void>;HideHostlight:()=>Promise<void>}}}}).go?.main.App
const status=ref<Status>({supported:false,available:false,closeToTray:false}),busy=ref(false),error=ref(''),stopped=ref(false)
let timer:ReturnType<typeof setInterval>,pending=false
async function refresh(){if(!api()?.GetDesktopStatus||pending||busy.value)return;pending=true;try{const next=await api()!.GetDesktopStatus!();if(!stopped.value)status.value=next}catch{if(!stopped.value)error.value='Desktop controls unavailable.'}finally{pending=false}}
async function toggle(event:Event){const input=event.target as HTMLInputElement;const enabled=input.checked;input.checked=status.value.closeToTray;busy.value=true;error.value='';try{await api()!.SetCloseToTray(enabled);status.value={...status.value,closeToTray:enabled}}catch{error.value='Unable to save close behavior. The tray may be unavailable or settings could not be saved.'}finally{busy.value=false}}
async function hide(){busy.value=true;error.value='';try{await api()!.HideHostlight()}catch{error.value='The tray is unavailable. Hostlight will stay visible.'}finally{busy.value=false}}
onMounted(()=>{refresh();timer=setInterval(refresh,3000)})
onUnmounted(()=>{stopped.value=true;clearInterval(timer)})
</script>
<template>
 <section class="desktop-controls"><h2>Desktop behavior</h2><p v-if="!api()?.GetDesktopStatus">Open Hostlight through Wails to change desktop behavior.</p><div v-else><p>{{status.available?'Tray ready · checks continue while hidden.':status.supported?'Tray unavailable · closing quits Hostlight.':'Tray integration is Windows-only.'}}</p><label><input type="checkbox" :checked="status.closeToTray" :disabled="busy||(!status.available&&!status.closeToTray)" @change="toggle"> Close to tray</label><p v-if="status.closeToTray">Use File → Quit Hostlight (Ctrl+Q), or Quit from the tray, to stop monitoring. In-app popups are not visible while hidden.</p><button :disabled="busy||!status.available" @click="hide">Hide to tray</button><p v-if="error" role="alert">{{error}}</p></div></section>
</template>
<style scoped>
.desktop-controls label{display:flex;align-items:center;gap:8px}.desktop-controls input{padding:0}.desktop-controls button{margin-top:16px}
</style>
