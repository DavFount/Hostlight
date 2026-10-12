<script setup lang="ts">
import type {CheckStatisticsData} from '../checkStatistics'
defineProps<{statistics?:CheckStatisticsData}>()
function failurePercent(statistics:CheckStatisticsData){const percent=100*statistics.sessionFailed/statistics.sessionChecks;return percent>0&&percent<0.01?'<0.01':percent.toFixed(2)}
</script>
<template>
 <div class="check-statistics" title="Session totals and last 100 completed availability checks. Local setup errors are counted separately. Paused time adds no checks; this is not uptime or packet loss.">
  <template v-if="!statistics">Check statistics unavailable</template>
  <template v-else>
   <div class="session-statistics">
    <div class="statistics-heading"><strong>Session</strong><span>{{statistics.sessionChecks.toLocaleString()}} {{statistics.sessionChecks===1?'check':'checks'}}</span></div>
    <dl class="statistics-values"><div><dt>Passed</dt><dd>{{statistics.sessionPassed.toLocaleString()}}</dd></div><div><dt>Failed</dt><dd :class="{warning:statistics.sessionFailed>0}">{{statistics.sessionFailed.toLocaleString()}}</dd></div><div><dt>Failure rate</dt><dd>{{statistics.sessionChecks?failurePercent(statistics)+'%':'—'}}</dd></div></dl>
    <p v-if="!statistics.sessionChecks" class="statistics-empty">No completed availability checks yet</p>
   </div>
   <div class="recent-statistics" :title="`Last ${statistics.checks} ${statistics.checks===1?'check':'checks'} · this session`"><span>Recent</span><span v-if="statistics.checks">{{statistics.failed}} / {{statistics.checks}} failed · {{(100*statistics.failed/statistics.checks).toFixed(1)}}%</span><span v-else>No checks yet</span></div>
   <details class="statistics-details"><summary>Session details<span v-if="statistics.setupErrors" class="warning"> · {{statistics.setupErrors.toLocaleString()}} {{statistics.setupErrors===1?'setup error':'setup errors'}}</span></summary><div><p v-if="statistics.started">Since <time :datetime="statistics.started">{{new Date(statistics.started).toLocaleString()}}</time></p><p>{{statistics.setupErrors.toLocaleString()}} DNS / setup errors · excluded from availability counts and failure rate.</p><p>Session counts all completed availability checks. Recent shows the last 100. Pauses add no checks. Resets on edit or app restart.</p></div></details>
  </template>
 </div>
</template>
<style scoped>
.check-statistics{font-size:11px;margin:12px 0;overflow-wrap:anywhere;line-height:1.5;color:var(--muted)}.statistics-heading{display:flex;justify-content:space-between;gap:8px;align-items:baseline}.statistics-heading strong{font-weight:600;color:var(--text)}.statistics-values{display:grid;grid-template-columns:1fr 1fr 1.2fr;gap:8px;margin:9px 0 10px}.statistics-values>div{min-width:0}.statistics-values dt{font-size:10px;margin-bottom:3px}.statistics-values dd{margin:0;font-size:15px;font-variant-numeric:tabular-nums;color:var(--text);line-height:1.3}.statistics-values>div+div{border-left:1px solid var(--border);padding-left:8px}.statistics-values .warning,.warning{color:var(--warning,#c9a250)}.recent-statistics{display:flex;justify-content:space-between;gap:8px;flex-wrap:wrap;font-size:10px}.statistics-details{border:0;padding:0;margin:8px 0 0}.statistics-details summary{font-size:10px;color:var(--muted)}.statistics-details>div{border-left:2px solid var(--border);padding-left:8px;margin-top:8px}.statistics-details p,.statistics-empty{font-size:10px;margin:5px 0;line-height:1.5}:global(.compact) .statistics-values dd{font-size:13px}:global([data-theme="light"]) .warning{color:#805700}
</style>
