<script setup lang="ts">
import {onMounted,onUnmounted,ref} from 'vue'
type Status={supported:boolean;available:boolean;closeToTray:boolean}
const api=()=> (window as unknown as {go?:{main:{App:{GetDesktopStatus?:()=>Promise<Status>;SetCloseToTray:(enabled:boolean)=>Promise<void>;HideHostlight:()=>Promise<void>;QuitHostlight:()=>Promise<void>}}}}).go?.main.App
const status=ref<Status>({supported:false,available:false,closeToTray:false}),busy=ref(false),error=ref(''),confirmQuit=ref(false),stopped=ref(false)
let timer:ReturnType<typeof setInterval>,pending=false
async function refresh(){if(!api()?.GetDesktopStatus||pending||busy.value)return;pending=true;try{const next=await api()!.GetDesktopStatus!();if(!stopped.value)status.value=next}catch{if(!stopped.value)error.value='Desktop controls unavailable.'}finally{pending=false}}
async function toggle(event:Event){const input=event.target as HTMLInputElement;const enabled=input.checked;input.checked=status.value.closeToTray;busy.value=true;error.value='';try{await api()!.SetCloseToTray(enabled);status.value={...status.value,closeToTray:enabled}}catch{error.value='Unable to save close behavior. The tray may be unavailable or settings could not be saved.'}finally{busy.value=false}}
async function hide(){busy.value=true;error.value='';try{await api()!.HideHostlight()}catch{error.value='The tray is unavailable. Hostlight will stay visible.'}finally{busy.value=false}}
async function quit(){busy.value=true;error.value='';try{await api()!.QuitHostlight()}catch{error.value='Unable to quit. Try closing the window.';busy.value=false}}
onMounted(()=>{refresh();timer=setInterval(refresh,3000)})
onUnmounted(()=>{stopped.value=true;clearInterval(timer)})
</script>
<template>
 <details v-if="api()?.GetDesktopStatus" class="desktop-controls"><summary>Desktop behavior</summary><p>{{status.available?'Tray ready · checks continue while hidden.':status.supported?'Tray unavailable · closing quits Hostlight.':'Tray integration is Windows-only.'}}</p><label><input type="checkbox" :checked="status.closeToTray" :disabled="busy||(!status.available&&!status.closeToTray)" @change="toggle"> Close to tray</label><p v-if="status.closeToTray">Use Quit Hostlight to stop monitoring. In-app popups are not visible while hidden.</p><button :disabled="busy||!status.available" @click="hide">Hide to tray</button><button :disabled="busy" @click="confirmQuit=!confirmQuit">Quit Hostlight</button><div v-if="confirmQuit"><p>Quit and stop all checks and alert deliveries?</p><button :disabled="busy" @click="quit">Confirm quit</button><button :disabled="busy" @click="confirmQuit=false">Cancel</button></div><p v-if="error" role="alert">{{error}}</p></details>
</template>
<style scoped>
.desktop-controls{font-size:11px;margin-top:18px;padding-top:14px}.desktop-controls p{font-size:11px}.desktop-controls label{display:flex;align-items:center;gap:6px}.desktop-controls input{padding:0}.desktop-controls button{font-size:11px;padding:7px;margin:8px 4px 0 0;white-space:normal}
</style>
