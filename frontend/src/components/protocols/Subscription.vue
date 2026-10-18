<template>
  <v-row>
    <v-col cols="12">
      <v-alert type="info" variant="tonal">{{ $t('subscriptionOutbound.help') }}</v-alert>
    </v-col>
    <v-col cols="12">
      <v-text-field v-model="data.url" :label="$t('subscriptionOutbound.url')" autocomplete="off" />
    </v-col>
    <v-col cols="12" sm="6">
      <v-text-field v-model="data.user_agent" :label="$t('subscriptionOutbound.userAgent')" />
    </v-col>
    <v-col cols="12" sm="6">
      <v-text-field v-model="data.update_interval" :label="$t('subscriptionOutbound.updateInterval')" :hint="$t('subscriptionOutbound.durationHint')" persistent-hint />
    </v-col>
    <v-col cols="12">
      <v-btn :loading="loading" :disabled="!data.url || loading" variant="tonal" @click="loadServers">{{ $t('subscriptionOutbound.load') }}</v-btn>
    </v-col>
    <v-col v-if="error || data.last_error" cols="12">
      <v-alert type="warning" variant="tonal" role="status">{{ error || data.last_error }}</v-alert>
    </v-col>
    <v-col cols="12">
      <v-autocomplete v-model="data.primary" :items="items" :label="$t('subscriptionOutbound.primary')" :disabled="loading" />
    </v-col>
    <v-col cols="12">
      <v-autocomplete v-model="data.backup" :items="items.filter(item => item.value !== data.primary)" :label="$t('subscriptionOutbound.backup')" :disabled="loading" clearable @update:model-value="data.backup = $event || ''" />
    </v-col>
    <template v-if="data.backup">
      <v-col cols="12" sm="6">
        <v-text-field v-model="data.failover.probe_target" :label="$t('subscriptionOutbound.probeTarget')" placeholder="https://www.gstatic.com/generate_204" />
      </v-col>
      <v-col cols="12" sm="6">
        <v-text-field v-model="data.failover.interval" :label="$t('subscriptionOutbound.probeInterval')" />
      </v-col>
      <v-col cols="12" sm="6">
        <v-text-field v-model.number="data.failover.hysteresis" type="number" min="1" :label="$t('subscriptionOutbound.hysteresis')" />
      </v-col>
    </template>
    <v-col cols="12"><div class="text-caption" role="status">{{ $t('subscriptionOutbound.refreshHint') }}</div></v-col>
  </v-row>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { Outbound } from '@/types/outbounds'
import HttpUtils from '@/plugins/httputil'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ data: Outbound }>()
const emit = defineEmits<{ validity: [valid: boolean] }>()
const { t } = useI18n()
const loading = ref(false)
const error = ref('')
const nodes = ref<Record<string, any>[]>([])
let generation = 0
watch(() => props.data, data => {
  generation++
  loading.value = false
  error.value = ''
  nodes.value = Array.isArray(data.nodes) ? data.nodes : []
  data.failover ??= { enabled: true, interval: '30s', hysteresis: 2 }
}, { immediate: true })
watch(() => [props.data, props.data.url, props.data.user_agent], (value, previous) => {
  if (!previous || value[0] !== previous[0]) return
  generation++
  loading.value = false
  nodes.value = []
  props.data.primary = ''
  props.data.backup = ''
  props.data.nodes = []
  error.value = ''
}, { flush: 'sync' })
const items = computed(() => {
  const counts = new Map<string, number>()
  for (const node of nodes.value) if (typeof node.tag === 'string') counts.set(node.tag, (counts.get(node.tag) ?? 0) + 1)
  return nodes.value.filter(node => node.tag && counts.get(node.tag) === 1).map(node => ({
    title: `${node.tag} · ${node.type} · ${node.server ?? ''}:${node.server_port ?? ''}`,
    value: node.tag as string,
  }))
})
watch(() => [loading.value, props.data.url, props.data.primary, props.data.backup, items.value], () => {
  const has = (name: string) => items.value.some(item => item.value === name)
  emit('validity', !loading.value && !!props.data.url && has(props.data.primary) && (!props.data.backup || (props.data.backup !== props.data.primary && has(props.data.backup))))
}, { immediate: true, deep: true })
async function loadServers() {
  const request = ++generation
  const data = props.data
  loading.value = true
  error.value = ''
  try {
    const response = await HttpUtils.post('api/subConvert', { link: data.url, user_agent: data.user_agent || 'sing-box' })
    if (generation !== request || props.data !== data) return
    if (!response.success || !Array.isArray(response.obj)) { error.value = t('subscriptionOutbound.loadError'); return }
    nodes.value = response.obj
    data.nodes = response.obj
    if (!items.value.some(item => item.value === data.primary)) data.primary = ''
    if (!items.value.some(item => item.value === data.backup)) data.backup = ''
    if (!items.value.length) error.value = t('subscriptionOutbound.namesError')
  } finally {
    if (generation === request) loading.value = false
  }
}
onBeforeUnmount(() => { generation++ })
</script>
