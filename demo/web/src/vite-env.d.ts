/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL: string
  /** Set to `"true"` to send browser traces to OTLP (see TRACING.md bước 6). */
  readonly VITE_OTEL_TRACING_ENABLED?: string
  /** OTLP HTTP traces URL; default same-origin `/otel/v1/traces` (Vite proxy → Jaeger). */
  readonly VITE_OTEL_EXPORTER_URL?: string
  /** `"true"` → root spans use ratio sampling (`VITE_OTEL_TRACE_SAMPLE_RATIO`, default 0.1). */
  readonly VITE_OTEL_TRACE_SAMPLING_ENABLED?: string
  /** Fraction [0,1] for browser root spans when sampling enabled. */
  readonly VITE_OTEL_TRACE_SAMPLE_RATIO?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
