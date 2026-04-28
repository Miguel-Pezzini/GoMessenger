<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { FileText, Paperclip, Send, Smile, X } from 'lucide-vue-next';

import type { ChatAttachment } from '../chat/types.ts';

const props = withDefaults(
  defineProps<{
    disabled?: boolean;
    placeholder?: string;
    attachments?: ChatAttachment[];
    isUploadingAttachments?: boolean;
  }>(),
  {
    disabled: false,
    placeholder: 'Type a message',
    attachments: () => [],
    isUploadingAttachments: false,
  }
);

const emit = defineEmits<{
  sendMessage: [text: string];
  attachFiles: [files: File[]];
  removeAttachment: [attachmentId: string];
  typingStarted: [];
  typingStopped: [];
}>();

const message = ref('');
const textareaRef = ref<HTMLTextAreaElement | null>(null);
const fileInputRef = ref<HTMLInputElement | null>(null);
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
  if (!props.disabled && !props.isUploadingAttachments && (trimmed || props.attachments.length > 0)) {
    emit('sendMessage', trimmed);
    message.value = '';
    emitTypingStopped();

    nextTick(() => {
      resizeTextarea();
    });
  }
};

const openFilePicker = () => {
  if (props.disabled || props.isUploadingAttachments || props.attachments.length >= 10) {
    return;
  }
  fileInputRef.value?.click();
};

const handleFileChange = (event: Event) => {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  if (files.length > 0) {
    emit('attachFiles', files);
  }
  input.value = '';
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
  <div class="border-t border-white/70 bg-white/85 p-4 shadow-lg backdrop-blur-sm dark:border-slate-700/60 dark:bg-slate-900/85">
    <div v-if="props.attachments.length || props.isUploadingAttachments" class="mb-3 ml-0 flex flex-wrap gap-2 md:ml-20">
      <div
        v-for="attachment in props.attachments"
        :key="attachment.id"
        class="flex max-w-full items-center gap-2 rounded-lg border border-indigo-100 bg-white px-3 py-2 text-xs text-slate-700 shadow-sm dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
      >
        <FileText :size="14" class="shrink-0 text-indigo-500 dark:text-indigo-400" />
        <span class="max-w-48 truncate">{{ attachment.filename }}</span>
        <button
          class="inline-flex size-5 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-700 dark:hover:text-slate-300"
          type="button"
          @click="emit('removeAttachment', attachment.id)"
        >
          <X :size="12" />
        </button>
      </div>
      <span v-if="props.isUploadingAttachments" class="rounded-lg border border-indigo-100 bg-indigo-50 px-3 py-2 text-xs font-medium text-indigo-600 dark:border-indigo-800 dark:bg-indigo-900/40 dark:text-indigo-400">
        Uploading...
      </span>
    </div>
    <div class="flex items-end gap-2">
      <input ref="fileInputRef" class="hidden" multiple type="file" @change="handleFileChange" />
      <button
        :disabled="props.disabled || props.isUploadingAttachments || props.attachments.length >= 10"
        class="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-medium transition-colors hover:bg-indigo-50 disabled:pointer-events-none disabled:opacity-50 dark:hover:bg-indigo-900/40"
        type="button"
        @click="openFilePicker"
      >
        <Paperclip :size="20" class="text-indigo-600 dark:text-indigo-400" />
      </button>

      <button class="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-medium transition-colors hover:bg-indigo-50 dark:hover:bg-indigo-900/40">
        <Smile :size="20" class="text-indigo-600 dark:text-indigo-400" />
      </button>

      <textarea
        ref="textareaRef"
        v-model="message"
        :disabled="props.disabled"
        :placeholder="props.placeholder"
        class="min-h-[44px] max-h-40 flex-1 resize-none rounded-3xl border border-indigo-200 bg-white px-4 py-3 text-sm text-slate-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo-400 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400 dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:placeholder-slate-500 dark:focus-visible:ring-indigo-600 dark:disabled:bg-slate-700 dark:disabled:text-slate-500"
        rows="1"
        @keydown="handleKeyDown"
      />

      <button
        :disabled="props.disabled || props.isUploadingAttachments || (!message.trim() && !props.attachments.length)"
        class="inline-flex h-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-r from-indigo-600 to-purple-600 px-4 text-sm font-medium text-white shadow-md transition-colors hover:from-indigo-700 hover:to-purple-700 disabled:pointer-events-none disabled:opacity-50"
        type="button"
        @click="handleSend"
      >
        <Send :size="20" />
      </button>
    </div>
    <p class="mt-2 ml-20 text-xs text-indigo-400 dark:text-indigo-500">
      Enter para enviar • Shift + Enter para quebrar linha
    </p>
  </div>
</template>
