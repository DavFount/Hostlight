<script setup lang="ts">
import {nextTick,onBeforeUnmount,ref,useId,watch} from 'vue'
import type {DialogRequest} from '../confirmation'
const props=defineProps<{request?:DialogRequest;busy?:boolean}>()
const emit=defineEmits<{resolve:[confirmed:boolean]}>()
const element=ref<HTMLDialogElement>(),cancelButton=ref<HTMLButtonElement>(),confirmButton=ref<HTMLButtonElement>()
const id=useId()
let opener:HTMLElement|null=null,submitted=false
function finish(confirmed:boolean){if(!props.request||submitted)return;submitted=true;emit('resolve',confirmed)}
watch(()=>({request:props.request,busy:props.busy}),async ({request})=>{
 if(request){
  if(element.value?.open)return
  submitted=false;opener=request.returnFocus??(document.activeElement instanceof HTMLElement?document.activeElement:null)
  await nextTick()
  if(props.request!==request||!element.value)return
  element.value.showModal()
  ;(request.mode==='alert'?confirmButton.value:cancelButton.value)?.focus()
 }else{
  element.value?.close()
  await nextTick()
  if(props.request||props.busy)return
  if(opener?.isConnected&&!opener.matches(':disabled'))opener.focus()
  opener=null
 }
},{flush:'post',immediate:true})
onBeforeUnmount(()=>{element.value?.close();if(opener?.isConnected&&!opener.matches(':disabled'))opener.focus()})
</script>
<template>
 <dialog ref="element" class="confirmation-dialog" :role="request?.mode==='alert'?'dialog':'alertdialog'" :aria-labelledby="id+'-title'" :aria-describedby="id+'-message'" @cancel.prevent="finish(false)" @close="!element?.open&&finish(false)">
  <template v-if="request"><h2 :id="id+'-title'">{{request.title}}</h2><p :id="id+'-message'">{{request.message}}</p><div class="confirmation-actions"><button v-if="request.mode!=='alert'" ref="cancelButton" type="button" @click="finish(false)">Cancel</button><button ref="confirmButton" type="button" :class="request.destructive?'confirm-destructive':'primary'" @click="finish(true)">{{request.confirmLabel||(request.mode==='alert'?'OK':'Confirm')}}</button></div></template>
 </dialog>
</template>
<style scoped>
.confirmation-dialog{position:fixed;inset:0;width:min(440px,calc(100vw - 32px));max-height:calc(100vh - 48px);margin:auto;padding:24px;background:var(--panel);color:var(--text);border:1px solid var(--border);border-radius:14px;box-shadow:0 24px 80px #0006;overflow:auto}.confirmation-dialog::backdrop{background:#050910b5}.confirmation-dialog h2{font-size:20px;line-height:1.3}.confirmation-dialog p{font-size:13px;line-height:1.6;white-space:pre-wrap;overflow-wrap:anywhere;margin:16px 0 24px}.confirmation-actions{display:flex;justify-content:flex-end;flex-wrap:wrap;gap:10px}.confirm-destructive{background:#aa3044;color:#fff;border-color:#aa3044;font-weight:600}.confirm-destructive:hover{background:#92283b;border-color:#92283b}
</style>
