const SECURITY_EVENT = 'btg:security-signal'

type SecuritySignal = {
  reason: 'debug-pause' | 'devtools-shortcut' | 'context-menu'
  detectedAt: number
}

function emitSecuritySignal(reason: SecuritySignal['reason']) {
  window.dispatchEvent(new CustomEvent<SecuritySignal>(SECURITY_EVENT, {
    detail: { reason, detectedAt: Date.now() },
  }))
}

let debuggerProbe = () => {}
try {
  // Constructing the probe at runtime prevents the production minifier from
  // stripping the debugger statement. A restrictive CSP simply disables this
  // one signal; shortcut blocking and watermark self-healing still operate.
  debuggerProbe = Function('debugger') as () => void
} catch {
  // Dynamic code execution may be disabled by the host policy.
}

export function installRuntimeGuards() {
  if (!import.meta.env.PROD) return

  const onKeyDown = (event: KeyboardEvent) => {
    const key = event.key.toLowerCase()
    const blocked = event.key === 'F12'
      || (event.ctrlKey && event.shiftKey && ['i', 'j', 'c'].includes(key))
      || (event.ctrlKey && key === 'u')

    if (!blocked) return
    event.preventDefault()
    event.stopImmediatePropagation()
    emitSecuritySignal('devtools-shortcut')
  }

  const onContextMenu = (event: MouseEvent) => {
    event.preventDefault()
    emitSecuritySignal('context-menu')
  }

  window.addEventListener('keydown', onKeyDown, { capture: true })
  window.addEventListener('contextmenu', onContextMenu, { capture: true })

  window.setInterval(() => {
    const startedAt = performance.now()
    debuggerProbe()
    if (performance.now() - startedAt > 700) {
      emitSecuritySignal('debug-pause')
    }
  }, 4_000)
}

export { SECURITY_EVENT }
