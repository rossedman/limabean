import { expect, mock, test } from 'claude-code/testing'
import type { On } from 'claude-code'

const LS = JSON.stringify([
  { name: 'dev', status: 'Running', env: 'scratch', cpus: 2, memoryGiB: 4, secrets: 1, sshHost: 'lima-dev', network: 'open', packs: ['go', 'node'] },
  { name: 'api', status: 'Stopped', env: 'scratch', cpus: 4, memoryGiB: 8, secrets: 0, sshHost: 'lima-api', network: '3 domain(s), locked', packs: ['go'] },
])
const IMAGES = JSON.stringify([{ name: 'limabean-img-1', env: 'scratch', packs: ['go', 'node'], built: '2026-10-15T00:00:00Z', used: '2026-10-17T00:00:00Z', status: 'current' }])
const PORTS = JSON.stringify([{ port: 3000, process: 'node', forwarded: true, url: 'http://localhost:3000' }])

// A Mac with limabean, beneath the plugin.
function onMac(on: On, ran: string[], status: string[], binary = 'limabean') {
  const clock = mock.clock(on)
  mock.env(on, { HOME: '/Users/me', PATH: '/usr/bin' })
  on('fs.exists', () => ({ value: false }))
  on('process.run', ($, e) => {
    ran.push(e.argv.join(' '))
    const args = e.argv.slice(1)
    const out =
      args[0] === 'version' ? 'limabean 1.0'
      : args[0] === 'ls' ? LS
      : args[0] === 'images' ? IMAGES
      : args[0] === 'ports' ? PORTS
      : ''
    if (e.argv[0] !== binary) throw new Error('not found')
    return { value: { exitCode: 0, stdout: out, stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })
  on('session.start', ($, e) => ({ cwd: e.cwd }))
  on('command.register', () => ({ value: { command: 'limabean' } }))
  on('ui.status', ($, e) => {
    status.push(e.text ?? '')
    return { value: undefined }
  })
  on('ui.toast', () => ({ value: undefined }))
  return clock
}

test('keeps a status line of the VMs on a Mac', async ($, on) => {
  const ran: string[] = []
  const status: string[] = []
  const clock = onMac(on, ran, status)
  on('ui.panes', () => ({ value: [] }))
  await $.session.start({ cwd: '/Users/me', surface: 'terminal', isInteractive: true })
  await clock.settle()
  expect(ran.some(r => r === 'limabean ls --json --plain')).toBe(true)
  expect(status.at(-1)).toBe('limabean: 1 running · 1 stopped')
})

test('draws a pane of the VMs, their ports and images', async ($, on) => {
  const ran: string[] = []
  const clock = onMac(on, ran, [])
  on('ui.panes', () => ({ value: [{ id: 'limabean', title: 'limabean', isShown: true, isFocused: false, isPlaced: true }] }) as never)
  await $.session.start({ cwd: '/Users/me', surface: 'terminal', isInteractive: true })
  await clock.settle()
  for (const surface of ['terminal', 'desktop'] as const) {
    const ui = await $.ui.mount({
      plugin: 'limabean',
      surface,
      component: 'Pane',
      requestId: 'limabean',
      props: { title: 'limabean', isFocused: false, bodyColumns: 80, placement: 'dock' },
    } as never)
    expect(await ui.find({ type: 'Text', text: /^dev$/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /locked/ })).toBeDefined()
    expect(await ui.find({ key: 'start-api' })).toBeDefined()
    expect(await ui.find({ key: 'stop-dev' })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /scratch \+ go, node · current/ })).toBeDefined()
    await ui.unmount()
  }
  expect(ran).toContain('limabean ports --json -- dev --plain')
})

test('uses limabean.binary when it is set', { options: { binary: '/opt/tools/limabean' } }, async ($, on) => {
  const ran: string[] = []
  const status: string[] = []
  const clock = onMac(on, ran, status, '/opt/tools/limabean')
  on('ui.panes', () => ({ value: [] }))
  await $.session.start({ cwd: '/Users/me', surface: 'terminal', isInteractive: true })
  await clock.settle()
  expect(ran[0]).toBe('/opt/tools/limabean version')
  expect(ran.some(r => r.startsWith('limabean '))).toBe(false)
  expect(status.at(-1)).toBe('limabean: 1 running · 1 stopped')
})
