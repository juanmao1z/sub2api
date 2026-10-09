import assert from 'node:assert/strict'
import { execFileSync, spawnSync } from 'node:child_process'
import { cpSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync, rmSync } from 'node:fs'
import { dirname, resolve, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import test from 'node:test'

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const workspace = JSON.parse(readFileSync(join(repoRoot, '../workspace.json'), 'utf8'))
const mock = `#!/usr/bin/env node
const fs = require('fs');
const cp = require('child_process');
const path = require('path');
const tool = path.basename(process.argv[1]);
const args = process.argv.slice(2);
const state = JSON.parse(fs.readFileSync(process.env.MOCK_STATE));
const save = () => fs.writeFileSync(process.env.MOCK_STATE, JSON.stringify(state));
const remote = process.env.MOCK_REMOTE === '1';
const archivePath = value => value.startsWith('/tmp/sub2api-') ? path.join(process.env.MOCK_ROOT, path.basename(value)) : value;
fs.appendFileSync(process.env.MOCK_LOG, JSON.stringify({tool, args, remote}) + '\\n');
if (tool === 'sudo') {
  const child = cp.spawnSync(args[0] === '-n' ? args[1] : args[0], args.slice(args[0] === '-n' ? 2 : 1), {stdio:'inherit',env:process.env});
  process.exit(child.status ?? 1);
}
if (tool === 'uname') { console.log(state.remoteArch || 'aarch64'); process.exit(0); }
if (tool === 'sleep') process.exit(0);
if (tool === 'curl') {
  state.curlCalls = (state.curlCalls || 0) + 1; save();
  if (state.publicFail && state.curlCalls === 2) process.exit(22);
  console.log('{"status":"ok"}'); process.exit(0);
}
if (tool === 'scp') {
  const dest = archivePath(args.at(-1).split(':').slice(1).join(':'));
  fs.copyFileSync(args.at(-2), dest);
  if (state.corruptUpload) fs.appendFileSync(dest, 'corrupt');
  process.exit(0);
}
if (tool === 'ssh') {
  const index = args.indexOf('bash');
  const script = fs.readFileSync(0, 'utf8');
  const child = cp.spawnSync('/bin/bash', ['-s', '--', ...args.slice(index + 3).map(archivePath)], {
    input: script, encoding:'utf8', env:{...process.env,MOCK_REMOTE:'1'}
  });
  process.stdout.write(child.stdout || ''); process.stderr.write(child.stderr || '');
  process.exit(child.status ?? 1);
}
if (tool === 'docker') {
  const metadata = {Id:'sha256:test-image',Os:'linux',Architecture:state.imageArch || 'arm64',Config:{Labels:{
    'org.opencontainers.image.version':state.imageVersion || '0.2.15',
    'org.opencontainers.image.revision':state.revision
  }}};
  if (args[0] === 'version') { console.log('linux/arm64'); process.exit(0); }
  if (args[0] === 'tag') process.exit(0);
  if (args[0] === 'save') { process.stdout.write('test-image-archive'); process.exit(0); }
  if (args[0] === 'load') { fs.readFileSync(0); console.log('Loaded image'); process.exit(0); }
  if (args[0] === 'image' && args[1] === 'inspect') {
    const format = args[args.indexOf('--format') + 1];
    if (format === '{{json .}}') console.log(JSON.stringify(metadata));
    else if (format === '{{.Id}}') console.log(metadata.Id);
    else if (format === '{{.Os}}/{{.Architecture}}') console.log(metadata.Os + '/' + metadata.Architecture);
    else if (format.includes('image.version')) console.log(metadata.Config.Labels['org.opencontainers.image.version']);
    else if (format.includes('image.revision')) console.log(metadata.Config.Labels['org.opencontainers.image.revision']);
    else process.exit(1);
    process.exit(0);
  }
  if (args[0] === 'inspect') {
    const name = args[1]; const format = args[args.indexOf('--format') + 1];
    if (format === '{{.Id}}') console.log(name + '-unchanged');
    else if (format === '{{.State.Status}}') console.log('running');
    else if (format.includes('.State.Health')) console.log('running/healthy');
    else if (format === '{{.Config.Image}}') console.log(state.activeImage || 'old:0.2.14');
    else process.exit(1);
    process.exit(0);
  }
  if (args[0] === 'compose') {
    if (args.includes('config')) process.exit(0);
    if (args.includes('ps')) { console.log('app-container-id'); process.exit(0); }
    if (args.includes('up')) {
      if (state.failStart && !state.failedStart) { state.failedStart = true; save(); process.exit(1); }
      let image = 'old:0.2.14';
      for (let i = 0; i < args.length; i++) {
        if (args[i] === '-f' && args[i+1].endsWith('.json') && fs.existsSync(args[i+1])) image = JSON.parse(fs.readFileSync(args[i+1])).services.sub2api.image;
      }
      state.activeImage = image; save(); process.exit(0);
    }
  }
}
console.error('unexpected mock call', tool, args); process.exit(1);
`

function fixture(t, overrides = {}) {
  const root = mkdtempSync(join(process.env.PI_SCRATCH_DIR || '/tmp', 'sub2api-release-test-'))
  t.after(() => rmSync(root, { recursive: true, force: true }))
  const repo = join(root, 'sub2api-custom')
  const bin = join(root, 'bin')
  const deploy = join(root, 'production')
  mkdirSync(join(repo, 'backend/cmd/server'), { recursive: true })
  mkdirSync(bin)
  mkdirSync(deploy)
  cpSync(join(repoRoot, 'tools'), join(repo, 'tools'), { recursive: true })
  const git = (...args) => execFileSync('git', ['-C', repo, ...args], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim()
  git('init', '-b', 'main')
  git('config', 'user.email', 'test@example.invalid')
  git('config', 'user.name', 'Release Test')
  writeFileSync(join(repo, 'backend/cmd/server/VERSION'), '0.2.14\n')
  writeFileSync(join(repo, '.gitignore'), '/deploy-artifacts/\n')
  git('add', '.')
  git('commit', '-m', 'official source')
  git('tag', 'v0.2.15')
  const officialCommit = git('rev-parse', 'HEAD')
  writeFileSync(join(repo, 'backend/cmd/server/VERSION'), '0.2.15\n')
  git('add', '.')
  git('commit', '-m', 'custom release version')
  const revision = git('rev-parse', 'HEAD')
  git('remote', 'add', 'origin', workspace.projects[0].remote)
  git('remote', 'add', 'upstream', workspace.versionVerification.officialRemote)
  const config = structuredClone(workspace)
  Object.assign(config.versionVerification, { latestVerifiedCommit: officialCommit, verifiedSourceCommit: officialCommit })
  Object.assign(config.production, { sshAlias: 'test-production', deployRoot: deploy, imageRoot: join(root, 'images') })
  for (const file of config.production.composeFiles) writeFileSync(join(deploy, file), 'services: {}\n')
  writeFileSync(join(root, 'workspace.json'), JSON.stringify(config))
  for (const name of ['docker', 'ssh', 'scp', 'curl', 'sudo', 'uname', 'sleep']) writeFileSync(join(bin, name), mock, { mode: 0o755 })
  const statePath = join(root, 'state.json')
  const log = join(root, 'calls.log')
  writeFileSync(statePath, JSON.stringify({ revision, ...overrides }))
  writeFileSync(log, '')
  const env = { ...process.env, PATH: `${bin}:${process.env.PATH}`, MOCK_STATE: statePath, MOCK_ROOT: root, MOCK_LOG: log, RETRY_ATTEMPTS: '1' }
  const run = (...args) => spawnSync('bash', [join(repo, 'tools/deploy-arm64.sh'), '--tag', 'sub2api-custom:test', '--release', '0.2.15-arm64-test', ...args], { env, encoding: 'utf8' })
  const calls = () => readFileSync(log, 'utf8').trim().split('\n').filter(Boolean).map(line => JSON.parse(line))
  const state = () => JSON.parse(readFileSync(statePath))
  return { root, repo, deploy, config, run, calls, state, git, env }
}

function succeeds(result) { assert.equal(result.status, 0, result.stdout + result.stderr) }
function fails(result, pattern) { assert.notEqual(result.status, 0, result.stdout + result.stderr); if (pattern) assert.match(result.stderr, pattern) }

test('plan has no Docker, SSH or upload side effects', t => {
  const f = fixture(t)
  succeeds(f.run())
  assert.deepEqual(f.calls(), [])
})

test('official tag VERSION lag is accepted only with verified release evidence', t => {
  const f = fixture(t)
  succeeds(f.run())
  f.config.versionVerification.verifiedSourceVersionFile = '0.2.15'
  writeFileSync(join(f.root, 'workspace.json'), JSON.stringify(f.config))
  fails(f.run(), /official source VERSION/)
  assert.deepEqual(f.calls(), [])
})

test('dirty sources are rejected before remote writes', t => {
  const f = fixture(t)
  writeFileSync(join(f.repo, 'uncommitted.txt'), 'dirty')
  fails(f.run('--upload-only'), /clean, committed/)
  assert.deepEqual(f.calls(), [])
})

for (const [name, override] of [['architecture', { imageArch: 'amd64' }], ['version', { imageVersion: '0.2.14' }], ['revision', { revision: 'stale' }]]) {
  test(`stale image ${name} is rejected before SSH`, t => {
    const f = fixture(t, override)
    fails(f.run('--upload-only'), /does not match/)
    assert.ok(!f.calls().some(call => call.tool === 'ssh' || call.tool === 'scp'))
  })
}

test('wrong server architecture is rejected before archive upload', t => {
  const f = fixture(t, { remoteArch: 'x86_64' })
  fails(f.run('--upload-only'), /remote architecture mismatch/)
  assert.ok(!f.calls().some(call => call.tool === 'scp'))
})

test('upload checks archive and image identity without switching a service', t => {
  const f = fixture(t)
  const result = f.run('--upload-only')
  succeeds(result)
  assert.match(result.stdout, /upload=verified/)
  assert.ok(f.calls().some(call => call.tool === 'docker' && call.args[0] === 'load'))
  assert.ok(!f.calls().some(call => call.tool === 'docker' && call.args.includes('up')))
})

test('corrupted upload fails before docker load', t => {
  const f = fixture(t, { corruptUpload: true })
  fails(f.run('--upload-only'), /checksum/)
  assert.ok(!f.calls().some(call => call.tool === 'docker' && call.args[0] === 'load'))
})

test('apply changes only the application through a dedicated override', t => {
  const f = fixture(t)
  succeeds(f.run('--apply'))
  const override = JSON.parse(readFileSync(join(f.deploy, f.config.production.releaseComposeFile)))
  assert.deepEqual(override, { services: { sub2api: { image: 'sub2api-custom:0.2.15-arm64-test' } } })
  for (const file of f.config.production.composeFiles) assert.equal(readFileSync(join(f.deploy, file), 'utf8'), 'services: {}\n')
  const up = f.calls().filter(call => call.tool === 'docker' && call.args.includes('up'))
  assert.equal(up.length, 1)
  assert.deepEqual(up[0].args.slice(-6), ['up', '-d', '--no-deps', '--pull', 'never', 'sub2api'])
})

for (const [name, override] of [['compose start failure', { failStart: true }], ['public health failure', { publicFail: true }]]) {
  test(`${name} restores previous application image`, t => {
    const f = fixture(t, override)
    fails(f.run('--apply'))
    assert.equal(f.state().activeImage, 'old:0.2.14')
    assert.equal(f.calls().filter(call => call.tool === 'docker' && call.args.includes('up')).length, 2)
  })
}

test('staged prepare blocks an unfinished merge without creating a tag', t => {
  const f = fixture(t)
  writeFileSync(join(f.repo, 'dirty.txt'), 'dirty')
  const result = spawnSync('bash', [join(f.repo, 'tools/sub2api-release.sh'), 'prepare', '--release', 'blocked', '--git-tag', 'release-blocked'], { env: f.env, encoding: 'utf8' })
  fails(result, /working tree is not clean/)
  assert.equal(f.git('tag', '--list', 'release-blocked'), '')
})

test('public health rollback restores an existing release override', t => {
  const f = fixture(t, { publicFail: true })
  const previous = { services: { sub2api: { image: 'previous:0.2.14' } } }
  const path = join(f.deploy, f.config.production.releaseComposeFile)
  writeFileSync(path, JSON.stringify(previous))
  fails(f.run('--apply'))
  assert.deepEqual(JSON.parse(readFileSync(path)), previous)
  assert.equal(f.state().activeImage, 'previous:0.2.14')
})

test('staged build reuses only a matching committed release image', t => {
  const f = fixture(t)
  const script = join(f.repo, 'tools/sub2api-release.sh')
  succeeds(spawnSync('bash', [script, 'prepare', '--release', 'reuse', '--git-tag', 'release-reuse'], { env: f.env, encoding: 'utf8' }))
  const result = spawnSync('bash', [script, 'build', '--release', 'reuse'], { env: f.env, encoding: 'utf8' })
  succeeds(result)
  assert.match(result.stdout, /existing_image=/)
  const manifest = JSON.parse(readFileSync(join(f.repo, 'deploy-artifacts/reuse/release.json')))
  assert.equal(manifest.stages.build.status, 'passed')
  assert.equal(manifest.artifact.image_id, 'sha256:test-image')
  assert.equal(manifest.artifact.architecture, 'linux/arm64')
})
