// limabean for Claude Code.
//
// Inside a limabean VM: tells Claude where it runs (the VM, its allowlist,
// whether it's locked, its packs) in the system prompt, shows the same in a
// band above the prompt, and adds a note when a command fails on a host the
// allowlist blocks, so Claude asks for it instead of retrying.
//
// On the Mac: keeps a status line of your VMs, opens a pane with each VM's
// state, packs, network, time left and forwarded ports (/limabean), and
// toasts what changed behind your back (autostop, an expired --ttl, a new
// image). It reads limabean's own --json output; it changes nothing but
// what its Start and Stop buttons ask for.

import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { InsideVM, MacView, Place, PortRow, VMRow } from '../types'
import * as F from './facts'

const PANE = 'limabean'
const POLL_MS = 30_000

const place = atom({ plugin: 'limabean', key: 'place' } as const, 'none' as Place)
const inside = atom({ plugin: 'limabean', key: 'inside' } as const, null as InsideVM | null)
const mac = atom({ plugin: 'limabean', key: 'mac' } as const, null as MacView | null)
const bandHidden = atom({ plugin: 'limabean', key: 'bandHidden' } as const, false)

type $ = EngineInterface

// Set by register from the options; a reload runs register again.
let configured = ''
let preview: 'off' | 'mac' | 'vm' = 'off'
// The limabean binary on the Mac, once found, and the PATH it runs with.
let binary: string | undefined
let env: Record<string, string> = {}

export const register: Register = (on, options) => {
  configured = typeof options.limabean === 'string' ? options.limabean.trim() : ''
  preview = options.preview === 'mac' || options.preview === 'vm' ? options.preview : 'off'
  binary = undefined
  env = {}

  on('session.start', async ($, e, next) => {
    const started = await next(e)
    await $.command.register({
      name: 'limabean',
      description: "Show your limabean VMs, or this VM's network and limits",
    })
    const where = await detect($)
    await update($, place, () => where)
    if (where === 'vm') {
      await readInside($)
      if (preview === 'off') $.clock.every(POLL_MS, () => void readInside($))
    } else if (where === 'mac') {
      void readMac($)
      if (preview === 'off') $.clock.every(POLL_MS, () => void readMac($))
    }
    return started
  })

  on('command.run', { command: 'limabean' }, async $ => {
    const where = await read($, place)
    if (where === 'mac') {
      const opened = await $.ui.open({ id: PANE, title: 'limabean' })
      void readMac($)
      return { text: opened.isPlaced ? 'Opened the limabean pane.' : `The limabean pane is waiting: ${opened.reason}` }
    }
    const vm = await read($, inside)
    if (where === 'vm' && vm !== null) {
      await update($, bandHidden, () => false)
      $.ui.status(undefined)
      return { text: F.insideSection(vm) }
    }
    return { text: "limabean isn't here: this session runs neither in a limabean VM nor on a Mac with limabean on its PATH. Set the binary under /config if it's somewhere else." }
  })

  // Inside a VM, Claude learns where it runs before its first step.
  on('prompt.compose', async ($, e, next) => {
    const composed = await next(e)
    const vm = await read($, inside)
    if (vm === null || e.traits.includes('bare')) return composed
    return { sections: [...composed.sections, { id: 'limabean:vm', text: F.insideSection(vm), scope: 'session' as const }] }
  })

  // A command that failed on a host the allowlist blocks gets a note, so
  // Claude asks for the domain instead of trying mirrors.
  on('tool.call', { tool: 'Bash' }, async ($, e, next) => {
    const ran = await next(e)
    if (ran.deny !== undefined) return ran
    const vm = await read($, inside)
    if (vm === null || vm.learning || vm.allow.length === 0) return ran
    const host = F.blockedHost(ran.text ?? '')
    if (host === undefined || F.allows(vm.allow, host)) return ran
    return { ...ran, context: [...(ran.context ?? []), F.blockedNote(vm, host)] }
  }).catch(($, e, next) => next(e))

  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    const vm = await read($, inside)
    if (vm === null || e.props.hasSurvey || (await read($, bandHidden))) return next(e)
    const { Box, Text, Button } = $.ui.resolve(e)
    const network = vm.learning
      ? 'learning: open, lookups recorded'
      : vm.allow.length > 0
        ? `only ${vm.allow.slice(0, 4).join(', ')}${vm.allow.length > 4 ? ` +${vm.allow.length - 4}` : ''}`
        : 'open network'
    return (
      <Box flexDirection="row" flexWrap="wrap">
        <Text color="green" bold>
          limabean{' '}
        </Text>
        <Text bold>{vm.name}</Text>
        <Text dimColor> · {network}</Text>
        {vm.locked && <Text color="yellow"> · locked: no sudo, no Docker</Text>}
        <Text> </Text>
        <Button
          key="hide"
          label="Hide"
          plain
          onPress={async () => {
            await update($, bandHidden, () => true)
            $.ui.status(F.insideLine(vm))
          }}
        />
      </Box>
    )
  })

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Box, Text, Button, Link } = $.ui.resolve(e)
    const view = await read($, mac)
    if (view === null) return <Text dimColor>Reading your VMs…</Text>
    if (view.error) return <Text color="red">{view.error}</Text>
    const now = view.readAt
    return (
      <Box flexDirection="column">
        {view.vms.length === 0 && <Text dimColor>No VMs yet. limabean new dev, or limabean up in a project.</Text>}
        {view.vms.map(vm => {
          const isUp = vm.status === 'Running'
          const left = F.timeLeft(vm.expires, now)
          return (
            <Box key={`vm-${vm.name}`} flexDirection="column">
              <Box flexDirection="row" flexWrap="wrap">
                <Text color={isUp ? 'green' : undefined} dimColor={!isUp}>
                  {isUp ? '● ' : '○ '}
                </Text>
                <Text bold>{vm.name}</Text>
                <Text dimColor>
                  {' '}
                  {vm.cpus} CPU · {vm.memoryGiB} GiB · {vm.network}
                </Text>
                {left !== undefined && <Text color="yellow"> · {left}</Text>}
                <Text> </Text>
                <Button
                  key={`${isUp ? 'stop' : 'start'}-${vm.name}`}
                  label={isUp ? 'Stop' : 'Start'}
                  onPress={async () => {
                    $.ui.toast(`${isUp ? 'Stopping' : 'Starting'} ${vm.name}…`)
                    try {
                      await limabean($, [isUp ? 'stop' : 'start', '--', vm.name], 600_000)
                    } catch (err) {
                      $.ui.toast(`${vm.name}: ${err instanceof Error ? err.message : String(err)}`)
                    }
                    await readMac($)
                  }}
                />
              </Box>
              {vm.packs !== null && vm.packs.length > 0 && <Text dimColor>   packs: {vm.packs.join(', ')}</Text>}
              {F.shownPorts(view.ports[vm.name]).map(p => (
                <Box key={`port-${vm.name}-${p.port}`} flexDirection="row">
                  <Text dimColor>   :{p.port} {p.process} → </Text>
                  <Link href={p.url} label={p.url} />
                </Box>
              ))}
            </Box>
          )
        })}
        {view.images.length > 0 && (
          <Box flexDirection="column">
            <Text> </Text>
            <Text bold>Images</Text>
            {view.images.map(im => (
              <Text key={`img-${im.name}`} dimColor={im.status !== 'current'}>
                {'  '}
                {im.env}
                {im.packs.length > 0 ? ' + ' + im.packs.join(', ') : ''} · {im.status} · built {F.ago(im.built, now)}
              </Text>
            ))}
          </Box>
        )}
        <Text> </Text>
        <Button key="refresh" label="Refresh" onPress={() => readMac($)} />
      </Box>
    )
  })
}

const EMPTY: MacView = { vms: [], images: [], ports: {}, readAt: 0 }

const PREVIEW_VM: InsideVM = {
  name: 'agent',
  allow: ['anthropic.com', 'claude.ai', 'claude.com', 'github.com', 'githubusercontent.com', 'npmjs.org'],
  locked: true,
  learning: false,
  packs: ['claude', 'node'],
  codeDir: '~/code',
}

/** Works out where this session runs. */
async function detect($: $): Promise<Place> {
  if (preview !== 'off') return preview
  if (await $.fs.exists('/etc/limabean/mise/00-core.toml')) return 'vm'
  const home = (await $.env.get('HOME')) ?? ''
  const path = (await $.env.get('PATH')) ?? ''
  const candidates = configured
    ? [configured]
    : ['limabean', '/opt/homebrew/bin/limabean', '/usr/local/bin/limabean', `${home}/go/bin/limabean`, `${home}/.local/bin/limabean`]
  for (const c of candidates) {
    try {
      const { exitCode } = await $.process.run([c, 'version'], { timeoutMs: 5000 })
      if (exitCode === 0) {
        binary = c
        // Apps started from the Dock have a bare PATH; limabean needs limactl.
        const dir = c.includes('/') ? c.slice(0, c.lastIndexOf('/')) : ''
        env = { PATH: [dir, '/opt/homebrew/bin', '/usr/local/bin', path].filter(Boolean).join(':') }
        return 'mac'
      }
    } catch {
      // not there; try the next
    }
  }
  return 'none'
}

/** Reads this VM's own facts: its name, policy, packs and code directory. */
async function readInside($: $): Promise<void> {
  let vm: InsideVM
  if (preview === 'vm') {
    vm = F.parseJSON<InsideVM>(await $.fs.read(`${$.plugin.root}/fixtures/inside.json`), PREVIEW_VM)
  } else {
    const policy = await $.fs.read('/mnt/host/limabean/network.conf').catch(() => '')
    const packs = await $.fs.list('/etc/limabean/mise').catch(() => [])
    const hostname = await $.fs.read('/etc/hostname').catch(() => 'lima-vm')
    const home = (await $.env.get('HOME')) ?? '~'
    const root = await $.process
      .run(['git', 'config', '--global', '--get', 'ghq.root'], { timeoutMs: 5000 })
      .then(r => r.stdout.trim())
      .catch(() => '')
    vm = {
      name: F.vmName(hostname),
      ...F.parsePolicy(policy),
      packs: F.packsFromFiles(packs.map(p => p.name)),
      codeDir: (root || `${home}/code`).replace(home, '~'),
    }
  }
  const before = await read($, inside)
  if (JSON.stringify(before) !== JSON.stringify(vm)) {
    await update($, inside, () => vm)
    if (before !== null && JSON.stringify(before.allow) !== JSON.stringify(vm.allow)) {
      $.ui.toast(`This VM's allowlist changed: ${vm.allow.length > 0 ? vm.allow.join(', ') : 'the network is open'}`)
    }
  }
  if (await read($, bandHidden)) $.ui.status(F.insideLine(vm))
}

/** Runs limabean on the Mac and returns its stdout. */
async function limabean($: $, args: string[], timeoutMs = 30_000): Promise<string> {
  if (binary === undefined) throw new Error("limabean isn't on this Mac's PATH")
  const r = await $.process.run([binary, ...args, '--plain'], { env, timeoutMs })
  if (r.exitCode !== 0) throw new Error(r.stderr.trim().split('\n').pop() || `limabean ${args[0]} failed`)
  return r.stdout
}

/** Reads VMs, images and (while the pane is open) ports, on the Mac. */
async function readMac($: $): Promise<void> {
  let view: MacView
  if (preview === 'mac') {
    view = { ...F.parseJSON<MacView>(await $.fs.read(`${$.plugin.root}/fixtures/mac.json`), EMPTY), readAt: await $.clock.now() }
  } else {
    try {
      const [vms, images] = await Promise.all([
        limabean($, ['ls', '--json']).then(t => F.parseJSON<VMRow[]>(t, [])),
        limabean($, ['images', 'ls', '--json']).then(t => F.parseJSON<MacView['images']>(t, [])),
      ])
      const ports: Record<string, PortRow[]> = {}
      if ((await $.ui.panes()).some(p => p.id === PANE)) {
        await Promise.all(
          vms
            .filter(v => v.status === 'Running')
            .map(async v => {
              ports[v.name] = await limabean($, ['ports', '--json', '--', v.name])
                .then(t => F.parseJSON<PortRow[]>(t, []))
                .catch(() => [])
            }),
        )
      }
      view = { vms, images, ports, readAt: await $.clock.now() }
    } catch (err) {
      view = { ...EMPTY, error: err instanceof Error ? err.message : String(err), readAt: await $.clock.now() }
    }
  }
  const before = await read($, mac)
  for (const change of F.changes(before, view, view.readAt).slice(0, 3)) $.ui.toast(change)
  await update($, mac, () => view)
  $.ui.status(F.macLine(view))
}
