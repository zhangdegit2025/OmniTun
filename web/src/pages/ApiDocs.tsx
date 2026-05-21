import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { apiRequest } from '@/lib/api'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { Skeleton } from '@/components/ui/Skeleton'
import { Button } from '@/components/ui/Button'
import {
  ChevronDown,
  ChevronRight,
  BookOpen,
  Copy,
  Check,
  AlertCircle,
} from 'lucide-react'
import { cn } from '@/lib/utils'

interface OpenAPISpec {
  openapi: string
  info: { title: string; version: string; description?: string }
  paths: Record<string, Record<string, PathOperation>>
  tags?: { name: string; description?: string }[]
}

interface PathOperation {
  summary?: string
  description?: string
  operationId?: string
  tags?: string[]
  parameters?: Parameter[]
  requestBody?: RequestBody
  responses: Record<string, Response>
  deprecated?: boolean
}

interface Parameter {
  name: string
  in: string
  description?: string
  required?: boolean
  schema: Schema
}

interface RequestBody {
  description?: string
  required?: boolean
  content: Record<string, { schema: Schema }>
}

interface Response {
  description: string
  content?: Record<string, { schema: Schema }>
}

interface Schema {
  type?: string
  format?: string
  description?: string
  properties?: Record<string, Schema>
  items?: Schema
  required?: string[]
  example?: unknown
  enum?: string[]
  allOf?: Schema[]
  $ref?: string
}

const methodColors: Record<string, string> = {
  get: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  post: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
  put: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
  patch: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
  delete: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
}

function generateCurl(method: string, path: string, params: Parameter[], body?: RequestBody): string {
  const baseUrl = 'https://api.omnitun.dev'
  let curl = `curl -X ${method.toUpperCase()} "${baseUrl}${path}"`

  const headerParams = params?.filter((p) => p.in === 'header') ?? []
  headerParams.forEach((p) => {
    curl += ` \\\n  -H "${p.name}: <${p.name}>"`
  })

  if (method !== 'get' && method !== 'delete') {
    curl += ` \\\n  -H "Content-Type: application/json"`
  }

  curl += ` \\\n  -H "Authorization: Bearer <token>"`

  const queryParams = params?.filter((p) => p.in === 'query') ?? []
  if (queryParams.length > 0) {
    const qs = queryParams.map((p) => `${p.name}=<${p.name}>`).join('&')
    curl = `curl -X ${method.toUpperCase()} "${baseUrl}${path}?${qs}"`
  }

  if (body && method !== 'get' && method !== 'delete') {
    const schema = body.content?.['application/json']?.schema
    if (schema?.properties) {
      const example: Record<string, unknown> = {}
      Object.entries(schema.properties).forEach(([key, prop]) => {
        example[key] = prop.example ?? `<${prop.type ?? 'string'}>`
      })
      curl += ` \\\n  -d '${JSON.stringify(example, null, 2)}'`
    }
  }

  return curl
}

function generateGoSnippet(method: string, path: string): string {
  const tag = path.split('/')[2]?.replace(/s$/, '') ?? 'resource'
  const svc = tag[0].toUpperCase() + tag.slice(1)

  const snippets: Record<string, string> = {
    'list-tunnels': `client, _ := omnitun.NewClient("<token>")
tunnels, err := client.Tunnels.List(ctx, nil)`,
    'create-tunnel': `client, _ := omnitun.NewClient("<token>")
tunnel, err := client.Tunnels.Create(ctx, omnitun.CreateTunnelOptions{
  Name:      "my-tunnel",
  Protocol:  "http",
  LocalPort: 3000,
})`,
    'get-tunnel': `client, _ := omnitun.NewClient("<token>")
tunnel, err := client.Tunnels.Get(ctx, "<tunnel-id>")`,
    'delete-tunnel': `client, _ := omnitun.NewClient("<token>")
err := client.Tunnels.Delete(ctx, "<tunnel-id>")`,
  }

  const key = `${method}-${tag}`
  return snippets[key] ?? `client, _ := omnitun.NewClient("<token>")
// TODO: call client.${svc}.${method[0].toUpperCase() + method.slice(1)}(ctx, ...)`
}

function generatePythonSnippet(method: string, path: string): string {
  const tag = path.split('/')[2]?.replace(/s$/, '') ?? 'resource'

  const snippets: Record<string, string> = {
    'list-tunnels': `from omnitun import OmniTun

client = OmniTun(token="<token>")
tunnels = client.tunnels.list()`,
    'create-tunnel': `from omnitun import OmniTun

client = OmniTun(token="<token>")
tunnel = client.tunnels.create(
    name="my-tunnel",
    protocol="http",
    local_port=3000,
)`,
    'get-tunnel': `from omnitun import OmniTun

client = OmniTun(token="<token>")
tunnel = client.tunnels.get("<tunnel-id>")`,
    'delete-tunnel': `from omnitun import OmniTun

client = OmniTun(token="<token>")
client.tunnels.delete("<tunnel-id>")`,
  }

  const key = `${method}-${tag}`
  return snippets[key] ?? `from omnitun import OmniTun

client = OmniTun(token="<token>")
# TODO: use client.${tag}s.${method}(...)`
}

function generateJSSnippet(method: string, path: string): string {
  return `const response = await fetch('https://api.omnitun.dev${path}', {
  method: '${method.toUpperCase()}',
  headers: {
    'Authorization': 'Bearer <token>',
    'Content-Type': 'application/json',
  }${method !== 'get' && method !== 'delete' ? `,
  body: JSON.stringify({
    // ... request body
  }),` : ''}
});
const data = await response.json();`
}

type CodeTab = 'curl' | 'go' | 'python' | 'javascript'

function CodeSample({
  method,
  path,
  params,
  body,
}: {
  method: string
  path: string
  params?: Parameter[]
  body?: RequestBody
}) {
  const [tab, setTab] = useState<CodeTab>('curl')
  const [copied, setCopied] = useState(false)

  const tabs: { key: CodeTab; label: string }[] = [
    { key: 'curl', label: 'cURL' },
    { key: 'go', label: 'Go' },
    { key: 'python', label: 'Python' },
    { key: 'javascript', label: 'JavaScript' },
  ]

  const code = useMemo(() => {
    switch (tab) {
      case 'curl':
        return generateCurl(method, path, params ?? [], body)
      case 'go':
        return generateGoSnippet(method, path)
      case 'python':
        return generatePythonSnippet(method, path)
      case 'javascript':
        return generateJSSnippet(method, path)
    }
  }, [tab, method, path, params, body])

  const handleCopy = () => {
    navigator.clipboard.writeText(code)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="mt-3 overflow-hidden rounded-md border">
      <div className="flex items-center justify-between bg-muted/50 px-3 py-1.5">
        <div className="flex gap-1">
          {tabs.map((t) => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={cn(
                'rounded px-2 py-0.5 text-xs font-medium transition-colors',
                tab === t.key
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {t.label}
            </button>
          ))}
        </div>
        <Button variant="ghost" size="sm" onClick={handleCopy}>
          {copied ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
        </Button>
      </div>
      <pre className="overflow-x-auto bg-background p-3 text-xs">
        <code>{code}</code>
      </pre>
    </div>
  )
}

function SchemaView({ schema, depth = 0 }: { schema?: Schema; depth?: number }) {
  if (!schema) return null
  if (depth > 4) return <span className="text-muted-foreground">...</span>

  if (schema.$ref) {
    const refName = schema.$ref.split('/').pop() ?? schema.$ref
    return <span className="font-mono text-primary">{refName}</span>
  }

  if (schema.allOf) {
    return (
      <div className="space-y-1">
        {schema.allOf.map((s, i) => (
          <SchemaView key={i} schema={s} depth={depth} />
        ))}
      </div>
    )
  }

  if (schema.enum) {
    return (
      <span>
        enum: [{schema.enum.map((v) => `"${v}"`).join(', ')}]
      </span>
    )
  }

  if (schema.type === 'array') {
    return (
      <span>
        [{schema.items ? <SchemaView schema={schema.items} depth={depth + 1} /> : 'any'}]
      </span>
    )
  }

  if (schema.type === 'object' && schema.properties) {
    return (
      <div className="ml-2 border-l-2 border-muted pl-3">
        {Object.entries(schema.properties).map(([key, prop]) => (
          <div key={key} className="py-0.5 text-xs">
            <span className="font-mono font-medium">{key}</span>
            {schema.required?.includes(key) && (
              <span className="ml-1 text-red-500">*</span>
            )}
            <span className="ml-2 text-muted-foreground">
              {prop.type}
              {prop.format && ` (${prop.format})`}
            </span>
            {prop.enum && (
              <span className="ml-1 text-muted-foreground">
                [{prop.enum.map((v) => `"${v}"`).join(' | ')}]
              </span>
            )}
            {prop.description && (
              <span className="ml-2 text-muted-foreground/70">{prop.description}</span>
            )}
            {prop.properties && <SchemaView schema={prop} depth={depth + 1} />}
            {prop.items && (
              <div className="ml-2">
                items: <SchemaView schema={prop.items} depth={depth + 1} />
              </div>
            )}
          </div>
        ))}
      </div>
    )
  }

  return (
    <span className="text-muted-foreground">
      {schema.type}
      {schema.format && ` (${schema.format})`}
    </span>
  )
}

function EndpointCard({
  method,
  path,
  operation,
}: {
  method: string
  path: string
  operation: PathOperation
}) {
  const [expanded, setExpanded] = useState(false)
  const upperMethod = method.toUpperCase()
  const badgeColor = methodColors[method] ?? 'bg-muted text-muted-foreground'

  return (
    <div className="border-b last:border-b-0">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-muted/30 transition-colors"
      >
        {expanded ? (
          <ChevronDown className="h-4 w-4 text-muted-foreground shrink-0" />
        ) : (
          <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />
        )}
        <Badge className={cn('font-mono text-xs uppercase shrink-0', badgeColor)}>
          {upperMethod}
        </Badge>
        <code className="font-mono text-sm font-medium flex-1">{path}</code>
        {operation.deprecated && (
          <Badge variant="secondary" className="text-xs">deprecated</Badge>
        )}
        <span className="text-xs text-muted-foreground shrink-0 hidden sm:inline">
          {operation.summary}
        </span>
      </button>

      {expanded && (
        <div className="space-y-4 border-t bg-muted/10 px-4 py-4">
          {operation.description && (
            <p className="text-sm text-muted-foreground">{operation.description}</p>
          )}

          {operation.parameters && operation.parameters.length > 0 && (
            <div>
              <h4 className="text-sm font-semibold mb-2">Parameters</h4>
              <div className="overflow-hidden rounded-md border">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b bg-muted/50">
                      <th className="px-3 py-1.5 text-left font-medium">Name</th>
                      <th className="px-3 py-1.5 text-left font-medium">In</th>
                      <th className="px-3 py-1.5 text-left font-medium hidden sm:table-cell">Type</th>
                      <th className="px-3 py-1.5 text-left font-medium">Required</th>
                      <th className="px-3 py-1.5 text-left font-medium hidden sm:table-cell">Description</th>
                    </tr>
                  </thead>
                  <tbody>
                    {operation.parameters.map((param) => (
                      <tr key={param.name} className="border-b last:border-b-0">
                        <td className="px-3 py-1.5 font-mono">{param.name}</td>
                        <td className="px-3 py-1.5">
                          <Badge variant="secondary" className="text-xs">{param.in}</Badge>
                        </td>
                        <td className="px-3 py-1.5 hidden sm:table-cell text-muted-foreground">
                          {param.schema.type}
                          {param.schema.format && ` (${param.schema.format})`}
                        </td>
                        <td className="px-3 py-1.5">
                          {param.required ? (
                            <span className="text-red-500">Yes</span>
                          ) : (
                            <span className="text-muted-foreground">No</span>
                          )}
                        </td>
                        <td className="px-3 py-1.5 hidden sm:table-cell text-muted-foreground">
                          {param.description ?? '-'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {operation.requestBody && (
            <div>
              <h4 className="text-sm font-semibold mb-2">
                Request Body
                {operation.requestBody.required && (
                  <span className="ml-1 text-red-500 text-xs">required</span>
                )}
              </h4>
              <div className="rounded-md border bg-background p-3">
                <SchemaView
                  schema={operation.requestBody.content?.['application/json']?.schema}
                />
              </div>
            </div>
          )}

          {operation.responses && Object.keys(operation.responses).length > 0 && (
            <div>
              <h4 className="text-sm font-semibold mb-2">Responses</h4>
              <div className="space-y-2">
                {Object.entries(operation.responses).map(([code, resp]) => (
                  <div key={code}>
                    <div className="flex items-center gap-2 text-xs">
                      <Badge
                        className={cn(
                          'shrink-0',
                          code.startsWith('2')
                            ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                            : code.startsWith('4')
                              ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
                              : 'bg-muted text-muted-foreground',
                        )}
                      >
                        {code}
                      </Badge>
                      <span className="text-muted-foreground">{resp.description}</span>
                    </div>
                    {resp.content?.['application/json']?.schema && (
                      <div className="mt-1 ml-1 rounded border bg-background p-2">
                        <SchemaView
                          schema={resp.content['application/json'].schema}
                        />
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          <CodeSample method={method} path={path} params={operation.parameters} body={operation.requestBody} />
        </div>
      )}
    </div>
  )
}

function ApiDocsSkeleton() {
  return (
    <div className="space-y-6 p-6">
      <Skeleton className="h-8 w-48" />
      <Skeleton className="h-4 w-96" />
      <div className="space-y-3">
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
    </div>
  )
}

const mockSpec: OpenAPISpec = {
  openapi: '3.0.3',
  info: {
    title: 'OmniTun API',
    version: 'v1',
    description: 'Enterprise Network Tunneling Platform REST API. Create and manage tunnels, domains, and mesh networks programmatically.',
  },
  tags: [
    { name: 'Tunnels', description: 'Create and manage tunnels' },
    { name: 'Domains', description: 'Custom domain management' },
    { name: 'Networks', description: 'Mesh network operations' },
    { name: 'Auth', description: 'Authentication and API keys' },
  ],
  paths: {
    '/v1/tunnels': {
      get: {
        summary: 'List tunnels',
        operationId: 'listTunnels',
        tags: ['Tunnels'],
        parameters: [
          { name: 'status', in: 'query', schema: { type: 'string', enum: ['active', 'stopped', 'error'] } },
          { name: 'protocol', in: 'query', schema: { type: 'string', enum: ['tcp', 'http', 'https'] } },
          { name: 'page', in: 'query', schema: { type: 'integer' } },
          { name: 'per_page', in: 'query', schema: { type: 'integer' } },
        ],
        responses: {
          '200': { description: 'Paginated list of tunnels' },
          '401': { description: 'Unauthorized' },
        },
      },
      post: {
        summary: 'Create a tunnel',
        operationId: 'createTunnel',
        tags: ['Tunnels'],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: {
                type: 'object',
                required: ['name', 'protocol', 'local_port'],
                properties: {
                  name: { type: 'string', description: 'Tunnel name' },
                  protocol: { type: 'string', enum: ['tcp', 'http', 'https'] },
                  local_port: { type: 'integer', description: 'Local port to forward' },
                  remote_port: { type: 'integer' },
                  domain: { type: 'string' },
                  tags: { type: 'array', items: { type: 'string' } },
                  auth_mode: { type: 'string', enum: ['none', 'basic', 'oauth'] },
                  tls_mode: { type: 'string', enum: ['off', 'flexible', 'strict'] },
                  compression: { type: 'boolean' },
                },
              },
            },
          },
        },
        responses: {
          '201': { description: 'Tunnel created' },
          '400': { description: 'Validation error' },
          '401': { description: 'Unauthorized' },
        },
      },
    },
    '/v1/tunnels/{id}': {
      get: {
        summary: 'Get tunnel by ID',
        operationId: 'getTunnel',
        tags: ['Tunnels'],
        parameters: [
          { name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } },
        ],
        responses: {
          '200': { description: 'Tunnel details' },
          '404': { description: 'Tunnel not found' },
        },
      },
      patch: {
        summary: 'Update tunnel',
        operationId: 'updateTunnel',
        tags: ['Tunnels'],
        parameters: [
          { name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } },
        ],
        responses: {
          '200': { description: 'Tunnel updated' },
          '404': { description: 'Tunnel not found' },
        },
      },
      delete: {
        summary: 'Delete tunnel',
        operationId: 'deleteTunnel',
        tags: ['Tunnels'],
        parameters: [
          { name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } },
        ],
        responses: {
          '204': { description: 'Tunnel deleted' },
          '404': { description: 'Tunnel not found' },
        },
      },
    },
    '/v1/tunnels/{id}/start': {
      post: {
        summary: 'Start tunnel',
        operationId: 'startTunnel',
        tags: ['Tunnels'],
        parameters: [
          { name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } },
        ],
        responses: {
          '200': { description: 'Tunnel started' },
          '400': { description: 'Tunnel already running' },
          '404': { description: 'Tunnel not found' },
        },
      },
    },
    '/v1/tunnels/{id}/stop': {
      post: {
        summary: 'Stop tunnel',
        operationId: 'stopTunnel',
        tags: ['Tunnels'],
        parameters: [
          { name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } },
        ],
        responses: {
          '200': { description: 'Tunnel stopped' },
          '404': { description: 'Tunnel not found' },
        },
      },
    },
    '/v1/domains': {
      get: {
        summary: 'List domains',
        operationId: 'listDomains',
        tags: ['Domains'],
        responses: { '200': { description: 'Paginated list of domains' } },
      },
      post: {
        summary: 'Add custom domain',
        operationId: 'createDomain',
        tags: ['Domains'],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: {
                type: 'object',
                required: ['domain'],
                properties: {
                  domain: { type: 'string', description: 'Custom domain name' },
                  tunnel_id: { type: 'string', format: 'uuid' },
                },
              },
            },
          },
        },
        responses: {
          '201': { description: 'Domain added' },
          '409': { description: 'Domain already exists' },
        },
      },
    },
    '/v1/domains/{id}': {
      get: {
        summary: 'Get domain',
        operationId: 'getDomain',
        tags: ['Domains'],
        parameters: [
          { name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } },
        ],
        responses: { '200': { description: 'Domain details' } },
      },
      delete: {
        summary: 'Remove domain',
        operationId: 'deleteDomain',
        tags: ['Domains'],
        parameters: [
          { name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } },
        ],
        responses: { '204': { description: 'Domain removed' } },
      },
    },
    '/v1/networks': {
      get: {
        summary: 'List networks',
        operationId: 'listNetworks',
        tags: ['Networks'],
        responses: { '200': { description: 'Paginated list of mesh networks' } },
      },
      post: {
        summary: 'Create mesh network',
        operationId: 'createNetwork',
        tags: ['Networks'],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: {
                type: 'object',
                required: ['name', 'cidr'],
                properties: {
                  name: { type: 'string' },
                  cidr: { type: 'string', example: '10.0.0.0/24' },
                },
              },
            },
          },
        },
        responses: { '201': { description: 'Network created' } },
      },
    },
    '/v1/networks/{id}': {
      get: {
        summary: 'Get network',
        operationId: 'getNetwork',
        tags: ['Networks'],
        parameters: [
          { name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } },
        ],
        responses: { '200': { description: 'Network details' } },
      },
      delete: {
        summary: 'Delete network',
        operationId: 'deleteNetwork',
        tags: ['Networks'],
        parameters: [
          { name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } },
        ],
        responses: { '204': { description: 'Network deleted' } },
      },
    },
    '/v1/networks/join': {
      post: {
        summary: 'Join network by invite',
        operationId: 'joinNetwork',
        tags: ['Networks'],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: {
                type: 'object',
                required: ['invite_code'],
                properties: {
                  invite_code: { type: 'string' },
                },
              },
            },
          },
        },
        responses: { '200': { description: 'Joined network successfully' } },
      },
    },
    '/v1/networks/{id}/leave': {
      post: {
        summary: 'Leave network',
        operationId: 'leaveNetwork',
        tags: ['Networks'],
        parameters: [
          { name: 'id', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } },
        ],
        responses: { '204': { description: 'Left network' } },
      },
    },
    '/v1/auth/refresh': {
      post: {
        summary: 'Refresh access token',
        operationId: 'refreshToken',
        tags: ['Auth'],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: {
                type: 'object',
                required: ['refresh_token'],
                properties: {
                  refresh_token: { type: 'string' },
                },
              },
            },
          },
        },
        responses: {
          '200': { description: 'New token pair' },
          '401': { description: 'Invalid refresh token' },
        },
      },
    },
  },
}

export default function ApiDocs() {
  const { t } = useTranslation()

  const specQuery = useQuery<OpenAPISpec>({
    queryKey: ['openapi', 'spec'],
    queryFn: () => apiRequest<OpenAPISpec>('/v1/openapi.json'),
    retry: 1,
  })

  const spec = specQuery.data ?? mockSpec
  const isLoading = specQuery.isLoading
  const isError = specQuery.isError && !specQuery.data

  const groupedPaths = useMemo(() => {
    const groups: Record<string, { path: string; method: string; op: PathOperation }[]> = {}
    Object.entries(spec.paths).forEach(([path, methods]) => {
      Object.entries(methods).forEach(([method, op]) => {
        const tag = op.tags?.[0] ?? 'Other'
        if (!groups[tag]) groups[tag] = []
        groups[tag].push({ path, method, op })
      })
    })
    return groups
  }, [spec])

  if (isLoading) return <ApiDocsSkeleton />

  return (
    <div className="space-y-6 p-6">
      <div>
        <div className="flex items-center gap-3">
          <BookOpen className="h-6 w-6 text-primary" />
          <h1 className="text-2xl font-bold">{t('apidocs.title', 'API Reference')}</h1>
        </div>
        <p className="mt-1 text-sm text-muted-foreground">
          {t('apidocs.subtitle', 'OmniTun REST API v1 — programmatic access to tunnels, domains, and mesh networks.')}
        </p>
      </div>

      {isError && (
        <div className="flex flex-col items-center gap-2 rounded-lg border p-6 text-center">
          <AlertCircle className="h-8 w-8 text-destructive" />
          <p className="text-sm text-destructive">{t('apidocs.failed_load', 'Failed to load API spec. Showing reference documentation.')}</p>
          <Button variant="outline" size="sm" onClick={() => specQuery.refetch()}>
            {t('common.retry')}
          </Button>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-[240px_1fr]">
        <aside className="hidden lg:block">
          <nav className="sticky top-20 space-y-1">
            <p className="mb-2 text-xs font-semibold uppercase text-muted-foreground">
              {t('apidocs.endpoints', 'Endpoints')}
            </p>
            {Object.keys(groupedPaths).map((tag) => (
              <a
                key={tag}
                href={`#tag-${tag.toLowerCase().replace(/\s+/g, '-')}`}
                className="block rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
              >
                {tag}
              </a>
            ))}
          </nav>
        </aside>

        <div className="space-y-6">
          {Object.entries(groupedPaths).map(([tag, endpoints]) => (
            <Card key={tag} id={`tag-${tag.toLowerCase().replace(/\s+/g, '-')}`}>
              <CardHeader>
                <CardTitle>{tag}</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                {endpoints.map(({ path, method, op }) => (
                  <EndpointCard
                    key={`${method}-${path}`}
                    method={method}
                    path={path}
                    operation={op}
                  />
                ))}
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </div>
  )
}
