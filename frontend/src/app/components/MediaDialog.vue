<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue';
import { X } from 'lucide-vue-next';

interface Props {
  src: string;
  kind: 'image' | 'video' | 'audio';
  filename: string;
}

const props = defineProps<Props>();
const emit = defineEmits<{ close: [] }>();

const onKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape') emit('close');
};

onMounted(() => document.addEventListener('keydown', onKeydown));
onUnmounted(() => document.removeEventListener('keydown', onKeydown));
</script>

<template>
  <Teleport to="body">
    <div
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm"
      @click.self="emit('close')"
    >
      <div class="relative flex max-h-[90vh] max-w-[90vw] flex-col items-center">
        <button
          class="absolute -top-10 right-0 rounded-full p-1 text-white/70 transition-colors hover:text-white"
          type="button"
          @click="emit('close')"
        >
          <X :size="24" />
        </button>

        <img
          v-if="props.kind === 'image'"
          :src="props.src"
          :alt="props.filename"
          class="max-h-[85vh] max-w-[85vw] rounded-lg object-contain shadow-2xl"
        />
        <video
          v-else-if="props.kind === 'video'"
          :src="props.src"
          class="max-h-[85vh] max-w-[85vw] rounded-lg shadow-2xl"
          controls
          autoplay
        />
        <div
          v-else-if="props.kind === 'audio'"
          class="flex flex-col items-center gap-4 rounded-2xl bg-slate-900 px-8 py-6 shadow-2xl"
        >
          <span class="max-w-xs truncate text-sm text-slate-300">{{ props.filename }}</span>
          <audio :src="props.src" controls autoplay class="w-72" />
        </div>

        <span class="mt-3 max-w-sm truncate text-xs text-white/50">{{ props.filename }}</span>
      </div>
    </div>
  </Teleport>
</template>
