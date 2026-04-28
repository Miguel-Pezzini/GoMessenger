<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';
import { Check, CheckCheck, Download, FileText } from 'lucide-vue-next';

import { API_BASE_URL } from '../chat/api.ts';
import { formatMessageTime } from '../chat/format.ts';
import type { ChatAttachment, ConversationMessage } from '../chat/types.ts';
import MediaDialog from './MediaDialog.vue';

interface Props {
  message: ConversationMessage;
  attachmentToken: string;
}

const props = defineProps<Props>();
const objectUrls = ref<Record<string, string>>({});

const isPreviewable = (attachment: ChatAttachment) => {
  return (
    attachment.kind === 'image' ||
    attachment.kind === 'video' ||
    attachment.kind === 'audio' ||
    attachment.content_type === 'application/pdf'
  );
};

const loadAttachmentBlob = async (attachment: ChatAttachment) => {
  if (!props.attachmentToken) {
    return null;
  }

  const response = await fetch(`${API_BASE_URL}${attachment.download_url}`, {
    headers: {
      Authorization: `Bearer ${props.attachmentToken}`,
    },
  });
  if (!response.ok) {
    return null;
  }
  return response.blob();
};

const refreshPreviewUrls = async () => {
  const nextIds = new Set(props.message.attachments.map((attachment) => attachment.id));
  for (const [id, url] of Object.entries(objectUrls.value)) {
    if (!nextIds.has(id)) {
      URL.revokeObjectURL(url);
      delete objectUrls.value[id];
    }
  }

  for (const attachment of props.message.attachments) {
    if (!isPreviewable(attachment) || objectUrls.value[attachment.id]) {
      continue;
    }

    const blob = await loadAttachmentBlob(attachment);
    if (blob) {
      objectUrls.value[attachment.id] = URL.createObjectURL(blob);
    }
  }
};

const handleDownload = async (attachment: ChatAttachment) => {
  const blob = await loadAttachmentBlob(attachment);
  if (!blob) {
    return;
  }

  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = attachment.filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
};

interface DialogState {
  src: string;
  kind: 'image' | 'video' | 'audio';
  filename: string;
}

const dialog = ref<DialogState | null>(null);

const openDialog = (attachment: ChatAttachment) => {
  const src = objectUrls.value[attachment.id];
  if (!src || !['image', 'video', 'audio'].includes(attachment.kind)) return;
  dialog.value = { src, kind: attachment.kind as 'image' | 'video' | 'audio', filename: attachment.filename };
};

const formatFileSize = (size: number) => {
  if (size >= 1024 * 1024) {
    return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  }
  if (size >= 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${size} B`;
};

watch(
  () => [props.attachmentToken, props.message.attachments.map((attachment) => attachment.id).join('|')] as const,
  () => {
    void refreshPreviewUrls();
  },
  { immediate: true }
);

onBeforeUnmount(() => {
  for (const url of Object.values(objectUrls.value)) {
    URL.revokeObjectURL(url);
  }
});
</script>

<template>
  <div :class="['mb-3 flex', message.isMine ? 'justify-end' : 'justify-start']">
    <div :class="['max-w-[85%] md:max-w-[70%]', message.isMine ? 'items-end' : 'items-start']">
      <div
        :class="[
          'rounded-3xl px-4 py-3 shadow-sm',
          message.isMine
            ? 'rounded-br-sm bg-gradient-to-br from-indigo-600 to-violet-600 text-white'
            : 'rounded-bl-sm border border-white/90 bg-white/92 text-slate-900 dark:border-slate-700/80 dark:bg-slate-800/90 dark:text-slate-100',
          message.isOptimistic ? 'opacity-90' : '',
        ]"
      >
        <p v-if="message.content" class="whitespace-pre-wrap break-words text-sm leading-6">{{ message.content }}</p>

        <div v-if="message.attachments.length" :class="['space-y-2', message.content ? 'mt-3' : '']">
          <div v-for="attachment in message.attachments" :key="attachment.id" class="overflow-hidden rounded-lg bg-black/5 dark:bg-white/5">
            <img
              v-if="attachment.kind === 'image' && objectUrls[attachment.id]"
              :src="objectUrls[attachment.id]"
              :alt="attachment.filename"
              class="max-h-80 w-full cursor-zoom-in object-contain"
              @click="openDialog(attachment)"
            />
            <div
              v-else-if="attachment.kind === 'video' && objectUrls[attachment.id]"
              class="relative"
            >
              <video
                :src="objectUrls[attachment.id]"
                class="max-h-80 w-full"
                controls
              />
              <button
                class="absolute right-2 top-2 rounded-full bg-black/50 p-1 text-white transition-colors hover:bg-black/70"
                type="button"
                aria-label="Expandir vídeo"
                @click="openDialog(attachment)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/><line x1="21" y1="3" x2="14" y2="10"/><line x1="3" y1="21" x2="10" y2="14"/></svg>
              </button>
            </div>
            <div
              v-else-if="attachment.kind === 'audio' && objectUrls[attachment.id]"
              class="flex items-center gap-2 px-1 py-1"
            >
              <audio
                :src="objectUrls[attachment.id]"
                class="flex-1"
                controls
              />
              <button
                class="shrink-0 rounded-full p-1 opacity-60 transition-opacity hover:opacity-100"
                type="button"
                aria-label="Expandir áudio"
                @click="openDialog(attachment)"
              >
                <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 3 21 3 21 9"/><polyline points="9 21 3 21 3 15"/><line x1="21" y1="3" x2="14" y2="10"/><line x1="3" y1="21" x2="10" y2="14"/></svg>
              </button>
            </div>
            <iframe
              v-else-if="attachment.content_type === 'application/pdf' && objectUrls[attachment.id]"
              :src="objectUrls[attachment.id]"
              :title="attachment.filename"
              class="h-64 w-72 max-w-full border-0 bg-white"
            />

            <button
              class="flex w-full cursor-pointer items-center gap-3 px-3 py-2 text-left text-xs transition-colors hover:bg-black/5 dark:hover:bg-white/5"
              type="button"
              @click="handleDownload(attachment)"
            >
              <FileText :size="18" class="shrink-0" />
              <span class="min-w-0 flex-1">
                <span class="block truncate font-medium">{{ attachment.filename }}</span>
                <span :class="message.isMine ? 'text-indigo-100/80' : 'text-slate-500'">{{ formatFileSize(attachment.size) }}</span>
              </span>
              <Download :size="16" class="shrink-0" />
            </button>
          </div>
        </div>

        <div
          :class="[
            'mt-2 flex items-center gap-1 text-[11px]',
            message.isMine ? 'justify-end text-indigo-100/90' : 'justify-end text-slate-400 dark:text-slate-500',
          ]"
        >
          <span>{{ formatMessageTime(message.timestamp) }}</span>
          <span v-if="message.isMine" :title="message.viewedStatus" class="inline-flex items-center">
            <Check v-if="message.viewedStatus === 'sent'" :size="14" />
            <CheckCheck
              v-else
              :size="14"
              :class="message.viewedStatus === 'seen' ? 'text-cyan-200' : 'text-indigo-100/90'"
            />
          </span>
        </div>
      </div>
    </div>
  </div>

  <MediaDialog
    v-if="dialog"
    :src="dialog.src"
    :kind="dialog.kind"
    :filename="dialog.filename"
    @close="dialog = null"
  />
</template>
