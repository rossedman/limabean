import { expect, mock, test } from 'claude-code/testing'
import type { On } from 'claude-code'

const FILES: Record<string, string> = {
  '/mnt/host/limabean/network.conf': 'locked\nallow github.com\nallow npmjs.org\n',
  '/etc/hostname': 'lima-agent\n',
}

// A limabean VM, beneath the plugin: its files, environment and git.
function inVM(on: On) {
  mock.clock(on)
  mock.env(on, { HOME: '/home/me.linux' })
  on('fs.exists', ($, e) => ({ value: e.path === '/etc/limabean/mise/00-core.toml' }))
  on('fs.read', ($, e) => {
    const text = FILES[e.path]
    if (text === undefined) throw new Error(`no ${e.path}`)
    return { value: text }
  })
  on('fs.list', () => ({
    value: [
      { name: '00-core.toml', kind: 'file' as const, size: 1, mtimeMs: 0, isLink: false },
      { name: '50-claude.toml', kind: 'file' as const, size: 1, mtimeMs: 0, isLink: false },
    ],
  }))
  on('process.run', () => ({ value: { exitCode: 0, stdout: '/home/me.linux/code\n', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }))
  on('session.start', ($, e) => ({ cwd: e.cwd }))
  on('command.register', () => ({ value: { command: 'limabean' } }))
  on('ui.status', () => ({ value: undefined }))
}

test('tells Claude which VM it is in, and what it may reach', async ($, on) => {
  inVM(on)
  on('prompt.compose', () => ({ sections: [{ id: 'intro', text: 'You are Claude Code.', scope: 'shared' as const }] }))
  await $.session.start({ cwd: '/home/me.linux', surface: 'terminal', isInteractive: true })
  const { sections } = await $.prompt.compose({ model: 'm', promptModel: 'm', surfaces: ['terminal'], tools: [], outputStyle: null, traits: [] })
  const mine = sections.find(s => s.id === 'limabean:vm')
  expect(mine?.scope).toBe('session')
  expect(mine?.text).toContain('limabean VM "agent"')
  expect(mine?.text).toContain('only github.com, npmjs.org')
  expect(mine?.text).toContain('no sudo and no Docker')
  expect(mine?.text).toContain('Repositories live under ~/code')
  const bare = await $.prompt.compose({ model: 'm', promptModel: 'm', surfaces: [], tools: [], outputStyle: null, traits: ['bare'] })
  expect(bare.sections.some(s => s.id === 'limabean:vm')).toBe(false)
})

test('notes a host the allowlist blocked, and nothing else', async ($, on) => {
  inVM(on)
  let output = ''
  on('tool.call', () => ({ result: { stdout: '', stderr: output, interrupted: false }, text: output }))
  await $.session.start({ cwd: '/', surface: 'terminal', isInteractive: true })

  output = 'curl: (6) Could not resolve host: pypi.org'
  const blocked = await $.tool.call({ tool: 'Bash', command: 'curl https://pypi.org' })
  expect(blocked.context?.join('\n')).toContain('limabean network allow agent pypi.org')

  output = 'curl: (6) Could not resolve host: api.github.com'
  const allowed = await $.tool.call({ tool: 'Bash', command: 'curl https://api.github.com' })
  expect(allowed.context ?? []).toEqual([])

  output = 'ok'
  const fine = await $.tool.call({ tool: 'Bash', command: 'true' })
  expect(fine.context ?? []).toEqual([])
})

test('shows the VM above the prompt, and hides it on request', async ($, on) => {
  inVM(on)
  // The engine's own band, beneath the plugin: an empty box.
  on('ui.render', ($, e) => {
    const { Box } = $.ui.resolve(e)
    return <Box key="engine" />
  })
  await $.session.start({ cwd: '/', surface: 'terminal', isInteractive: true })
  for (const surface of ['terminal', 'desktop'] as const) {
    const ui = await $.ui.mount({
      plugin: 'limabean',
      surface,
      component: 'AbovePrompt',
      props: { hasSurvey: false, isWorking: false, maxRows: 4, bodyColumns: 100, scroll: { isAtBottom: true }, view: {} },
    } as never)
    expect(await ui.find({ type: 'Text', text: /agent/ })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: /locked: no sudo, no Docker/ })).toBeDefined()
    await ui.unmount()
  }
  const ui = await $.ui.mount({
    plugin: 'limabean',
    surface: 'terminal',
    component: 'AbovePrompt',
    props: { hasSurvey: false, isWorking: false, maxRows: 4, bodyColumns: 100, scroll: { isAtBottom: true }, view: {} },
  } as never)
  await ui.press({ key: 'hide' })
  expect(await ui.find({ type: 'Text', text: /agent/ })).toBeUndefined()
  await ui.unmount()
})
