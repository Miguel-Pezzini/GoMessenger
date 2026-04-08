<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { Paperclip, Send, Smile } from 'lucide-vue-next';

const props = withDefaults(
  defineProps<{
    disabled?: boolean;
    placeholder?: string;
  }>(),
  {
    disabled: false,
    placeholder: 'Type a message',
  }
);

const emit = defineEmits<{
  sendMessage: [text: string];
  typingStarted: [];
  typingStopped: [];
}>();

const message = ref('');
const textareaRef = ref<HTMLTextAreaElement | null>(null);
const isTyping = ref(false);

let typingStopTimer: number | null = null;
let lastTypingSignalAt = 0;

const resizeTextarea = () => {
  if (!textareaRef.value) {
    return;
  }

  textareaRef.value.style.height = '0px';
  textareaRef.value.style.height = `${Math.min(textareaRef.value.scrollHeight, 160)}px`;
};

const clearTypingTimer = () => {
  if (typingStopTimer !== null) {
    window.clearTimeout(typingStopTimer);
    typingStopTimer = null;
  }
};

const emitTypingStopped = () => {
  clearTypingTimer();

  if (!isTyping.value) {
    return;
  }

  isTyping.value = false;
  lastTypingSignalAt = 0;
  emit('typingStopped');
};

const scheduleTypingStop = () => {
  clearTypingTimer();
  typingStopTimer = window.setTimeout(() => {
    emitTypingStopped();
  }, 1200);
};

const emitTypingStarted = () => {
  const now = Date.now();
  if (!isTyping.value || now - lastTypingSignalAt >= 2000) {
    emit('typingStarted');
    lastTypingSignalAt = now;
    isTyping.value = true;
  }

  scheduleTypingStop();
};

const handleSend = () => {
  const trimmed = message.value.trim();
  if (!props.disabled && trimmed) {
    emit('sendMessage', trimmed);
    message.value = '';
    emitTypingStopped();

    nextTick(() => {
      resizeTextarea();
    });
  }
};

const handleKeyDown = (event: KeyboardEvent) => {
  if (props.disabled) {
    return;
  }

  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault();
    handleSend();
  }
};

watch(message, async (value) => {
  await nextTick();
  resizeTextarea();

  if (props.disabled) {
    emitTypingStopped();
    return;
  }

  if (!value.trim()) {
    emitTypingStopped();
    return;
  }

  emitTypingStarted();
});

watch(
  () => props.disabled,
  (disabled) => {
    if (disabled) {
      emitTypingStopped();
    }
  }
);

onBeforeUnmount(() => {
  emitTypingStopped();
});
</script>

<template>
  <div class="border-t border-white/70 bg-white/85 p-4 shadow-lg backdrop-blur-sm">
    <div class="flex items-end gap-2">
      <button class="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-medium transition-colors hover:bg-indigo-50">
        <Paperclip :size="20" class="text-indigo-600" />
      </button>

      <button class="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-medium transition-colors hover:bg-indigo-50">
        <Smile :size="20" class="text-indigo-600" />
      </button>

      <textarea
        ref="textareaRef"
        v-model="message"
        :disabled="props.disabled"
        :placeholder="props.placeholder"
        class="min-h-[44px] max-h-40 flex-1 resize-none rounded-3xl border border-indigo-200 bg-white px-4 py-3 text-sm text-slate-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-400 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400"
        rows="1"
        @keydown="handleKeyDown"
      />

      <button
        :disabled="props.disabled || !message.trim()"
        class="inline-flex h-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-r from-indigo-600 to-purple-600 px-4 text-sm font-medium text-white shadow-md transition-colors hover:from-indigo-700 hover:to-purple-700 disabled:pointer-events-none disabled:opacity-50"
        @click="handleSend"
      >
        <Send :size="20" />
      </button>
    </div>
    <p class="mt-2 ml-20 text-xs text-indigo-400">
      Enter para enviar • Shift + Enter para quebrar linha
    </p>
  </div>
</template>
