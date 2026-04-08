<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';

import type { ConversationMessage } from '../chat/types.ts';
import MessageBubble from './MessageBubble.vue';

interface Props {
  messages: ConversationMessage[];
  contactId: string;
  hasMore: boolean;
  isLoadingMore: boolean;
  isPeerTyping?: boolean;
  typingLabel?: string;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  loadMore: [];
  messagesRendered: [];
}>();

const scrollRef = ref<HTMLDivElement | null>(null);
const previousContactId = ref(props.contactId);
const previousMessageCount = ref(props.messages.length);
const pendingPrependAdjustment = ref<{ scrollHeight: number; scrollTop: number } | null>(null);

const scrollToBottom = () => {
  if (scrollRef.value) {
    scrollRef.value.scrollTop = scrollRef.value.scrollHeight;
  }
};

const handleScroll = () => {
  const container = scrollRef.value;
  if (!container || props.isLoadingMore || !props.hasMore) {
    return;
  }

  if (container.scrollTop > 80) {
    return;
  }

  pendingPrependAdjustment.value = {
    scrollHeight: container.scrollHeight,
    scrollTop: container.scrollTop,
  };
  emit('loadMore');
};

watch(
  () => [props.contactId, props.messages.length, props.isLoadingMore] as const,
  async ([contactId, messageCount, isLoadingMore]) => {
    const container = scrollRef.value;
    const contactChanged = contactId !== previousContactId.value;
    const messageCountChanged = messageCount !== previousMessageCount.value;

    await nextTick();

    if (!container) {
      previousContactId.value = contactId;
      previousMessageCount.value = messageCount;
      emit('messagesRendered');
      return;
    }

    if (contactChanged) {
      scrollToBottom();
    } else if (pendingPrependAdjustment.value && messageCountChanged) {
      const { scrollHeight, scrollTop } = pendingPrependAdjustment.value;
      container.scrollTop = scrollTop + (container.scrollHeight - scrollHeight);
      pendingPrependAdjustment.value = null;
    } else if (pendingPrependAdjustment.value && !isLoadingMore) {
      pendingPrependAdjustment.value = null;
    } else if (messageCountChanged) {
      scrollToBottom();
    }

    previousContactId.value = contactId;
    previousMessageCount.value = messageCount;
    emit('messagesRendered');
  },
  { immediate: true }
);
</script>

<template>
  <div
    ref="scrollRef"
    class="flex-1 overflow-y-auto bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.8),rgba(224,231,255,0.22)_40%,rgba(240,249,255,0.35)_100%)]"
    @scroll="handleScroll"
  >
    <div class="flex min-h-full flex-col p-4 md:p-6">
      <div v-if="isLoadingMore" class="mb-4 text-center text-xs font-medium uppercase tracking-[0.18em] text-indigo-500">
        Loading older messages
      </div>

      <MessageBubble v-for="message in messages" :key="message.id" :message="message" />

      <div v-if="isPeerTyping" class="mb-4 flex justify-start">
        <div class="rounded-3xl rounded-bl-sm border border-white/80 bg-white/90 px-4 py-3 shadow-sm">
          <div class="flex items-center gap-2 text-sm font-medium text-emerald-700">
            <span>{{ typingLabel || 'typing...' }}</span>
            <span class="flex items-center gap-1">
              <span class="h-1.5 w-1.5 animate-bounce rounded-full bg-emerald-500" />
              <span class="h-1.5 w-1.5 animate-bounce rounded-full bg-emerald-500 [animation-delay:0.15s]" />
              <span class="h-1.5 w-1.5 animate-bounce rounded-full bg-emerald-500 [animation-delay:0.3s]" />
            </span>
          </div>
        </div>
      </div>

      <div v-if="!messages.length && !isLoadingMore" class="my-auto text-center text-sm text-slate-500">
        No messages yet. Start the conversation.
      </div>
    </div>
  </div>
</template>
