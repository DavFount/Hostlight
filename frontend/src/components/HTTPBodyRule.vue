<script setup lang="ts">
defineProps<{mode?:string;expected?:string;pointer?:string;valueType?:string}>()
const emit=defineEmits<{'update:mode':[string];'update:expected':[string];'update:pointer':[string];'update:valueType':[string]}>()
const field=(event:Event)=>(event.target as HTMLInputElement).value
function typeChanged(event:Event){const value=field(event);emit('update:valueType',value);if(value==='boolean')emit('update:expected','true');if(value==='null')emit('update:expected','')}
</script>
<template>
 <section class="http-body-rule" aria-label="HTTP response body rule">
  <label>Additional response check<select :value="mode||''" @change="emit('update:mode',field($event))"><option value="">Status only</option><option value="contains">Response contains text</option><option value="json">JSON field equals</option></select></label>
  <template v-if="mode==='contains'"><label>Required response text<input :value="expected||''" maxlength="4096" required placeholder="e.g. service ready" @input="emit('update:expected',field($event))"></label><p class="check-help">Case-sensitive substring match anywhere in the response. Both status and text must match. Larger words can also match: healthy appears in unhealthy. Use JSON equality for an exact health value.</p></template>
  <template v-if="mode==='json'">
   <label>JSON field path<input :value="pointer||''" maxlength="1024" placeholder="/status" @input="emit('update:pointer',field($event))"></label>
   <p class="check-help">Use /status for {"status":"healthy"}, /checks/0/healthy for an array entry, or leave blank to check the whole JSON value. In field names, write ~1 for / and ~0 for ~. This is a JSON Pointer, not a script.</p>
   <label>Expected value type<select :value="valueType||'string'" @change="typeChanged"><option value="string">Text</option><option value="number">Number</option><option value="boolean">True / false</option><option value="null">Null (empty value)</option></select></label>
   <label v-if="valueType==='boolean'">Expected value<select :value="expected||'true'" @change="emit('update:expected',field($event))"><option value="true">true</option><option value="false">false</option></select></label>
   <label v-else-if="valueType!=='null'">Expected value<input :value="expected||''" maxlength="4096" :required="valueType==='number'" :placeholder="valueType==='number'?'e.g. 0':'e.g. healthy (no quotation marks)'" @input="emit('update:expected',field($event))"></label>
   <p class="check-help">The field must exist and match its value and type. A missing field is not null. Arrays and objects cannot be expected values.</p>
  </template>
  <p v-if="mode" class="check-help">Checks inspect at most 1 MiB and fail if the body is larger, incomplete, or times out. Response content is not stored or shown. Expected values are saved and exported—do not enter secrets.</p>
 </section>
</template>
<style scoped>
.http-body-rule{margin-top:16px;padding:0 14px 8px;border:1px solid var(--border);border-radius:8px}.http-body-rule label{display:grid;gap:8px;font-size:12px;margin-top:14px}.http-body-rule p{font-size:12px;line-height:1.6;overflow-wrap:anywhere}
</style>
