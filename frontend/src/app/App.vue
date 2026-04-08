<script setup lang="ts">
import { useTemplateRef } from 'vue';
import { LogOut, MessageCirclePlus, Wifi, WifiOff } from 'lucide-vue-next';

import AuthCard from './components/AuthCard.vue';
import ChatHeader from './components/ChatHeader.vue';
import ChatMessages from './components/ChatMessages.vue';
import ChatSidebar from './components/ChatSidebar.vue';
import MessageInput from './components/MessageInput.vue';
import { useChatController } from './chat/useChatController.ts';
import type { AuthMode } from './chat/types.ts';

type AuthCardExposed = {
  setError: (message: string) => void;
  setSuccess: (message: string) => void;
};

const authCardRef = useTemplateRef<AuthCardExposed>('authCardRef');

const {
  actionError,
  actionSuccess,
  authMode,
  composerDisabled,
  composerPlaceholder,
  connectionState,
  contacts,
  currentMessages,
  currentUser,
  currentUserId,
  handleAcceptRequest,
  handleAuthSubmit: submitAuth,
  handleDeclineRequest,
  handleLeaveConversation,
  handleLoadOlderMessages,
  handleLogout,
  handleRemoveFriend,
  handleSelectContact,
  handleSendFriendRequest,
  handleSendMessage,
  handleTypingStarted,
  handleTypingStopped,
  isAuthenticated,
  isFriendsLoading,
  isPeerTyping,
  pendingRequests,
  selectedContact,
  selectedContactId,
  selectedContactStatus,
  selectedConversationState,
  selectedPresence,
  toggleAuthMode,
  acknowledgeVisibleConversation,
} = useChatController();

const handleAuthSubmit = async (payload: { username: string; password: string; mode: AuthMode }) => {
  const result = await submitAuth(payload);

  if (result.error) {
    authCardRef.value?.setError(result.error);
  }

  if (result.success) {
    authCardRef.value?.setSuccess(result.success);
  }
};
</script>

<template>
  <main>
    <div
      v-if="!isAuthenticated"
      class="flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-900 via-indigo-950 to-violet-900 px-4 py-8"
    >
      <div class="absolute inset-0 bg-[radial-gradient(circle_at_20%_20%,rgba(129,140,248,.2),transparent_35%),radial-gradient(circle_at_80%_0%,rgba(14,165,233,.18),transparent_25%)]" />
      <AuthCard ref="authCardRef" :mode="authMode" @submit="handleAuthSubmit" @toggleMode="toggleAuthMode" />
    </div>

    <div
      v-else
      class="relative mx-auto flex h-screen max-w-[1600px] overflow-hidden bg-[radial-gradient(circle_at_top_left,rgba(129,140,248,0.25),transparent_32%),radial-gradient(circle_at_top_right,rgba(34,197,94,0.15),transparent_22%),linear-gradient(135deg,#eef2ff_0%,#f8fafc_45%,#fdf2f8_100%)] shadow-2xl"
    >
      <div class="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.35)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.28)_1px,transparent_1px)] bg-[size:26px_26px] opacity-25" />

      <button
        type="button"
        class="absolute right-3 top-3 z-20 inline-flex items-center gap-1 rounded-full bg-white/90 px-3 py-2 text-xs font-semibold text-slate-700 shadow-md transition hover:bg-white"
        @click="handleLogout"
      >
        <LogOut :size="14" />
        Logout {{ currentUser }}
      </button>

      <ChatSidebar
        :class="[selectedContactId ? 'hidden md:flex' : 'flex']"
        :contacts="contacts"
        :pendingRequests="pendingRequests"
        :selectedContactId="selectedContactId"
        :isLoading="isFriendsLoading"
        :currentUserId="currentUserId"
        :actionError="actionError"
        :actionSuccess="actionSuccess"
        @selectContact="handleSelectContact"
        @sendFriendRequest="handleSendFriendRequest"
        @acceptRequest="handleAcceptRequest"
        @declineRequest="handleDeclineRequest"
        @removeFriend="handleRemoveFriend"
      />

      <div v-if="selectedContact" class="flex min-w-0 flex-1 flex-col">
        <ChatHeader
          :name="selectedContact.name"
          :avatar="selectedContact.avatar"
          :status="selectedContactStatus"
          :online="selectedContact.online"
          :isTyping="isPeerTyping"
          :isInCurrentChat="Boolean(selectedPresence?.current_chat_id && selectedPresence.current_chat_id === currentUserId)"
          :showBackButton="Boolean(selectedContactId)"
          @back="handleLeaveConversation"
        />

        <div class="border-b border-white/60 bg-white/65 px-4 py-2 text-xs font-medium text-slate-500 backdrop-blur-sm">
          <span class="inline-flex items-center gap-2">
            <Wifi v-if="connectionState === 'connected'" :size="14" class="text-emerald-500" />
            <WifiOff v-else :size="14" class="text-amber-500" />
            {{ connectionState === 'connected' ? 'Realtime connected' : 'Realtime reconnecting' }}
          </span>
        </div>

        <ChatMessages
          :messages="currentMessages"
          :contact-id="selectedContact.id"
          :has-more="selectedConversationState.hasMore"
          :is-loading-more="selectedConversationState.isLoading"
          :is-peer-typing="isPeerTyping"
          :typing-label="`${selectedContact.name} is typing...`"
          @loadMore="handleLoadOlderMessages"
          @messagesRendered="acknowledgeVisibleConversation"
        />
        <MessageInput
          :disabled="composerDisabled"
          :placeholder="composerPlaceholder"
          @sendMessage="handleSendMessage"
          @typingStarted="handleTypingStarted"
          @typingStopped="handleTypingStopped"
        />
      </div>

      <div v-else class="flex flex-1 items-center justify-center px-6">
        <div class="text-center">
          <div class="mx-auto mb-4 flex size-16 items-center justify-center rounded-full bg-white/80 shadow-lg">
            <MessageCirclePlus :size="32" class="text-indigo-600" />
          </div>
          <p class="font-medium text-slate-700">Choose a conversation</p>
          <p class="mt-1 text-sm text-slate-500">Open a friend from the sidebar to start a WhatsApp-like 1:1 chat.</p>
        </div>
      </div>
    </div>
  </main>
</template>
