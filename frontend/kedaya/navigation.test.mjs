/** @file @brief Verify public/console document boundaries and browser navigation semantics. */
import test from 'node:test';
import assert from 'node:assert/strict';
import { installNavigation, runtimeForPath } from './runtime/navigation.js';

function browser(pathname, origin = 'https://api.zhouz.online') {
  let current = new URL(pathname, origin);
  const actions = [];
  const listeners = new Map();
  const win = {
    location: {
      get href() { return current.href; },
      get origin() { return current.origin; },
      get protocol() { return current.protocol; },
      get host() { return current.host; },
      get pathname() { return current.pathname; },
      assign(url) { actions.push(['assign', url]); },
      replace(url) { actions.push(['replace', url]); },
      reload() { actions.push(['reload']); },
    },
    history: Object.fromEntries(['pushState', 'replaceState'].map(method => [method, (state, unused, target) => {
      actions.push([method, state, target]);
      if (target !== undefined && target !== null) current = new URL(target, current);
    }])),
    addEventListener(type, listener) { listeners.set(type, listener); },
    document: { addEventListener(type, listener) { listeners.set(type, listener); } },
  };
  installNavigation(win);
  return {
    win,
    actions,
    pop(path) {
      current = new URL(path, current);
      const event = { stopped: false, stopImmediatePropagation() { this.stopped = true; } };
      listeners.get('popstate')(event);
      return event;
    },
    click(href, options = {}) {
      const anchor = { href: new URL(href, current).href, target: options.target || '', hasAttribute: name => name === 'download' && options.download };
      const event = {
        button: 0,
        defaultPrevented: false,
        target: { closest: () => anchor },
        preventDefault() { this.defaultPrevented = true; },
        stopImmediatePropagation() { this.stopped = true; },
        ...options,
      };
      listeners.get('click')(event);
      return event;
    },
  };
}

test('only the landing aliases and login route select restored public pages', () => {
  for (const path of ['/', '/home', '/home/', '/login', '/login/']) assert.equal(runtimeForPath(path), 'native', path);
  for (const path of ['/dashboard', '/admin', '/payment/result', '/register', '/forgot-password', '/privacy', '/home-other', '/login/oauth']) {
    assert.equal(runtimeForPath(path), 'review', path);
  }
});

test('login redirects reload the console document while preserving query and fragment', () => {
  const { win, actions } = browser('/login?redirect=%2Fkeys');
  win.history.pushState({ position: 1 }, '', '/keys?group=2#usage');
  assert.deepEqual(actions, [['assign', 'https://api.zhouz.online/keys?group=2#usage']]);
});

test('logout and auth guards preserve replacement semantics across entries', () => {
  const { win, actions } = browser('/dashboard');
  win.history.replaceState({}, '', '/login?redirect=%2Fdashboard');
  assert.deepEqual(actions, [['replace', 'https://api.zhouz.online/login?redirect=%2Fdashboard']]);
});

test('navigation inside one entry retains SPA state and history', () => {
  const { win, actions } = browser('/home');
  const state = { position: 2 };
  win.history.pushState(state, '', '/login?redirect=%2Fadmin');
  assert.deepEqual(actions, [['pushState', state, 'https://api.zhouz.online/login?redirect=%2Fadmin']]);
  assert.equal(win.location.pathname, '/login');
  assert.equal(browser('/home').click('/login').defaultPrevented, false);
});

test('back and forward reload only when the current document has the wrong entry', () => {
  const console = browser('/dashboard');
  assert.equal(console.pop('/keys').stopped, false);
  assert.deepEqual(console.actions, []);
  assert.equal(console.pop('/login?redirect=%2Fkeys').stopped, true);
  assert.deepEqual(console.actions, [['reload']]);
  const publicPage = browser('/home');
  assert.equal(publicPage.pop('/login').stopped, false);
  assert.deepEqual(publicPage.actions, []);
});

test('ordinary cross-entry links reload without intercepting external or modified clicks', () => {
  const page = browser('/home');
  assert.equal(page.click('/dashboard').defaultPrevented, true);
  assert.deepEqual(page.actions, [['assign', 'https://api.zhouz.online/dashboard']]);
  for (const options of [{ ctrlKey: true }, { metaKey: true }, { button: 1 }, { target: '_blank' }, { download: true }, { defaultPrevented: true }]) {
    const ignored = browser('/home');
    ignored.click('/dashboard', options);
    assert.deepEqual(ignored.actions, [], JSON.stringify(options));
  }
  const external = browser('/home');
  assert.equal(external.click('https://example.invalid/login').defaultPrevented, false);
  assert.deepEqual(external.actions, []);
});

test('captured absolute links remain local in previews on either side of the boundary', () => {
  const page = browser('/home', 'http://127.0.0.1:3000');
  page.win.history.pushState({}, '', 'https://api.zhouz.online/login?redirect=%2Fkeys');
  assert.deepEqual(page.actions, [['pushState', {}, 'http://127.0.0.1:3000/login?redirect=%2Fkeys']]);
  assert.equal(page.click('https://api.zhouz.online/dashboard').defaultPrevented, true);
  assert.deepEqual(page.actions[1], ['assign', 'http://127.0.0.1:3000/dashboard']);
});
