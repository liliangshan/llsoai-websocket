import { createRouter, createWebHistory } from 'vue-router';
import ChatView from '@/views/ChatView.vue';
import { ROUTES } from '@/constants/routes';

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: ROUTES.chat },
    { path: ROUTES.chat, component: ChatView },
    { path: '/:pathMatch(.*)*', redirect: ROUTES.chat },
  ],
});

