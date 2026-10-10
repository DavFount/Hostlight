<script setup lang="ts">
import {computed} from 'vue'
const props=defineProps<{expires?:string;minimumDays?:number;now:number}>()
const remaining=computed(()=>props.expires?new Date(props.expires).getTime()-props.now:NaN)
const valid=computed(()=>Number.isFinite(remaining.value))
const warning=computed(()=>remaining.value<Math.max(0,props.minimumDays||0)*86400000)
const days=computed(()=>Math.max(0,Math.floor(remaining.value/86400000)))
</script>
<template><p v-if="valid" class="certificate-expiry" :class="{warning}" :title="`Last verified server certificate expiry: ${new Date(expires!).toLocaleString()}`">{{remaining<=0?'Certificate expiry reached':`Certificate: ${days} ${days===1?'day':'days'} remaining`}}<small>Expires {{new Date(expires!).toLocaleString()}}</small></p></template>
<style scoped>
.certificate-expiry{font-size:11px;padding:7px 9px;border:1px solid var(--border);border-radius:6px;line-height:1.5}.certificate-expiry small{display:block;font-size:10px}.certificate-expiry.warning{color:#ffd580;background:#5a3c12;border-color:#c38a29}.certificate-expiry.warning small{color:inherit}[data-theme="light"] .certificate-expiry.warning{color:#694300;background:#fff0c2;border-color:#b47d18}
</style>
