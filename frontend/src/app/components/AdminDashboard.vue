<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue';
import {
  Activity,
  AlertTriangle,
  BarChart3,
  Circle,
  Clock,
  Filter,
  LogOut,
  MessageSquare,
  Moon,
  RefreshCw,
  Search,
  Server,
  Sun,
  Users,
  Wifi,
  WifiOff,
} from 'lucide-vue-next';
import {
  createAdminLogsSocket,
  fetchActiveUsers,
  fetchAdminLogs,
  type ActiveUser,
  type AdminLogEvent,
  type AdminLogFilters,
} from '../admin/api.ts';

const props = defineProps<{
  token: string;
  username: string;
  isDark: boolean;
}>();

const emit = defineEmits<{
  logout: [];
  toggleTheme: [];
  goChat: [];
}>();

const logs = ref<AdminLogEvent[]>([]);
const activeUsers = ref<ActiveUser[]>([]);
const activeCount = ref(0);
const isLoadingLogs = ref(false);
const isLoadingActiveUsers = ref(false);
const logsError = ref('');
const activeUsersError = ref('');
const liveError = ref('');
const isLiveConnected = ref(false);
const lastUpdatedAt = ref<number | null>(null);

const filters = reactive<AdminLogFilters>({
  limit: 100,
  service: '',
  category: '',
  status: '',
  eventType: '',
  actorUserId: '',
  query: '',
});

let logsSocket: WebSocket | null = null;
let activeUsersTimer: number | null = null;
let reconnectTimer: number | null = null;

const dateTimeFormatter = new Intl.DateTimeFormat('en-US', {
  month: 'short',
  day: 'numeric',
  hour: 'numeric',
  minute: '2-digit',
  second: '2-digit',
});

const metricCards = computed(() => {
  const failures = logs.value.filter((event) => event.status === 'failure' || event.category === 'error').length;
  const services = new Set(logs.value.map((event) => event.service).filter(Boolean)).size;

  return [
    {
      label: 'Active users',
      value: activeCount.value,
      icon: Users,
      tone: 'text-emerald-600 bg-emerald-50 dark:text-emerald-300 dark:bg-emerald-500/10',
    },
    {
      label: 'Loaded events',
      value: logs.value.length,
      icon: Activity,
      tone: 'text-sky-600 bg-sky-50 dark:text-sky-300 dark:bg-sky-500/10',
    },
    {
      label: 'Failures',
      value: failures,
      icon: AlertTriangle,
      tone: 'text-rose-600 bg-rose-50 dark:text-rose-300 dark:bg-rose-500/10',
    },
    {
      label: 'Services',
      value: services,
      icon: Server,
      tone: 'text-amber-600 bg-amber-50 dark:text-amber-300 dark:bg-amber-500/10',
    },
  ];
});

const serviceOptions = computed(() => {
  return Array.from(new Set(logs.value.map((event) => event.service).filter(Boolean))).sort();
});

const liveStatusLabel = computed(() => (isLiveConnected.value ? 'Live stream connected' : 'Live stream offline'));

const formatDateTime = (value?: string | null) => {
  if (!value) {
    return 'Never';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return dateTimeFormatter.format(date);
};

const statusClass = (status: string) => {
  if (status === 'success') {
    return 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/20';
  }
  if (status === 'failure') {
    return 'bg-rose-50 text-rose-700 ring-rose-200 dark:bg-rose-500/10 dark:text-rose-300 dark:ring-rose-500/20';
  }
  return 'bg-slate-100 text-slate-700 ring-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:ring-slate-700';
};

const metadataPreview = (metadata?: Record<string, unknown>) => {
  if (!metadata || Object.keys(metadata).length === 0) {
    return '';
  }
  return Object.entries(metadata)
    .slice(0, 3)
    .map(([key, value]) => `${key}: ${String(value)}`)
    .join('  ');
};

const mergeLiveLog = (event: AdminLogEvent) => {
  const key = event.stream_id || event.event_id;
  logs.value = [event, ...logs.value.filter((item) => (item.stream_id || item.event_id) !== key)].slice(0, 200);
  lastUpdatedAt.value = Date.now();
};

async function loadLogs() {
  if (!props.token) {
    return;
  }

  isLoadingLogs.value = true;
  logsError.value = '';
  try {
    logs.value = await fetchAdminLogs(props.token, filters);
    lastUpdatedAt.value = Date.now();
  } catch (error) {
    logsError.value = error instanceof Error ? error.message : 'Failed to load admin logs.';
  } finally {
    isLoadingLogs.value = false;
  }
}

async function loadActiveUsers() {
  if (!props.token) {
    return;
  }

  isLoadingActiveUsers.value = true;
  activeUsersError.value = '';
  try {
    const response = await fetchActiveUsers(props.token, 100);
    activeUsers.value = response.users;
    activeCount.value = response.count;
  } catch (error) {
    activeUsersError.value = error instanceof Error ? error.message : 'Failed to load active users.';
  } finally {
    isLoadingActiveUsers.value = false;
  }
}

function clearReconnectTimer() {
  if (reconnectTimer !== null) {
    window.clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
}

function disconnectLiveLogs() {
  clearReconnectTimer();
  if (!logsSocket) {
    isLiveConnected.value = false;
    return;
  }
  const socket = logsSocket;
  logsSocket = null;
  socket.close();
  isLiveConnected.value = false;
}

function connectLiveLogs() {
  disconnectLiveLogs();
  liveError.value = '';
  if (!props.token) {
    return;
  }

  const socket = createAdminLogsSocket(props.token);
  logsSocket = socket;

  socket.addEventListener('open', () => {
    if (logsSocket !== socket) {
      return;
    }
    isLiveConnected.value = true;
    liveError.value = '';
  });

  socket.addEventListener('message', (event: MessageEvent) => {
    try {
      mergeLiveLog(JSON.parse(String(event.data)) as AdminLogEvent);
    } catch {
      // Ignore malformed log events.
    }
  });

  socket.addEventListener('close', () => {
    if (logsSocket !== socket) {
      return;
    }
    isLiveConnected.value = false;
    reconnectTimer = window.setTimeout(() => {
      connectLiveLogs();
    }, 2500);
  });

  socket.addEventListener('error', () => {
    if (logsSocket !== socket) {
      return;
    }
    liveError.value = 'Live log stream is unavailable.';
  });
}

function resetFilters() {
  filters.limit = 100;
  filters.service = '';
  filters.category = '';
  filters.status = '';
  filters.eventType = '';
  filters.actorUserId = '';
  filters.query = '';
  void loadLogs();
}

function startActiveUsersPolling() {
  if (activeUsersTimer !== null) {
    window.clearInterval(activeUsersTimer);
  }
  activeUsersTimer = window.setInterval(() => {
    void loadActiveUsers();
  }, 10000);
}

watch(
  () => props.token,
  () => {
    void loadLogs();
    void loadActiveUsers();
    connectLiveLogs();
  }
);

onMounted(() => {
  void loadLogs();
  void loadActiveUsers();
  connectLiveLogs();
  startActiveUsersPolling();
});

onBeforeUnmount(() => {
  disconnectLiveLogs();
  if (activeUsersTimer !== null) {
    window.clearInterval(activeUsersTimer);
  }
});
</script>

<template>
  <section class="min-h-screen bg-slate-100 text-slate-950 dark:bg-slate-950 dark:text-slate-100">
    <header class="border-b border-slate-200 bg-white/90 px-4 py-3 backdrop-blur dark:border-slate-800 dark:bg-slate-950/90">
      <div class="mx-auto flex max-w-[1600px] flex-wrap items-center justify-between gap-3">
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-slate-950 text-white dark:bg-white dark:text-slate-950">
            <BarChart3 :size="20" />
          </div>
          <div class="min-w-0">
            <h1 class="truncate text-lg font-semibold leading-tight text-slate-950 dark:text-white">Admin dashboard</h1>
            <p class="truncate text-sm text-slate-500 dark:text-slate-400">{{ username }}</p>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <span
            class="hidden items-center gap-2 rounded-lg px-3 py-2 text-xs font-semibold ring-1 sm:inline-flex"
            :class="isLiveConnected ? 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/20' : 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/20'"
          >
            <Wifi v-if="isLiveConnected" :size="14" />
            <WifiOff v-else :size="14" />
            {{ liveStatusLabel }}
          </span>
          <button
            type="button"
            class="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
            @click="emit('goChat')"
          >
            <MessageSquare :size="16" />
            Chat
          </button>
          <button
            type="button"
            class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-700 transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
            @click="emit('toggleTheme')"
          >
            <Moon v-if="!isDark" :size="16" />
            <Sun v-else :size="16" />
          </button>
          <button
            type="button"
            class="inline-flex h-9 items-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white transition hover:bg-slate-800 dark:bg-white dark:text-slate-950 dark:hover:bg-slate-200"
            @click="emit('logout')"
          >
            <LogOut :size="16" />
            Logout
          </button>
        </div>
      </div>
    </header>

    <div class="mx-auto grid max-w-[1600px] gap-4 px-4 py-4 xl:grid-cols-[1fr_360px]">
      <div class="space-y-4">
        <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <article
            v-for="card in metricCards"
            :key="card.label"
            class="rounded-lg border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900"
          >
            <div class="flex items-center justify-between gap-3">
              <div>
                <p class="text-sm font-medium text-slate-500 dark:text-slate-400">{{ card.label }}</p>
                <p class="mt-1 text-2xl font-semibold text-slate-950 dark:text-white">{{ card.value }}</p>
              </div>
              <div class="flex h-10 w-10 items-center justify-center rounded-lg" :class="card.tone">
                <component :is="card.icon" :size="19" />
              </div>
            </div>
          </article>
        </div>

        <section class="rounded-lg border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 px-4 py-3 dark:border-slate-800">
            <div>
              <h2 class="text-base font-semibold text-slate-950 dark:text-white">Audit logs</h2>
              <p class="text-xs text-slate-500 dark:text-slate-400">
                {{ lastUpdatedAt ? `Updated ${formatDateTime(new Date(lastUpdatedAt).toISOString())}` : 'Waiting for data' }}
              </p>
            </div>
            <button
              type="button"
              class="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
              @click="loadLogs"
            >
              <RefreshCw :size="15" :class="{ 'animate-spin': isLoadingLogs }" />
              Refresh
            </button>
          </div>

          <form class="grid gap-3 border-b border-slate-200 px-4 py-3 dark:border-slate-800 lg:grid-cols-[1.5fr_repeat(5,minmax(0,1fr))_auto]" @submit.prevent="loadLogs">
            <label class="relative">
              <span class="sr-only">Search logs</span>
              <Search class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" :size="16" />
              <input
                v-model="filters.query"
                type="search"
                class="h-10 w-full rounded-lg border border-slate-200 bg-slate-50 pl-9 pr-3 text-sm text-slate-900 outline-none transition focus:border-sky-400 focus:bg-white focus:ring-2 focus:ring-sky-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 dark:focus:border-sky-500 dark:focus:ring-sky-500/20"
                placeholder="Search"
              />
            </label>
            <select v-model="filters.service" class="h-10 rounded-lg border border-slate-200 bg-slate-50 px-3 text-sm text-slate-900 outline-none focus:border-sky-400 focus:ring-2 focus:ring-sky-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 dark:focus:border-sky-500 dark:focus:ring-sky-500/20">
              <option value="">All services</option>
              <option v-for="service in serviceOptions" :key="service" :value="service">{{ service }}</option>
            </select>
            <select v-model="filters.category" class="h-10 rounded-lg border border-slate-200 bg-slate-50 px-3 text-sm text-slate-900 outline-none focus:border-sky-400 focus:ring-2 focus:ring-sky-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 dark:focus:border-sky-500 dark:focus:ring-sky-500/20">
              <option value="">All categories</option>
              <option value="audit">Audit</option>
              <option value="error">Error</option>
            </select>
            <select v-model="filters.status" class="h-10 rounded-lg border border-slate-200 bg-slate-50 px-3 text-sm text-slate-900 outline-none focus:border-sky-400 focus:ring-2 focus:ring-sky-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 dark:focus:border-sky-500 dark:focus:ring-sky-500/20">
              <option value="">All statuses</option>
              <option value="success">Success</option>
              <option value="failure">Failure</option>
            </select>
            <input
              v-model="filters.eventType"
              type="text"
              class="h-10 rounded-lg border border-slate-200 bg-slate-50 px-3 text-sm text-slate-900 outline-none focus:border-sky-400 focus:ring-2 focus:ring-sky-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 dark:focus:border-sky-500 dark:focus:ring-sky-500/20"
              placeholder="Event type"
            />
            <input
              v-model="filters.actorUserId"
              type="text"
              class="h-10 rounded-lg border border-slate-200 bg-slate-50 px-3 text-sm text-slate-900 outline-none focus:border-sky-400 focus:ring-2 focus:ring-sky-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 dark:focus:border-sky-500 dark:focus:ring-sky-500/20"
              placeholder="Actor ID"
            />
            <div class="flex items-center gap-2">
              <button type="submit" class="inline-flex h-10 items-center gap-2 rounded-lg bg-slate-950 px-3 text-sm font-semibold text-white transition hover:bg-slate-800 dark:bg-white dark:text-slate-950 dark:hover:bg-slate-200">
                <Filter :size="15" />
                Apply
              </button>
              <button type="button" class="inline-flex h-10 items-center rounded-lg border border-slate-200 px-3 text-sm font-semibold text-slate-600 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800" @click="resetFilters">
                Reset
              </button>
            </div>
          </form>

          <div v-if="logsError || liveError" class="border-b border-rose-100 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-500/20 dark:bg-rose-500/10 dark:text-rose-300">
            {{ logsError || liveError }}
          </div>

          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-slate-200 text-left dark:divide-slate-800">
              <thead class="bg-slate-50 text-xs uppercase text-slate-500 dark:bg-slate-950 dark:text-slate-400">
                <tr>
                  <th class="px-4 py-3 font-semibold">Time</th>
                  <th class="px-4 py-3 font-semibold">Service</th>
                  <th class="px-4 py-3 font-semibold">Event</th>
                  <th class="px-4 py-3 font-semibold">Status</th>
                  <th class="px-4 py-3 font-semibold">Actor</th>
                  <th class="px-4 py-3 font-semibold">Message</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
                <tr v-if="isLoadingLogs && logs.length === 0">
                  <td colspan="6" class="px-4 py-10 text-center text-sm text-slate-500 dark:text-slate-400">Loading logs...</td>
                </tr>
                <tr v-else-if="logs.length === 0">
                  <td colspan="6" class="px-4 py-10 text-center text-sm text-slate-500 dark:text-slate-400">No logs found.</td>
                </tr>
                <tr v-for="event in logs" :key="event.stream_id || event.event_id" class="align-top transition hover:bg-slate-50 dark:hover:bg-slate-800/50">
                  <td class="whitespace-nowrap px-4 py-3 text-sm text-slate-500 dark:text-slate-400">
                    <span class="inline-flex items-center gap-2">
                      <Clock :size="14" />
                      {{ formatDateTime(event.occurred_at) }}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-sm font-semibold text-slate-800 dark:text-slate-200">{{ event.service }}</td>
                  <td class="px-4 py-3">
                    <p class="text-sm font-semibold text-slate-950 dark:text-white">{{ event.event_type }}</p>
                    <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ event.category }}</p>
                  </td>
                  <td class="px-4 py-3">
                    <span class="inline-flex rounded-lg px-2 py-1 text-xs font-semibold ring-1" :class="statusClass(event.status)">{{ event.status }}</span>
                  </td>
                  <td class="max-w-[180px] truncate px-4 py-3 text-sm text-slate-600 dark:text-slate-300">{{ event.actor_user_id || '-' }}</td>
                  <td class="min-w-[280px] px-4 py-3">
                    <p class="text-sm text-slate-800 dark:text-slate-200">{{ event.message }}</p>
                    <p v-if="metadataPreview(event.metadata)" class="mt-1 max-w-xl truncate text-xs text-slate-500 dark:text-slate-400">{{ metadataPreview(event.metadata) }}</p>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <aside class="space-y-4">
        <section class="rounded-lg border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div class="flex items-center justify-between gap-3 border-b border-slate-200 px-4 py-3 dark:border-slate-800">
            <div>
              <h2 class="text-base font-semibold text-slate-950 dark:text-white">Active users</h2>
              <p class="text-xs text-slate-500 dark:text-slate-400">{{ activeCount }} online</p>
            </div>
            <button
              type="button"
              class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-slate-200 text-slate-600 transition hover:bg-slate-50 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
              @click="loadActiveUsers"
            >
              <RefreshCw :size="15" :class="{ 'animate-spin': isLoadingActiveUsers }" />
            </button>
          </div>

          <div v-if="activeUsersError" class="border-b border-rose-100 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-500/20 dark:bg-rose-500/10 dark:text-rose-300">
            {{ activeUsersError }}
          </div>

          <div class="max-h-[560px] overflow-y-auto">
            <div v-if="isLoadingActiveUsers && activeUsers.length === 0" class="px-4 py-8 text-center text-sm text-slate-500 dark:text-slate-400">Loading users...</div>
            <div v-else-if="activeUsers.length === 0" class="px-4 py-8 text-center text-sm text-slate-500 dark:text-slate-400">No active users.</div>
            <div v-for="user in activeUsers" :key="user.user_id" class="border-b border-slate-100 px-4 py-3 last:border-0 dark:border-slate-800">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="truncate text-sm font-semibold text-slate-950 dark:text-white">{{ user.username || user.user_id }}</p>
                  <p class="mt-1 truncate text-xs text-slate-500 dark:text-slate-400">{{ user.user_id }}</p>
                </div>
                <span class="inline-flex items-center gap-1 rounded-lg bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/20">
                  <Circle :size="8" fill="currentColor" />
                  online
                </span>
              </div>
              <p v-if="user.current_chat_id" class="mt-2 truncate text-xs text-slate-500 dark:text-slate-400">Chat: {{ user.current_chat_id }}</p>
            </div>
          </div>
        </section>
      </aside>
    </div>
  </section>
</template>
