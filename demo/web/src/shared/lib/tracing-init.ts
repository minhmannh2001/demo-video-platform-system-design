// Side-effect module: import this first from main so fetch/XHR are patched before any API calls.
import { initBrowserTracingIfEnabled } from '@/shared/lib/browser-tracing'

initBrowserTracingIfEnabled()
