<script setup lang="ts">
defineProps<{statistics?:{checks:number;failed:number}}>()
</script>
<template>
 <p class="check-statistics" title="Last 100 completed availability checks in this session. Local setup errors and paused time add no checks. Resets after editing or restarting; this is not uptime or packet loss.">
  <template v-if="!statistics">Recent check statistics unavailable</template>
  <template v-else-if="!statistics.checks">No completed availability checks yet</template>
  <template v-else>{{statistics.failed}} / {{statistics.checks}} failed checks · {{(100*statistics.failed/statistics.checks).toFixed(1)}}%<small>Last {{statistics.checks}} {{statistics.checks===1?'check':'checks'}} · this session</small></template>
 </p>
</template>
<style scoped>
.check-statistics{font-size:11px;margin:10px 0;overflow-wrap:anywhere}.check-statistics small{display:block;margin-top:3px;font-size:10px}
</style>
