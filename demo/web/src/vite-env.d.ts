/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL: string
  /** Set to `"true"` to send browser traces to OTLP (see TRACING.md bước 6). */
  readonly VITE_OTEL_TRACING_ENABLED?: string
  /** OTLP HTTP traces URL; default same-origin `/otel/v1/traces` (Vite proxy → Jaeger). */
  readonly VITE_OTEL_EXPORTER_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
