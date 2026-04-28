<script setup lang="ts">
import { useTemplateRef } from 'vue';
import { LogOut, MessageCirclePlus, Moon, Sun, Wifi, WifiOff } from 'lucide-vue-next';
import { useTheme } from './chat/useTheme.ts';

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

const { isDark, toggleTheme } = useTheme();

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
  currentFriendCode,
  draftAttachments,
  handleAcceptRequest,
  handleAttachFiles,
  handleAuthSubmit: submitAuth,
  handleDeclineRequest,
  handleLeaveConversation,
  handleLoadOlderMessages,
  handleLogout,
  handleRemoveFriend,
  handleRemoveDraftAttachment,
  handleSelectContact,
  handleSendFriendRequest,
  handleSendMessage,
  handleTypingStarted,
  handleTypingStopped,
  isAuthenticated,
  isFriendsLoading,
  isPeerTyping,
  isUploadingAttachments,
  pendingRequests,
  selectedContact,
  selectedContactId,
  selectedContactStatus,
  selectedConversationState,
  selectedPresence,
  sessionToken,
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
      class="relative flex min-h-screen items-center justify-center bg-gradient-to-br from-slate-900 via-indigo-950 to-violet-900 px-4 py-8"
    >
      <div class="absolute inset-0 bg-[radial-gradient(circle_at_20%_20%,rgba(129,140,248,.2),transparent_35%),radial-gradient(circle_at_80%_0%,rgba(14,165,233,.18),transparent_25%)]" />
      <button
        type="button"
        class="absolute right-4 top-4 z-20 inline-flex h-9 w-9 cursor-pointer items-center justify-center rounded-full bg-white/10 text-white/70 transition hover:bg-white/20 hover:text-white"
        @click="toggleTheme"
      >
        <Moon v-if="!isDark" :size="16" />
        <Sun v-else :size="16" />
      </button>
      <AuthCard ref="authCardRef" :mode="authMode" @submit="handleAuthSubmit" @toggleMode="toggleAuthMode" />
    </div>

    <div
      v-else
      class="chat-bg relative mx-auto flex h-screen max-w-[1600px] overflow-hidden shadow-2xl"
    >
      <div class="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(255,255,255,0.35)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.28)_1px,transparent_1px)] bg-[size:26px_26px] opacity-25 dark:opacity-5" />

      <div class="absolute right-3 top-3 z-20 flex items-center gap-2">
        <button
          type="button"
          class="inline-flex h-9 w-9 cursor-pointer items-center justify-center rounded-full bg-white/90 text-slate-600 shadow-md transition hover:bg-white dark:bg-slate-800/90 dark:text-slate-400 dark:hover:bg-slate-800"
          @click="toggleTheme"
        >
          <Moon v-if="!isDark" :size="15" />
          <Sun v-else :size="15" />
        </button>
        <button
          type="button"
          class="inline-flex cursor-pointer items-center gap-1 rounded-full bg-white/90 px-3 py-2 text-xs font-semibold text-slate-700 shadow-md transition hover:bg-white dark:bg-slate-800/90 dark:text-slate-300 dark:hover:bg-slate-800"
          @click="handleLogout"
        >
          <LogOut :size="14" />
          Logout {{ currentUser }}
        </button>
      </div>

      <ChatSidebar
        :class="[selectedContactId ? 'hidden md:flex' : 'flex']"
        :contacts="contacts"
        :pendingRequests="pendingRequests"
        :selectedContactId="selectedContactId"
        :isLoading="isFriendsLoading"
        :currentUsername="currentUser"
        :myFriendCode="currentFriendCode"
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

        <div class="border-b border-white/60 bg-white/65 px-4 py-2 text-xs font-medium text-slate-500 backdrop-blur-sm dark:border-slate-700/60 dark:bg-slate-900/65 dark:text-slate-400">
          <span class="inline-flex items-center gap-2">
            <Wifi v-if="connectionState === 'connected'" :size="14" class="text-emerald-500" />
            <WifiOff v-else :size="14" class="text-amber-500" />
            {{ connectionState === 'connected' ? 'Realtime connected' : 'Realtime reconnecting' }}
          </span>
        </div>

        <ChatMessages
          :messages="currentMessages"
          :contact-id="selectedContact.id"
          :attachment-token="sessionToken"
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
          :attachments="draftAttachments"
          :is-uploading-attachments="isUploadingAttachments"
          @attachFiles="handleAttachFiles"
          @removeAttachment="handleRemoveDraftAttachment"
          @sendMessage="handleSendMessage"
          @typingStarted="handleTypingStarted"
          @typingStopped="handleTypingStopped"
        />
      </div>

      <div v-else class="flex flex-1 items-center justify-center px-6">
        <div class="text-center">
          <div class="mx-auto mb-4 flex size-16 items-center justify-center rounded-full bg-white/80 shadow-lg dark:bg-slate-800/80">
            <MessageCirclePlus :size="32" class="text-indigo-600" />
          </div>
          <p class="font-medium text-slate-700 dark:text-slate-300">Choose a conversation</p>
          <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">Open a friend from the sidebar to start a WhatsApp-like 1:1 chat.</p>
        </div>
      </div>
    </div>
  </main>
</template>
