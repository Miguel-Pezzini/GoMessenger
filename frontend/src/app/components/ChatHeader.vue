<script setup lang="ts">
import { ArrowLeft, MoreVertical, Phone, Video } from 'lucide-vue-next';

interface Props {
  name: string;
  avatar: string;
  status: string;
  online: boolean;
  isTyping?: boolean;
  isInCurrentChat?: boolean;
  showBackButton?: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  back: [];
}>();
</script>

<template>
  <div class="flex h-18 items-center justify-between border-b border-white/70 bg-white/80 px-4 shadow-sm backdrop-blur-sm md:px-6">
    <div class="flex min-w-0 items-center gap-3">
      <button
        v-if="showBackButton"
        type="button"
        class="inline-flex h-10 w-10 items-center justify-center rounded-full text-slate-500 transition hover:bg-slate-100 hover:text-slate-800 md:hidden"
        @click="emit('back')"
      >
        <ArrowLeft :size="18" />
      </button>

      <div class="relative">
        <div class="flex size-11 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-indigo-200 via-white to-emerald-100 font-semibold text-slate-800 ring-2 ring-white">
          <img v-if="avatar" :src="avatar" :alt="name" class="h-full w-full object-cover" />
          <span v-else class="text-sm font-medium">{{ name.slice(0, 2).toUpperCase() }}</span>
        </div>
        <span v-if="online" class="absolute bottom-0 right-0 size-3 rounded-full border-2 border-white bg-emerald-500" />
      </div>

      <div class="min-w-0">
        <div class="flex min-w-0 items-center gap-2">
          <h2 class="truncate font-semibold text-slate-900">{{ name }}</h2>
          <span
            v-if="isInCurrentChat && !isTyping"
            class="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-[0.16em] text-emerald-700"
          >
            In chat
          </span>
        </div>
        <p :class="['truncate text-sm', isTyping ? 'font-medium text-emerald-600' : 'text-indigo-600']">
          {{ status }}
        </p>
      </div>
    </div>

    <div class="flex items-center gap-2">
      <button class="inline-flex h-10 w-10 items-center justify-center rounded-full text-sm font-medium transition-colors hover:bg-indigo-50">
        <Phone :size="20" class="text-indigo-600" />
      </button>
      <button class="inline-flex h-10 w-10 items-center justify-center rounded-full text-sm font-medium transition-colors hover:bg-indigo-50">
        <Video :size="20" class="text-indigo-600" />
      </button>
      <button class="inline-flex h-10 w-10 items-center justify-center rounded-full text-sm font-medium transition-colors hover:bg-indigo-50">
        <MoreVertical :size="20" class="text-indigo-600" />
      </button>
    </div>
  </div>
</template>
