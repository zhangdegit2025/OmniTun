import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate, Outlet } from 'react-router-dom'
import { useAuthStore } from '@/store/auth'
import Layout from '@/components/Layout'
import { CommandPaletteProvider } from '@/components/CommandPalette'
import { Skeleton } from '@/components/ui/Skeleton'

const Login = lazy(() => import('@/pages/Login'))
const Register = lazy(() => import('@/pages/Register'))
const ForgotPassword = lazy(() => import('@/pages/ForgotPassword'))
const ResetPassword = lazy(() => import('@/pages/ResetPassword'))
const Dashboard = lazy(() => import('@/pages/Dashboard'))
const Tunnels = lazy(() => import('@/pages/Tunnels'))
const TunnelDetail = lazy(() => import('@/pages/TunnelDetail'))
const Domains = lazy(() => import('@/pages/Domains'))
const Networks = lazy(() => import('@/pages/Networks'))
const NetworkDetail = lazy(() => import('@/pages/NetworkDetail'))
const Settings = lazy(() => import('@/pages/Settings'))
const Billing = lazy(() => import('@/pages/Billing'))
const Onboarding = lazy(() => import('@/pages/Onboarding'))
const Notifications = lazy(() => import('@/pages/Notifications'))
const ApiDocs = lazy(() => import('@/pages/ApiDocs'))
const Downloads = lazy(() => import('@/pages/Downloads'))

function PageSkeleton() {
  return (
    <div className="p-6 space-y-4">
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-64 w-full" />
    </div>
  )
}

function LazyRoute({ children }: { children: React.ReactNode }) {
  return <Suspense fallback={<PageSkeleton />}>{children}</Suspense>
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isLoggedIn = useAuthStore((s) => s.isLoggedIn)
  if (!isLoggedIn) return <Navigate to="/login" replace />
  return <>{children}</>
}

function ProtectedLayout() {
  return (
    <Layout>
      <LazyRoute>
        <Outlet />
      </LazyRoute>
    </Layout>
  )
}

export default function App() {
  return (
    <CommandPaletteProvider>
      <Routes>
        <Route path="/login" element={<LazyRoute><Login /></LazyRoute>} />
        <Route path="/register" element={<LazyRoute><Register /></LazyRoute>} />
        <Route path="/forgot-password" element={<LazyRoute><ForgotPassword /></LazyRoute>} />
        <Route path="/reset-password" element={<LazyRoute><ResetPassword /></LazyRoute>} />
        <Route
          element={
            <ProtectedRoute>
              <ProtectedLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<Dashboard />} />
          <Route path="tunnels" element={<Tunnels />} />
          <Route path="tunnels/:id" element={<TunnelDetail />} />
          <Route path="domains" element={<Domains />} />
          <Route path="networks" element={<Networks />} />
          <Route path="networks/:id" element={<NetworkDetail />} />
          <Route path="settings" element={<Settings />} />
          <Route path="billing" element={<Billing />} />
          <Route path="notifications" element={<Notifications />} />
          <Route path="docs/api" element={<ApiDocs />} />
          <Route path="downloads" element={<Downloads />} />
        </Route>
        <Route
          path="/onboarding"
          element={
            <ProtectedRoute>
              <LazyRoute><Onboarding /></LazyRoute>
            </ProtectedRoute>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </CommandPaletteProvider>
  )
}
