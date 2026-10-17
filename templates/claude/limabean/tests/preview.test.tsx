import { expect, mock, test } from 'claude-code/testing'

// Preview data is for the person to look at: it never reaches Claude, and
// it says it's sample data. Here the session runs on a Mac without limabean.
test('a vm preview never tells Claude it is in a VM', { options: { preview: 'vm' } }, async ($, on) => {
  mock.clock(on)
  mock.env(on, { HOME: '/Users/me' })
  on('fs.exists', () => ({ value: false }))
  on('fs.read', ($, e) => {
    if (!e.path.endsWith('/fixtures/inside.json')) throw new Error(`no ${e.path}`)
    return { value: JSON.stringify({ name: 'agent', allow: ['github.com'], locked: true, learning: false, packs: [], codeDir: '~/code' }) }
  })
  on('session.start', ($, e) => ({ cwd: e.cwd }))
  on('command.register', () => ({ value: { command: 'limabean' } }))
  on('ui.status', () => ({ value: undefined }))
  on('prompt.compose', () => ({ sections: [{ id: 'intro', text: 'You are Claude Code.', scope: 'shared' as const }] }))
  on('tool.call', () => ({ result: { stdout: '', stderr: '', interrupted: false }, text: 'curl: (6) Could not resolve host: pypi.org' }))
  on('ui.render', ($, e) => {
    const { Box } = $.ui.resolve(e)
    return <Box key="engine" />
  })
  await $.session.start({ cwd: '/Users/me', surface: 'terminal', isInteractive: true })

  const { sections } = await $.prompt.compose({ model: 'm', promptModel: 'm', surfaces: ['terminal'], tools: [], outputStyle: null, traits: [] })
  expect(sections.some(s => s.id === 'limabean:vm')).toBe(false)

  const ran = await $.tool.call({ tool: 'Bash', command: 'curl https://pypi.org' })
  expect(ran.context ?? []).toEqual([])

  const ui = await $.ui.mount({
    plugin: 'limabean',
    surface: 'terminal',
    component: 'AbovePrompt',
    props: { hasSurvey: false, isWorking: false, maxRows: 4, bodyColumns: 100, scroll: { isAtBottom: true }, view: {} },
  } as never)
  expect(await ui.find({ type: 'Text', text: /preview, sample data/ })).toBeDefined()
  await ui.unmount()
})
