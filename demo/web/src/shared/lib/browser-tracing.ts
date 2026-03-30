import type { Span } from '@opentelemetry/api'
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http'
import { registerInstrumentations } from '@opentelemetry/instrumentation'
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load'
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch'
import { XMLHttpRequestInstrumentation } from '@opentelemetry/instrumentation-xml-http-request'
import { resourceFromAttributes } from '@opentelemetry/resources'
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base'
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web'

import { getApiBase } from '@/shared/config/env'

/** Human-readable span name for Jaeger (aligned with Go httpSpanName: "GET /videos"). */
export function formatClientHttpSpanName(
  method: string,
  absoluteUrl: string,
  baseForRelative?: string,
): string {
  const u = new URL(absoluteUrl, baseForRelative ?? window.location.href)
  const path = `${u.pathname}${u.search || ''}` || '/'
  return `${method.toUpperCase()} ${path}`
}

function applyClientHttpSpanName(
  span: Span,
  method: string,
  absoluteUrl: string,
): void {
  try {
    span.updateName(formatClientHttpSpanName(method, absoluteUrl))
  } catch {
    /* ignore invalid URL */
  }
}

function fetchMethod(request: Request | RequestInit): string {
  if (typeof Request !== 'undefined' && request instanceof Request) {
    return request.method
  }
  const m = (request as RequestInit).method
  if (typeof m === 'string' && m.trim() !== '') {
    return m.trim().toUpperCase()
  }
  return 'GET'
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function apiOriginRegex(): RegExp | null {
  try {
    const origin = new URL(getApiBase()).origin
    return new RegExp(`^${escapeRegExp(origin)}`)
  } catch {
    return null
  }
}

function tracesEndpoint(): string {
  const raw = import.meta.env.VITE_OTEL_EXPORTER_URL
  if (typeof raw === 'string' && raw.trim() !== '') {
    return raw.trim()
  }
  return `${window.location.origin}/otel/v1/traces`
}

/** Registers W3C propagation + fetch/XHR spans; exports OTLP via Vite proxy in dev. */
export function initBrowserTracingIfEnabled(): void {
  if (import.meta.env.VITE_OTEL_TRACING_ENABLED !== 'true') {
    return
  }

  const propagate = apiOriginRegex()
  const propagateTraceHeaderCorsUrls = propagate ? [propagate] : []
  const ignoreTelemetry: Array<string | RegExp> = [/otel\/v1\/traces/]

  const exporter = new OTLPTraceExporter({ url: tracesEndpoint() })
  const resource = resourceFromAttributes({
    'service.name': 'video-web',
  })

  const provider = new WebTracerProvider({
    resource,
    spanProcessors: [new BatchSpanProcessor(exporter)],
  })
  provider.register()

  registerInstrumentations({
    instrumentations: [
      new DocumentLoadInstrumentation(),
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls,
        ignoreUrls: ignoreTelemetry,
        // `fetch(url, init)` passes only `init` into requestHook — set name after response when we have final URL.
        requestHook(span, req) {
          if (typeof Request !== 'undefined' && req instanceof Request) {
            applyClientHttpSpanName(span, req.method, req.url)
          }
        },
        applyCustomAttributesOnSpan(span, request, result) {
          const method = fetchMethod(request)
          let href = ''
          if (typeof Response !== 'undefined' && result instanceof Response) {
            href = result.url
          } else if (
            result &&
            typeof result === 'object' &&
            'url' in result &&
            typeof (result as { url?: unknown }).url === 'string'
          ) {
            href = (result as { url: string }).url
          }
          if (!href && typeof Request !== 'undefined' && request instanceof Request) {
            href = request.url
          }
          if (href) {
            applyClientHttpSpanName(span, method, href)
          }
        },
      }),
      new XMLHttpRequestInstrumentation({
        propagateTraceHeaderCorsUrls,
        ignoreUrls: ignoreTelemetry,
        // XHR has no standard `method` on the object; HLS segment loads are GET.
        applyCustomAttributesOnSpan(span, xhr) {
          const href = xhr.responseURL
          if (href) {
            applyClientHttpSpanName(span, 'GET', href)
          }
        },
      }),
    ],
  })
}
