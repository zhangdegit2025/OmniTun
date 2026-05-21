export interface User {
  id: string
  email: string
  name: string
  display_name?: string
  org_id: string
  role: 'admin' | 'member'
  mfa_enabled?: boolean
  onboarding_completed?: boolean
  created_at: string
}

export interface Tunnel {
  id: string
  name: string
  protocol: 'tcp' | 'http' | 'https'
  local_port: number
  remote_port: number
  domain?: string
  status: 'active' | 'stopped' | 'error'
  traffic_in: number
  traffic_out: number
  tags?: string[]
  auth_mode?: string
  tls_mode?: string
  compression?: boolean
  created_at: string
  updated_at: string
}

export interface TunnelStats {
  active_tunnels: number
  total_tunnels: number
  total_traffic_in: number
  total_traffic_out: number
  active_connections: number
  today_requests: number
}

export interface RecentEvent {
  id: string
  type: string
  tunnel_id?: string
  tunnel_name?: string
  message: string
  status: 'created' | 'started' | 'stopped' | 'error'
  created_at: string
}

export interface ApiKey {
  id: string
  name: string
  key_prefix: string
  status: 'active' | 'revoked'
  created_at: string
  last_used_at?: string
}

export interface ConnectionLogEntry {
  id: string
  timestamp: string
  client_ip: string
  method: string
  path: string
  status_code: number
  bytes_sent: number
  duration_ms: number
}

export interface TunnelConfig {
  auth_mode: 'none' | 'basic' | 'oauth'
  max_connections: number
}

export interface TrafficPoint {
  timestamp: string
  bytes_in: number
  bytes_out: number
}

export interface OrgUsage {
  bandwidth_used: number
  bandwidth_limit: number
  tunnels_used: number
  tunnels_limit: number
}

export interface BillingPlan {
  id: string
  name: string
  price_monthly_usd: number
  max_tunnels: number
  max_bandwidth_gb: number
  features: string[]
}

export interface BillingUsage {
  plan: string
  tunnels_used: number
  tunnels_limit: number
  bandwidth_bytes: number
  bandwidth_limit: number
  bandwidth_gb: number
  bandwidth_limit_gb: number
  period_start: string
  period_end: string
}

export interface Invoice {
  id: string
  plan_id: string
  amount_usd: number
  status: string
  created_at: string
}

export interface BillingPlan {
  id: string
  name: string
  price_monthly_usd: number
  max_tunnels: number
  max_bandwidth_gb: number
  features: string[]
}

export interface BillingUsage {
  plan: string
  tunnels_used: number
  tunnels_limit: number
  bandwidth_bytes: number
  bandwidth_limit: number
  bandwidth_gb: number
  bandwidth_limit_gb: number
  period_start: string
  period_end: string
}

export interface Invoice {
  id: string
  plan_id: string
  amount_usd: number
  status: string
  created_at: string
}

export interface MeshNetwork {
  id: string
  name: string
  cidr: string
  node_count: number
  created_at: string
}

export interface MeshNode {
  id: string
  network_id: string
  name: string
  ip_address: string
  public_key: string
  nat_type: string
  endpoints: string[]
  status: 'online' | 'offline'
  last_seen_at: string
  created_at: string
}

export interface MeshInvite {
  id: string
  network_id: string
  code: string
  created_at: string
}

export interface Invitation {
  id: string
  code: string
  max_uses: number
  use_count: number
  expires_at?: string
  created_at: string
}

export interface OrgEvent {
  id: string
  user_id: string
  user_name: string
  user_email: string
  action: string
  resource_type: string
  resource_id: string
  created_at: string
}

export interface Session {
  id: string
  current: boolean
  browser: string
  os: string
  ip: string
  location: string
  last_active: string
  created_at: string
}

export interface BatchResult {
  success: number
  total: number
  failures: number
}

export interface TunnelTemplate {
  id: string
  name: string
  icon: string
  protocol: Tunnel['protocol']
  local_port: number
  tls_mode: string
  compression: boolean
  description: string
}

export interface InspectEntry {
  id: string
  timestamp: string
  method: string
  path: string
  status_code: number
  duration_ms: number
  client_ip: string
  request_headers?: Record<string, string>
  request_body?: string
  response_headers?: Record<string, string>
  response_body?: string
}

export interface Webhook {
  id: string
  name: string
  url: string
  events: string[]
  secret: string
  status: 'active' | 'failed' | 'disabled'
  last_delivery_at?: string
  created_at: string
}

export interface WebhookDelivery {
  id: string
  webhook_id: string
  event: string
  status: 'success' | 'failed' | 'pending'
  status_code?: number
  duration_ms: number
  retry_count: number
  request_headers?: Record<string, string>
  request_body?: string
  response_headers?: Record<string, string>
  response_body?: string
  created_at: string
}
