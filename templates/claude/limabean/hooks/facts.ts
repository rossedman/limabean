// Pure parsing and wording: everything here takes text or data and returns
// data or text, so tests cover it without a VM or a Mac.

import type { ImageRow, InsideVM, MacView, PortRow, VMRow } from '../types'

/** Reads network.conf: `allow <domain>`, `locked` and `learn` lines. */
export function parsePolicy(text: string): Pick<InsideVM, 'allow' | 'locked' | 'learning'> {
  const allow: string[] = []
  let locked = false
  let learning = false
  for (const line of text.split('\n')) {
    const [word, arg] = line.trim().split(/\s+/)
    if (word === 'allow' && arg) allow.push(arg.toLowerCase())
    if (word === 'locked') locked = true
    if (word === 'learn') learning = true
  }
  return { allow: [...new Set(allow)].sort(), locked, learning }
}

/** Pack names from /etc/limabean/mise's files: 50-rust.toml is rust; 00-core is left out. */
export function packsFromFiles(names: readonly string[]): string[] {
  return names
    .map(n => /^\d+-([a-z0-9-]+)\.toml$/.exec(n)?.[1])
    .filter((n): n is string => n !== undefined && n !== 'core')
    .sort()
}

/** The VM's name from its hostname, lima-<name>. */
export function vmName(hostname: string): string {
  const h = hostname.trim()
  return h.startsWith('lima-') ? h.slice(5) : h
}

/** The system prompt section that tells Claude where it runs. */
export function insideSection(vm: InsideVM): string {
  const lines = [
    `You are running inside the limabean VM "${vm.name}", an Ubuntu VM on the user's Mac. Work happens here, not on the Mac.`,
    `- Repositories live under ${vm.codeDir} (ghq layout: <host>/<owner>/<repo>). Push work to a remote before it matters: the VM can be rebuilt or deleted.`,
    `- Tools come from packs (mise): core${vm.packs.length > 0 ? ', ' + vm.packs.join(', ') : ''}. \`mise use <tool>@<version>\` adds others for one project.`,
  ]
  if (vm.learning) {
    lines.push('- The network is open while it learns: every DNS lookup is recorded to build this VM\'s allowlist later.')
  } else if (vm.allow.length > 0) {
    lines.push(`- The network is an allowlist: only ${vm.allow.join(', ')} and their subdomains resolve. Anything else fails with "Could not resolve host". Don't try other hosts or mirrors; tell the user which domain is needed, and that they can allow it on the Mac with: limabean network allow ${vm.name} <domain>`)
  } else {
    lines.push('- The network is open.')
  }
  if (vm.locked) {
    lines.push('- The VM is locked: there is no sudo and no Docker for this user. Don\'t try either; install tools in your home directory (mise, pip --user, npm prefixes) or ask the user.')
  }
  lines.push('- Files on the Mac aren\'t reachable from here. The user moves files with `limabean cp` on the Mac.')
  return lines.join('\n')
}

/** One line for the band and the status line inside a VM. */
export function insideLine(vm: InsideVM): string {
  const parts = [`limabean · ${vm.name}`]
  if (vm.learning) parts.push('learning')
  else if (vm.allow.length > 0) parts.push(`${vm.allow.length} domain${vm.allow.length === 1 ? '' : 's'}`)
  else parts.push('open network')
  if (vm.locked) parts.push('locked')
  return parts.join(' · ')
}

/** The host a failed command couldn't resolve, if its output says so. */
export function blockedHost(output: string): string | undefined {
  const patterns = [
    /Could not resolve host:?\s*'?([A-Za-z0-9.-]+\.[A-Za-z]{2,})/,
    /getaddrinfo (?:ENOTFOUND|EAI_AGAIN) ([A-Za-z0-9.-]+\.[A-Za-z]{2,})/,
    /Failed to resolve '([A-Za-z0-9.-]+\.[A-Za-z]{2,})'/,
    /Temporary failure in name resolution.*?([A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+\.[A-Za-z]{2,})/,
    /lookup ([A-Za-z0-9.-]+\.[A-Za-z]{2,})(?: on \S+)?: no such host/,
    /Name or service not known.*?([A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+\.[A-Za-z]{2,})/,
  ]
  for (const re of patterns) {
    const host = re.exec(output)?.[1]
    if (host) return host.toLowerCase()
  }
  return undefined
}

/** Whether an allowlist lets a host through (a domain allows its subdomains). */
export function allows(allow: readonly string[], host: string): boolean {
  return allow.some(d => host === d || host.endsWith('.' + d))
}

/** The note Claude reads after a command hit a host the allowlist blocks. */
export function blockedNote(vm: InsideVM, host: string): string {
  return `limabean: ${host} isn't on the allowlist of VM "${vm.name}" (it allows ${vm.allow.join(', ')}), so the lookup failed. Retrying won't help. Tell the user, who can allow it on the Mac with: limabean network allow ${vm.name} ${host}`
}

/** Parses JSON output, or returns the fallback. */
export function parseJSON<T>(text: string, fallback: T): T {
  try {
    const v = JSON.parse(text) as unknown
    return (v ?? fallback) as T
  } catch {
    return fallback
  }
}

/** The status line on the Mac. */
export function macLine(view: MacView): string {
  if (view.error) return `limabean: ${view.error}`
  const running = view.vms.filter(v => v.status === 'Running').length
  const parts = [`limabean: ${running} running`]
  const stopped = view.vms.length - running
  if (stopped > 0) parts.push(`${stopped} stopped`)
  if (view.vms.length === 0) return 'limabean: no VMs'
  return parts.join(' · ')
}

/** What changed between two reads, worth a toast. */
export function changes(before: MacView | null, after: MacView, now: number): string[] {
  if (before === null || before.error || after.error) return []
  const out: string[] = []
  const was = new Map(before.vms.map(v => [v.name, v]))
  const is = new Map(after.vms.map(v => [v.name, v]))
  for (const [name, v] of was) {
    const n = is.get(name)
    if (n === undefined) {
      out.push(v.expires && Date.parse(v.expires) <= now ? `${name} expired and was deleted` : `${name} was deleted`)
    } else if (v.status === 'Running' && n.status !== 'Running') {
      out.push(`${name} stopped`)
    } else if (v.status !== 'Running' && n.status === 'Running') {
      out.push(`${name} started`)
    }
  }
  for (const name of is.keys()) {
    if (!was.has(name)) out.push(`${name} was created`)
  }
  const images = new Set(before.images.map(i => i.name))
  for (const im of after.images) {
    if (!images.has(im.name)) out.push(`An image of ${im.env}${im.packs.length > 0 ? ' + ' + im.packs.join(', ') : ''} is ready: new VMs like it only have to boot`)
  }
  return out
}

/** "in 1h20m", for a VM's --ttl. */
export function timeLeft(expires: string | undefined, now: number): string | undefined {
  if (!expires) return undefined
  const ms = Date.parse(expires) - now
  if (Number.isNaN(ms)) return undefined
  if (ms <= 0) return 'expiring'
  const m = Math.floor(ms / 60000)
  return m < 60 ? `${m}m left` : `${Math.floor(m / 60)}h${String(m % 60).padStart(2, '0')}m left`
}

/** "3d ago", for an image. */
export function ago(when: string, now: number): string {
  const ms = now - Date.parse(when)
  if (Number.isNaN(ms) || ms < 0) return ''
  const m = Math.floor(ms / 60000)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  return h < 48 ? `${h}h ago` : `${Math.floor(h / 24)}d ago`
}

/** The forwarded ports worth showing for a VM. */
export function shownPorts(ports: readonly PortRow[] | undefined): PortRow[] {
  return (ports ?? []).filter(p => p.forwarded && p.process !== '')
}

export type { ImageRow, VMRow }
