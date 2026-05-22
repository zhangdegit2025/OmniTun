export interface AdminUser {
  id: string
  email: string
  name: string
  role: 'super_admin' | 'admin' | 'operator'
  created_at: string
}

export interface Organization {
  id: string
  name: string
  email: string
  plan: string
  status: 'active' | 'suspended' | 'trial'
  tunnel_count: number
  user_count: number
  bandwidth_used: number
  bandwidth_limit: number
  created_at: string
}

export interface OrgStats {
  total_orgs: number
  active_orgs: number
  trial_orgs: number
  suspended_orgs: number
}

export interface User {
  id: string
  email: string
  name: string
  org_id: string
  org_name: string
  role: string
  status: 'active' | 'disabled'
  created_at: string
  last_login_at?: string
}

export interface RelayNode {
  id: string
  name: string
  hostname: string
  region: string
  status: 'online' | 'offline' | 'maintenance'
  version: string
  cpu_usage: number
  memory_usage: number
  bandwidth_in: number
  bandwidth_out: number
  active_connections: number
  last_seen_at: string
  created_at: string
}

export interface RelayStats {
  total_nodes: number
  online_nodes: number
  total_bandwidth_in: number
  total_bandwidth_out: number
  total_connections: number
}

export interface DashboardStats {
  total_orgs: number
  active_tunnels: number
  today_traffic_bytes: number
  active_relays: number
}

export interface RecentSignup {
  id: string
  org_name: string
  email: string
  plan: string
  created_at: string
}

export interface SystemHealth {
  component: string
  status: 'healthy' | 'degraded' | 'down'
  uptime_percent: number
  last_checked: string
}

export interface Tunnel {
  id: string
  name: string
  org_id: string
  protocol: string
  local_port: number
  remote_port: number
  domain?: string
  status: 'active' | 'stopped' | 'error'
  traffic_in: number
  traffic_out: number
  created_at: string
}

export interface TrafficPoint {
  timestamp: string
  bytes_in: number
  bytes_out: number
}

export interface AuditLog {
  id: string
  org_id: string
  user_id: string
  action: string
  resource_type: string
  resource_id: string
  details: string
  client_ip: string
  created_at: string
}

export interface Announcement {
  id: string
  title: string
  body: string
  severity: 'info' | 'warning' | 'critical'
  target: string
  active: boolean
  start_at: string | null
  end_at: string | null
  created_at: string
  updated_at: string
}

export interface SystemCertificate {
  id: string
  domain: string
  issuer: string
  not_before: string
  not_after: string
  days_remaining: number
  auto_renew: boolean
  status: 'valid' | 'expiring_soon' | 'expired'
}

export interface TenantCertificate {
  id: string
  domain: string
  issuer: string
  org_id: string
  org_name: string
  not_before: string
  not_after: string
  days_remaining: number
  auto_renew: boolean
  status: 'valid' | 'expiring_soon' | 'expired'
}

export interface Invoice {
  id: string
  customer: string
  amount: number
  subtotal: number
  tax: number
  status: 'paid' | 'pending' | 'overdue' | 'void'
  date: string
  due_date: string
  payment_method: string
  items: InvoiceItem[]
  organization?: { id: string; name: string }
}

export interface InvoiceItem {
  description: string
  quantity: number
  unit_price: number
  total: number
}

export interface PricingPlan {
  id: string
  name: string
  price_monthly_usd: number
  max_tunnels: number
  max_bandwidth_gb: number
  features: string[]
}

export interface DiscountCode {
  id: string
  code: string
  type: 'percentage' | 'fixed'
  value: number
  uses: number
  max_uses: number
  active: boolean
  expires_at: string
  created_at: string
  applicable_plans: string
}

export interface MRRData {
  mrr: number
  arr: number
  active_subscriptions: number
  churn_rate: number
  trend: MRRTrendPoint[]
  forecast: RevenueForecast
}

export interface MRRTrendPoint {
  month: string
  mrr: number
  new: number
  expansion: number
  contraction: number
  churn: number
}

export interface RevenueForecast {
  days_30: number
  days_60: number
  days_90: number
}

export interface ChurnData {
  churn_rate: number
  voluntary_churn: number
  involuntary_churn: number
  retention_rate: number
  at_risk_customers: number
  monthly_trend: { month: string; rate: number }[]
}

export interface FunnelData {
  stages: FunnelStage[]
}

export interface FunnelStage {
  name: string
  count: number
  rate: number
}

export interface Customer {
  id: string
  org_name: string
  plan: string
  mrr: number
  status: string
  health_score: number
  created_at: string
  contact_email: string
  user_count: number
  tunnel_count: number
}

export interface CustomerListResponse {
  customers: Customer[]
  total: number
}

export interface CustomerDetail extends Customer {
  contacts: { name: string; email: string; role: string }[]
  usage: {
    tunnels: number
    bandwidth_gb: number
    active_users: number
    connections: number
  }
  invoices: {
    id: string
    amount: number
    status: string
    date: string
    plan: string
  }[]
  subscriptions: {
    id: string
    plan: string
    status: string
    start: string
    end: string
  }[]
  activity: {
    timestamp: string
    action: string
    detail: string
  }[]
  health_history: {
    date: string
    score: number
  }[]
}

export interface CustomerHealth {
  customer_id: string
  health_score: number
  trend: { date: string; score: number }[]
  factors: { factor: string; score: number; weight: number }[]
}

export interface CustomRole {
  id: string
  name: string
  permissions: string[]
  assigned_users: number
  created_at: string
  updated_at: string
}

export interface IPWhitelistEntry {
  id: string
  org_id: string
  cidr: string
  label: string
  created_at: string
}

export interface IPWhitelistConfig {
  entries: IPWhitelistEntry[]
  enabled: boolean
}

export interface RetentionSettings {
  audit_logs: string
  traffic_events: string
  user_sessions: string
}

export interface RetentionUsage {
  audit_logs: { count: number; size_bytes: number; size_mb: number }
  traffic_events: { count: number; size_bytes: number; size_mb: number }
  user_sessions: { count: number; size_bytes: number; size_mb: number }
}

export interface RetentionData {
  settings: RetentionSettings
  usage: RetentionUsage
  plan_limits: Record<string, string>
  last_cleanup: string
}

export interface SLAUptimeData {
  current_month: {
    api_uptime: number
    tunnel_control_plane: number
    relay_data_plane: number
  }
  previous_month: {
    api_uptime: number
    tunnel_control_plane: number
    relay_data_plane: number
  }
  monthly_trend: {
    month: string
    api_uptime: number
    control_plane: number
    data_plane: number
  }[]
}

export interface SLAIncident {
  id: string
  date: string
  duration: string
  duration_minutes: number
  impact: string
  root_cause: string
  status: string
  affected_services: string[]
}

export interface SLACreditData {
  current_month: {
    sla_breached: boolean
    credits_owed: number
    breach_threshold: number
    actual_uptime: number
  }
  history: {
    month: string
    uptime: number
    sla_met: boolean
    credits: number
  }[]
  total_credits_ytd: number
  sla_thresholds: Record<string, number>
  credit_formula: Record<string, string>
}

export interface AuditReportTemplate {
  id: string
  name: string
  description: string
}

export interface AuditReportHistory {
  id: string
  template: string
  generated_by: string
  generated_at: string
  format: string
  date_range: { from: string; to: string }
  org_filter: string
  size_bytes: number
}
