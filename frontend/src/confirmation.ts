import {onUnmounted,shallowRef} from 'vue'

export type DialogRequest={title:string;message:string;confirmLabel?:string;destructive?:boolean;mode?:'confirm'|'alert';returnFocus?:HTMLElement}

export function useConfirmation(){
 const dialog=shallowRef<DialogRequest>()
 let resolve:((confirmed:boolean)=>void)|undefined
 function settle(confirmed:boolean){const pending=resolve;resolve=undefined;dialog.value=undefined;pending?.(confirmed)}
 function ask(request:DialogRequest):Promise<boolean>{
  if(resolve)return Promise.resolve(false)
  // Capture before busy-state rendering disables and blurs the initiating button.
  dialog.value={...request,returnFocus:document.activeElement instanceof HTMLElement?document.activeElement:undefined}
  return new Promise<boolean>(done=>{resolve=done})
 }
 onUnmounted(()=>settle(false))
 return {dialog,ask,settle}
}
