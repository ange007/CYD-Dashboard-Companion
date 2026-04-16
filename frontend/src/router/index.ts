import { createRouter, createWebHashHistory } from 'vue-router'
import DashboardPage from '@/pages/DashboardPage.vue'
import SettingsPage  from '@/pages/SettingsPage.vue'
import RulesPage     from '@/pages/RulesPage.vue'

export default createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/',         component: DashboardPage },
    { path: '/settings', component: SettingsPage },
    { path: '/rules',    component: RulesPage },
  ],
})
