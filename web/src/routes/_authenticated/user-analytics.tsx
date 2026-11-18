import { createFileRoute, redirect } from '@tanstack/react-router'

import { UserAnalyticsPage } from '@/features/user-analytics'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/user-analytics')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: UserAnalyticsPage,
})
