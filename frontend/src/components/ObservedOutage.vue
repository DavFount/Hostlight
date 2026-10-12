<script setup lang="ts">
import {computed} from 'vue'
const props=defineProps<{seconds?:number;state:string}>()
const duration=computed(()=>{const total=Math.max(0,Math.floor(props.seconds??0));const days=Math.floor(total/86400),hours=Math.floor(total%86400/3600),minutes=Math.floor(total%3600/60);return [days?`${days}d`:'',hours?`${hours}h`:'',minutes?`${minutes}m`:'',`${total%60}s`].filter(Boolean).join(' ')})
</script>
<template><p v-if="seconds!==undefined&&['down','paused','unknown'].includes(state)" class="observed-outage" title="Through the last recorded observation. Paused time is excluded; this is not a wall-clock timer."><span>Observed outage</span><strong>{{duration}}</strong><small v-if="state==='paused'">Paused · duration held</small><small v-else-if="state==='unknown'">Awaiting fresh checks</small></p></template>
<style scoped>
.observed-outage{display:flex;gap:6px 12px;flex-wrap:wrap;justify-content:space-between;font-size:11px;color:var(--muted);margin:12px 0}.observed-outage strong{color:var(--text);font-weight:600;font-variant-numeric:tabular-nums}.observed-outage small{flex-basis:100%;font-size:10px}
</style>
