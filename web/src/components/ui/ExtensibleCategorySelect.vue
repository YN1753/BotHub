<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAppStore } from '../../stores/useAppStore'
import type { AssetCategoryOption } from '../../types/bothub'
import {
  ASSET_CATEGORY_ICONS,
  SUGGESTED_ASSET_CATEGORIES,
  mergeExtensibleCategories,
  normalizeCategoryId,
  type AssetCategoryIconId,
} from '../../utils/assetCategories'
import AssetCategoryIcon from './AssetCategoryIcon.vue'

const props = defineProps<{
  modelValue: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const store = useAppStore()
const showCreator = ref(false)
const newLabel = ref('')
const newIcon = ref<AssetCategoryIconId>('tag')
const saving = ref(false)

const categories = computed(() =>
  mergeExtensibleCategories(store.state.assetCategories, store.state.assets)
)

const unusedSuggestions = computed(() => {
  const existingIds = new Set(categories.value.map((c) => c.id.toLowerCase()))
  return SUGGESTED_ASSET_CATEGORIES.filter((s) => !existingIds.has(s.id.toLowerCase()))
})

const customIdSet = computed(
  () => new Set(store.state.assetCategories.map((c) => c.id.toLowerCase()))
)

function selectCategory(id: string) {
  emit('update:modelValue', id)
}

async function addCustomCategory() {
  const label = newLabel.value.trim()
  if (!label || saving.value) return
  const id = normalizeCategoryId(label)
  saving.value = true
  try {
    const current = [...store.state.assetCategories]
    if (!current.some((c) => c.id.toLowerCase() === id.toLowerCase())) {
      current.push({ id, label, icon: newIcon.value })
      await store.saveAssetCategories(current)
    }
    emit('update:modelValue', id)
    newLabel.value = ''
    showCreator.value = false
  } catch (err: any) {
    store.notify(err?.message || '保存分类失败')
  } finally {
    saving.value = false
  }
}

async function addSuggestedCategory(item: AssetCategoryOption) {
  if (saving.value) return
  saving.value = true
  try {
    const current = [...store.state.assetCategories]
    if (!current.some((c) => c.id.toLowerCase() === item.id.toLowerCase())) {
      current.push({ id: item.id, label: item.label, icon: item.icon || 'tag' })
      await store.saveAssetCategories(current)
    }
    emit('update:modelValue', item.id)
  } catch (err: any) {
    store.notify(err?.message || '添加分类失败')
  } finally {
    saving.value = false
  }
}

async function removeCustomCategory(item: AssetCategoryOption) {
  if (saving.value) return
  saving.value = true
  try {
    const next = store.state.assetCategories.filter(
      (c) => c.id.toLowerCase() !== item.id.toLowerCase()
    )
    await store.saveAssetCategories(next)
    if (props.modelValue.toLowerCase() === item.id.toLowerCase()) {
      emit('update:modelValue', next[0]?.id || 'general')
    }
  } catch (err: any) {
    store.notify(err?.message || '移除分类失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-2">
    <div class="flex items-center justify-between gap-2">
      <span class="text-xs text-slate-300">资产类别（自由扩展，可随时增删）</span>
      <button
        type="button"
        @click="showCreator = !showCreator"
        class="px-2.5 py-0.5 rounded-full border border-[#f2b880]/40 bg-[#f2b880]/10 hover:bg-[#f2b880]/20 text-[11px] text-[#f2b880] cursor-pointer transition-colors"
      >
        {{ showCreator ? '收起管理' : '+ 自定义 / 扩展类型' }}
      </button>
    </div>

    <!-- Active Categories Chips -->
    <div v-if="categories.length > 0" class="flex flex-wrap gap-1.5">
      <div
        v-for="cat in categories"
        :key="cat.id"
        class="inline-flex items-center"
      >
        <button
          type="button"
          @click="selectCategory(cat.id)"
          :class="[
            'inline-flex items-center gap-1.5 min-h-[34px] px-3 py-1 rounded-xl border text-xs transition-all cursor-pointer',
            modelValue === cat.id
              ? 'btn-tactile-gold font-semibold'
              : 'btn-tactile text-slate-300 hover:text-white'
          ]"
        >
          <AssetCategoryIcon :icon="cat.icon" :size="13" />
          <span>{{ cat.label }}</span>
          <span
            v-if="showCreator && customIdSet.has(cat.id.toLowerCase())"
            role="button"
            tabindex="0"
            title="从常用列表移除此分类"
            aria-label="从常用列表移除此分类"
            @click.stop="removeCustomCategory(cat)"
            @keydown.enter.stop.prevent="removeCustomCategory(cat)"
            class="ml-0.5 text-[11px] opacity-65 hover:opacity-100 hover:text-rose-300"
          >
            ×
          </span>
        </button>
      </div>
    </div>
    <div
      v-else
      class="rounded-xl border border-dashed border-[#fff8ee]/14 px-3 py-2 text-[11.5px] text-slate-400"
    >
      当前暂无自定义分类（默认归入「通用资产」）；点击右上角「+ 自定义 / 扩展类型」可自由添加你的专属分类。
    </div>

    <!-- Expandable Creator + Optional Quick Suggestions -->
    <div
      v-if="showCreator"
      class="rounded-xl border border-[#f2b880]/30 bg-[#050811]/70 p-2.5 flex flex-col gap-2.5"
    >
      <div class="flex items-center gap-2 flex-wrap sm:flex-nowrap">
        <input
          v-model="newLabel"
          type="text"
          placeholder="输入新类型名称（如：保险、License、证件、软件...）"
          @keydown.enter.prevent="addCustomCategory"
          class="flex-1 min-w-[160px] min-h-[36px] rounded-xl border border-[#fff8ee]/14 bg-[#050811] px-2.5 py-1 text-xs text-white focus:border-[#f2b880]/60"
        />
        <button
          type="button"
          :disabled="!newLabel.trim() || saving"
          @click="addCustomCategory"
          class="min-h-[36px] px-3.5 rounded-xl bg-[#f2b880] text-[#070a13] font-semibold text-xs cursor-pointer disabled:opacity-45 shrink-0"
        >
          保存类型
        </button>
      </div>

      <div class="flex items-center gap-1.5 flex-wrap">
        <span class="text-[11px] text-slate-400">图标:</span>
        <button
          v-for="ic in ASSET_CATEGORY_ICONS"
          :key="ic.id"
          type="button"
          :title="ic.label"
          @click="newIcon = ic.id"
          :class="[
            'inline-flex items-center gap-1 px-2 py-1 rounded-lg border text-[11px] cursor-pointer transition-colors',
            newIcon === ic.id
              ? 'border-[#f2b880]/60 bg-[#f2b880]/16 text-[#f2b880]'
              : 'border-[#fff8ee]/10 text-slate-400 hover:text-white'
          ]"
        >
          <AssetCategoryIcon :icon="ic.id" :size="12" />
          <span>{{ ic.label }}</span>
        </button>
      </div>

      <div v-if="unusedSuggestions.length > 0" class="pt-1.5 border-t border-[#fff8ee]/10 flex flex-col gap-1.5">
        <span class="text-[11px] text-slate-400">可选快捷添加（按需点选纳入，不强制内置）:</span>
        <div class="flex flex-wrap gap-1.5">
          <button
            v-for="sug in unusedSuggestions"
            :key="sug.id"
            type="button"
            @click="addSuggestedCategory(sug)"
            class="inline-flex items-center gap-1 px-2 py-0.5 rounded-lg border border-[#fff8ee]/12 text-[11px] text-slate-300 hover:border-[#f2b880]/45 hover:text-[#f2b880] cursor-pointer"
          >
            <AssetCategoryIcon :icon="sug.icon" :size="11" />
            <span>+ {{ sug.label }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
