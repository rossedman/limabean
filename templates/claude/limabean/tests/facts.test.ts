import { describe, expect, test } from 'claude-code/testing'

import * as F from '../hooks/facts'
import type { MacView } from '../types'

describe('facts', () => {
  test('reads a network policy', () => {
    const p = F.parsePolicy('# comment\nlocked\nallow GitHub.com\nallow pypi.org\nallow github.com\n')
    expect(p).toEqual({ allow: ['github.com', 'pypi.org'], locked: true, learning: false })
    expect(F.parsePolicy('')).toEqual({ allow: [], locked: false, learning: false })
    expect(F.parsePolicy('learn\n').learning).toBe(true)
  })

  test('names packs and the VM', () => {
    expect(F.packsFromFiles(['00-core.toml', '50-rust.toml', '50-supply-chain.toml', 'notes.txt'])).toEqual(['rust', 'supply-chain'])
    expect(F.vmName('lima-agent\n')).toBe('agent')
    expect(F.vmName('myhost')).toBe('myhost')
  })

  test('finds the host a command could not resolve', () => {
    expect(F.blockedHost('curl: (6) Could not resolve host: pypi.org')).toBe('pypi.org')
    expect(F.blockedHost('npm error getaddrinfo ENOTFOUND registry.yarnpkg.com')).toBe('registry.yarnpkg.com')
    expect(F.blockedHost("Failed to resolve 'files.pythonhosted.org' ([Errno -3] Temporary failure in name resolution)")).toBe('files.pythonhosted.org')
    expect(F.blockedHost('dial tcp: lookup proxy.golang.org on 127.0.0.2:53: no such host')).toBe('proxy.golang.org')
    expect(F.blockedHost('fatal: unable to access: Could not resolve host: GitHub.com')).toBe('github.com')
    expect(F.blockedHost('all tests passed')).toBeUndefined()
    expect(F.allows(['github.com'], 'api.github.com')).toBe(true)
    expect(F.allows(['github.com'], 'notgithub.com')).toBe(false)
  })

  test('words the VM for Claude', () => {
    const vm = { name: 'agent', allow: ['github.com'], locked: true, learning: false, packs: ['claude'], codeDir: '~/code' }
    const text = F.insideSection(vm)
    expect(text).toContain('limabean VM "agent"')
    expect(text).toContain('only github.com')
    expect(text).toContain('limabean network allow agent <domain>')
    expect(text).toContain('no sudo and no Docker')
    expect(text).toContain('core, claude')
    expect(F.insideSection({ ...vm, allow: [], locked: false })).toContain('The network is open.')
    expect(F.insideSection({ ...vm, learning: true })).toContain('learns')
    expect(F.insideLine(vm)).toBe('limabean · agent · 1 domain · locked')
    expect(F.blockedNote(vm, 'pypi.org')).toContain('limabean network allow agent pypi.org')
  })

  test('says what changed on the Mac', () => {
    const vm = (name: string, status: string, expires?: string) => ({ name, status, env: 'scratch', cpus: 2, memoryGiB: 4, secrets: 0, sshHost: `lima-${name}`, network: 'open', packs: [], expires })
    const before: MacView = { vms: [vm('dev', 'Running'), vm('spike', 'Running', '2026-01-01T00:00:00Z'), vm('api', 'Stopped')], images: [], ports: {}, readAt: 0 }
    const after: MacView = {
      vms: [vm('dev', 'Stopped'), vm('api', 'Running'), vm('new', 'Running')],
      images: [{ name: 'limabean-img-1', env: 'scratch', packs: ['go'], built: '', used: '', status: 'current' }],
      ports: {},
      readAt: 0,
    }
    expect(F.changes(before, after, Date.parse('2026-06-01T00:00:00Z'))).toEqual([
      'dev stopped',
      'spike expired and was deleted',
      'api started',
      'new was created',
      'An image of scratch + go is ready: new VMs like it only have to boot',
    ])
    expect(F.changes(null, after, 0)).toEqual([])
    expect(F.macLine(after)).toBe('limabean: 2 running · 1 stopped')
    expect(F.macLine({ ...after, vms: [] })).toBe('limabean: no VMs')
    expect(F.macLine({ ...after, error: 'limactl not found' })).toBe('limabean: limactl not found')
  })

  test('formats time', () => {
    const now = Date.parse('2026-10-17T12:00:00Z')
    expect(F.timeLeft('2026-10-17T13:20:00Z', now)).toBe('1h20m left')
    expect(F.timeLeft('2026-10-17T12:30:00Z', now)).toBe('30m left')
    expect(F.timeLeft('2026-10-17T11:00:00Z', now)).toBe('expiring')
    expect(F.timeLeft(undefined, now)).toBeUndefined()
    expect(F.ago('2026-10-14T12:00:00Z', now)).toBe('3d ago')
    expect(F.ago('2026-10-17T10:00:00Z', now)).toBe('2h ago')
    expect(F.shownPorts([{ port: 1, process: 'node', forwarded: true, url: 'u' }, { port: 2, process: '', forwarded: true, url: 'u' }, { port: 3, process: 'x', forwarded: false, url: 'u' }]).map(p => p.port)).toEqual([1])
  })
})
