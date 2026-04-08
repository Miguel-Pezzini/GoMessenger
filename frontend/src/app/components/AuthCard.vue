<script setup lang="ts">
import { computed, ref } from 'vue';
import { LoaderCircle, MessageCircleMore } from 'lucide-vue-next';

type AuthMode = 'login' | 'register';

const props = defineProps<{
  mode: AuthMode;
}>();

const emit = defineEmits<{
  submit: [payload: { username: string; password: string; mode: AuthMode }];
  toggleMode: [];
}>();

const username = ref('');
const password = ref('');
const isLoading = ref(false);
const error = ref('');
const success = ref('');

const title = computed(() =>
  props.mode === 'login' ? 'Welcome back' : 'Create your account'
);

const subtitle = computed(() =>
  props.mode === 'login'
    ? 'Sign in to continue your conversations.'
    : 'Register to start chatting in real-time.'
);

const cta = computed(() => (props.mode === 'login' ? 'Sign in' : 'Create account'));
const toggleText = computed(() =>
  props.mode === 'login'
    ? "Don't have an account? Register"
    : 'Already have an account? Sign in'
);

const submit = async () => {
  error.value = '';
  success.value = '';

  if (!username.value.trim() || !password.value.trim()) {
    error.value = 'Username and password are required.';
    return;
  }

  isLoading.value = true;

  try {
    emit('submit', {
      username: username.value.trim(),
      password: password.value,
      mode: props.mode,
    });
  } finally {
    setTimeout(() => {
      isLoading.value = false;
    }, 350);
  }
};

const setError = (message: string) => {
  error.value = message;
};

const setSuccess = (message: string) => {
  success.value = message;
};

defineExpose({
  setError,
  setSuccess,
});
</script>

<template>
  <section class="w-full max-w-md rounded-3xl border border-white/40 bg-white/75 p-6 shadow-2xl backdrop-blur-xl sm:p-8">
    <div class="mb-6">
      <div class="mb-4 inline-flex items-center gap-2 rounded-full bg-indigo-100 px-3 py-1 text-sm font-medium text-indigo-700">
        <MessageCircleMore :size="16" />
        GoMessenger
      </div>
      <h1 class="text-2xl font-bold text-gray-900 sm:text-3xl">{{ title }}</h1>
      <p class="mt-2 text-sm text-gray-600 sm:text-base">{{ subtitle }}</p>
    </div>

    <form class="space-y-4" @submit.prevent="submit">
      <div>
        <label class="mb-1 block text-sm font-medium text-gray-700" for="username">Username</label>
        <input
          id="username"
          v-model="username"
          type="text"
          autocomplete="username"
          class="w-full rounded-xl border border-gray-200 bg-white px-4 py-3 text-gray-900 outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
          placeholder="Enter your username"
        />
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium text-gray-700" for="password">Password</label>
        <input
          id="password"
          v-model="password"
          type="password"
          :autocomplete="mode === 'login' ? 'current-password' : 'new-password'"
          class="w-full rounded-xl border border-gray-200 bg-white px-4 py-3 text-gray-900 outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
          placeholder="Enter your password"
        />
      </div>

      <p v-if="error" class="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">{{ error }}</p>
      <p v-if="success" class="rounded-lg bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{{ success }}</p>

      <button
        type="submit"
        class="inline-flex w-full items-center justify-center gap-2 rounded-xl bg-indigo-600 px-4 py-3 text-sm font-semibold text-white transition hover:bg-indigo-500 disabled:cursor-not-allowed disabled:opacity-70"
        :disabled="isLoading"
      >
        <LoaderCircle v-if="isLoading" :size="16" class="animate-spin" />
        {{ cta }}
      </button>
    </form>

    <button
      type="button"
      class="mt-4 w-full text-sm font-medium text-indigo-700 transition hover:text-indigo-900"
      @click="emit('toggleMode')"
    >
      {{ toggleText }}
    </button>
  </section>
</template>
