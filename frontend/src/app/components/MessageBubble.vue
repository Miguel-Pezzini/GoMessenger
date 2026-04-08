<script setup lang="ts">
import { Check, CheckCheck } from 'lucide-vue-next';

import { formatMessageTime } from '../chat/format.ts';
import type { ConversationMessage } from '../chat/types.ts';

interface Props {
  message: ConversationMessage;
}

defineProps<Props>();
</script>

<template>
  <div :class="['mb-3 flex', message.isMine ? 'justify-end' : 'justify-start']">
    <div :class="['max-w-[85%] md:max-w-[70%]', message.isMine ? 'items-end' : 'items-start']">
      <div
        :class="[
          'rounded-3xl px-4 py-3 shadow-sm',
          message.isMine
            ? 'rounded-br-sm bg-gradient-to-br from-indigo-600 to-violet-600 text-white'
            : 'rounded-bl-sm border border-white/90 bg-white/92 text-slate-900',
          message.isOptimistic ? 'opacity-90' : '',
        ]"
      >
        <p class="whitespace-pre-wrap break-words text-sm leading-6">{{ message.content }}</p>

        <div
          :class="[
            'mt-2 flex items-center gap-1 text-[11px]',
            message.isMine ? 'justify-end text-indigo-100/90' : 'justify-end text-slate-400',
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
</template>
