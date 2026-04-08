<script setup lang="ts">
import { computed, ref } from 'vue';
import { Check, Search, Trash2, UserPlus, UserRoundCheck, X } from 'lucide-vue-next';

import type { ContactListItem, FriendRequest } from '../chat/types.ts';

interface Props {
  contacts: ContactListItem[];
  pendingRequests: FriendRequest[];
  selectedContactId: string | null;
  isLoading: boolean;
  currentUserId: string;
  actionError: string;
  actionSuccess: string;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  selectContact: [id: string];
  sendFriendRequest: [receiverId: string];
  acceptRequest: [requestId: string];
  declineRequest: [requestId: string];
  removeFriend: [friendId: string];
}>();

const search = ref('');
const receiverId = ref('');

const requestDateFormatter = new Intl.DateTimeFormat('en-US', {
  month: 'short',
  day: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
});

const filteredContacts = computed(() => {
  const term = search.value.trim().toLowerCase();
  if (!term) {
    return props.contacts;
  }

  return props.contacts.filter((contact) => {
    return contact.name.toLowerCase().includes(term) || contact.id.toLowerCase().includes(term);
  });
});

const formatRequestTimestamp = (value: string) => {
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) {
    return value;
  }

  return requestDateFormatter.format(new Date(parsed));
};

const submitFriendRequest = () => {
  const value = receiverId.value.trim();
  if (!value) {
    return;
  }

  emit('sendFriendRequest', value);
  receiverId.value = '';
};
</script>

<template>
  <aside class="z-10 flex h-screen w-full max-w-[26rem] flex-col border-r border-white/70 bg-white/80 shadow-xl backdrop-blur-sm md:w-[26rem]">
    <div class="border-b border-white/60 bg-white/70 p-4 backdrop-blur-sm">
      <div class="mb-3 flex items-center gap-2 rounded-2xl bg-indigo-950 px-3 py-2 text-sm font-medium text-indigo-50">
        <UserRoundCheck :size="16" />
        Friends
      </div>

      <form class="mb-3 space-y-2" @submit.prevent="submitFriendRequest">
        <label class="block text-xs font-semibold uppercase tracking-[0.18em] text-indigo-500">
          Add Friend By User ID
        </label>
        <div class="flex gap-2">
          <input
            v-model="receiverId"
            type="text"
            placeholder="Paste the user id"
            class="min-w-0 flex-1 rounded-xl border border-indigo-200 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
          />
          <button
            type="submit"
            class="inline-flex h-10 shrink-0 items-center justify-center rounded-xl bg-indigo-600 px-4 text-sm font-semibold text-white transition hover:bg-indigo-700"
          >
            <UserPlus :size="16" class="mr-2" />
            Send
          </button>
        </div>
      </form>

      <div class="relative">
        <Search :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-indigo-400" />
        <input
          v-model="search"
          type="text"
          placeholder="Search friends"
          class="flex h-10 w-full rounded-xl border border-indigo-200 bg-white px-3 py-2 pl-9 text-sm text-gray-900 outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
        />
      </div>

      <p v-if="actionError" class="mt-3 rounded-xl bg-red-50 px-3 py-2 text-sm text-red-700">
        {{ actionError }}
      </p>
      <p v-if="actionSuccess" class="mt-3 rounded-xl bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
        {{ actionSuccess }}
      </p>
      <p v-if="currentUserId" class="mt-3 text-xs text-slate-500">
        Logged in as <span class="font-semibold text-slate-700">{{ currentUserId }}</span>
      </p>
    </div>

    <div class="border-b border-white/60 bg-white/40 px-4 py-3">
      <div class="mb-2 flex items-center justify-between">
        <h2 class="text-sm font-semibold text-slate-800">Pending requests</h2>
        <span class="rounded-full bg-indigo-100 px-2 py-0.5 text-xs font-semibold text-indigo-700">
          {{ pendingRequests.length }}
        </span>
      </div>

      <div
        v-if="pendingRequests.length === 0"
        class="rounded-2xl border border-dashed border-indigo-200 px-3 py-4 text-sm text-slate-500"
      >
        No pending requests.
      </div>

      <div v-else class="space-y-2">
        <div
          v-for="request in pendingRequests"
          :key="request.id"
          class="rounded-2xl border border-indigo-100 bg-white px-3 py-3 shadow-sm"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-slate-900">{{ request.senderId }}</p>
              <p class="mt-1 text-xs text-slate-500">Sent {{ formatRequestTimestamp(request.createdAt) }}</p>
            </div>
            <div class="flex shrink-0 items-center gap-1">
              <button
                type="button"
                class="inline-flex h-8 w-8 items-center justify-center rounded-full bg-emerald-100 text-emerald-700 transition hover:bg-emerald-200"
                @click="emit('acceptRequest', request.id)"
              >
                <Check :size="16" />
              </button>
              <button
                type="button"
                class="inline-flex h-8 w-8 items-center justify-center rounded-full bg-rose-100 text-rose-700 transition hover:bg-rose-200"
                @click="emit('declineRequest', request.id)"
              >
                <X :size="16" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto">
      <div class="flex items-center justify-between px-4 py-3">
        <h2 class="text-sm font-semibold text-slate-800">Friends list</h2>
        <span class="rounded-full bg-slate-200 px-2 py-0.5 text-xs font-semibold text-slate-700">
          {{ contacts.length }}
        </span>
      </div>

      <div v-if="isLoading" class="px-4 py-8 text-sm text-slate-500">
        Loading friends...
      </div>

      <div v-else-if="filteredContacts.length === 0" class="px-4 py-8 text-sm text-slate-500">
        {{ contacts.length === 0 ? 'No friends yet. Send a request to start.' : 'No friends match your search.' }}
      </div>

      <div v-else class="space-y-2 px-3 pb-4">
        <div
          v-for="contact in filteredContacts"
          :key="contact.id"
          :class="[
            'flex items-center gap-3 rounded-3xl border p-3 transition',
            selectedContactId === contact.id
              ? 'border-indigo-300 bg-indigo-100/75 shadow-sm'
              : 'border-transparent bg-white/75 hover:border-indigo-100 hover:bg-white',
          ]"
        >
          <button
            type="button"
            class="flex min-w-0 flex-1 items-center gap-3 text-left"
            @click="emit('selectContact', contact.id)"
          >
            <div class="relative">
              <div class="flex size-11 items-center justify-center overflow-hidden rounded-full bg-indigo-200 font-semibold text-indigo-800 ring-2 ring-white">
                <img v-if="contact.avatar" :src="contact.avatar" :alt="contact.name" class="h-full w-full object-cover" />
                <span v-else>{{ contact.name.slice(0, 2).toUpperCase() }}</span>
              </div>
              <span v-if="contact.online" class="absolute bottom-0 right-0 size-3 rounded-full border-2 border-white bg-emerald-500" />
            </div>

            <div class="min-w-0 flex-1">
              <div class="mb-1 flex items-center justify-between gap-2">
                <div class="flex min-w-0 items-center gap-2">
                  <h3 class="truncate text-sm font-semibold text-slate-900">{{ contact.name }}</h3>
                  <span
                    v-if="contact.isCurrentChat"
                    class="rounded-full bg-emerald-50 px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-[0.18em] text-emerald-700"
                  >
                    Here
                  </span>
                </div>
                <span class="shrink-0 text-xs font-medium text-indigo-500">{{ contact.timestamp }}</span>
              </div>

              <div class="flex items-center justify-between gap-3">
                <p :class="['truncate text-xs', contact.isTyping ? 'font-medium text-emerald-600' : 'text-slate-500']">
                  {{ contact.lastMessage }}
                </p>
                <span
                  v-if="contact.unreadCount > 0"
                  class="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-emerald-500 px-1.5 text-[11px] font-semibold text-white"
                >
                  {{ contact.unreadCount }}
                </span>
              </div>
            </div>
          </button>

          <button
            type="button"
            class="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-slate-400 transition hover:bg-rose-50 hover:text-rose-600"
            @click="emit('removeFriend', contact.id)"
          >
            <Trash2 :size="16" />
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>
