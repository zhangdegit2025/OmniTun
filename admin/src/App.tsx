import { lazy, Suspense } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from '@/store/auth'
import AdminLayout from '@/components/AdminLayout'
import { Skeleton } from '@/components/ui/Skeleton'

const Dashboard = lazy(() => import('@/pages/Dashboard'))
const Organizations = lazy(() => import('@/pages/Organizations'))
const OrganizationDetail = lazy(() => import('@/pages/OrganizationDetail'))
const Users = lazy(() => import('@/pages/Users'))
const UserDetail = lazy(() => import('@/pages/UserDetail'))
const RelayNodes = lazy(() => import('@/pages/RelayNodes'))
const RelayNodeDetail = lazy(() => import('@/pages/RelayNodeDetail'))
const AuditLogs = lazy(() => import('@/pages/AuditLogs'))
const Announcements = lazy(() => import('@/pages/Announcements'))
const Certificates = lazy(() => import('@/pages/Certificates'))
const RevenueDashboard = lazy(() => import('@/pages/RevenueDashboard'))
const CustomerCRM = lazy(() => import('@/pages/CustomerCRM'))
const CustomerDetail = lazy(() => import('@/pages/CustomerDetail'))
const Invoices = lazy(() => import('@/pages/Invoices'))
const PricingConfig = lazy(() => import('@/pages/PricingConfig'))
const DiscountCodes = lazy(() => import('@/pages/DiscountCodes'))
const Roles = lazy(() => import('@/pages/Roles'))
const IPWhitelist = lazy(() => import('@/pages/IPWhitelist'))
const DataRetention = lazy(() => import('@/pages/DataRetention'))
const SLADashboard = lazy(() => import('@/pages/SLADashboard'))
const AuditReports = lazy(() => import('@/pages/AuditReports'))
const Login = lazy(() => import('@/pages/Login'))

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

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LazyRoute><Login /></LazyRoute>} />
      <Route
        element={
          <ProtectedRoute>
            <AdminLayout />
          </ProtectedRoute>
        }
      >
        <Route index element={<LazyRoute><Dashboard /></LazyRoute>} />
        <Route path="organizations" element={<LazyRoute><Organizations /></LazyRoute>} />
        <Route path="organizations/:id" element={<LazyRoute><OrganizationDetail /></LazyRoute>} />
        <Route path="users" element={<LazyRoute><Users /></LazyRoute>} />
        <Route path="users/:id" element={<LazyRoute><UserDetail /></LazyRoute>} />
        <Route path="relays" element={<LazyRoute><RelayNodes /></LazyRoute>} />
        <Route path="relays/:id" element={<LazyRoute><RelayNodeDetail /></LazyRoute>} />
        <Route path="audit-logs" element={<LazyRoute><AuditLogs /></LazyRoute>} />
        <Route path="announcements" element={<LazyRoute><Announcements /></LazyRoute>} />
        <Route path="certificates" element={<LazyRoute><Certificates /></LazyRoute>} />
        <Route path="revenue" element={<LazyRoute><RevenueDashboard /></LazyRoute>} />
        <Route path="customers" element={<LazyRoute><CustomerCRM /></LazyRoute>} />
        <Route path="customers/:id" element={<LazyRoute><CustomerDetail /></LazyRoute>} />
        <Route path="invoices" element={<LazyRoute><Invoices /></LazyRoute>} />
        <Route path="pricing" element={<LazyRoute><PricingConfig /></LazyRoute>} />
        <Route path="discounts" element={<LazyRoute><DiscountCodes /></LazyRoute>} />
        <Route path="roles" element={<LazyRoute><Roles /></LazyRoute>} />
        <Route path="ip-whitelist" element={<LazyRoute><IPWhitelist /></LazyRoute>} />
        <Route path="data-retention" element={<LazyRoute><DataRetention /></LazyRoute>} />
        <Route path="sla" element={<LazyRoute><SLADashboard /></LazyRoute>} />
        <Route path="audit-reports" element={<LazyRoute><AuditReports /></LazyRoute>} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
