<script setup lang="ts">
import {computed} from 'vue'
import {diagnosticSummary} from '../diagnostics'
const props=withDefaults(defineProps<{message:string;prefix?:string}>(),{prefix:''})
const summary=computed(()=>diagnosticSummary(props.message))
</script>
<template>
 <div class="check-diagnostic">
  <p class="diagnostic" :title="prefix+summary">{{prefix}}{{summary}}</p>
  <details v-if="summary!==message" class="diagnostic-details"><summary>Technical details</summary><p>{{message}}</p></details>
 </div>
</template>
<style scoped>
.diagnostic-details{margin:0 0 14px;padding-top:8px}.diagnostic-details summary{font-size:11px}.diagnostic-details p{white-space:pre-wrap;overflow-wrap:anywhere;user-select:text;font-size:11px}.diagnostic{overflow-wrap:anywhere;white-space:normal}
</style>
