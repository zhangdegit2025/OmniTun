export function prefetchPage(page: string) {
  switch (page) {
    case 'dashboard':
      return import('../pages/Dashboard')
    case 'tunnels':
      return import('../pages/Tunnels')
    case 'domains':
      return import('../pages/Domains')
    case 'networks':
      return import('../pages/Networks')
    case 'billing':
      return import('../pages/Billing')
    case 'settings':
      return import('../pages/Settings')
    case 'notifications':
      return import('../pages/Notifications')
  }
}
