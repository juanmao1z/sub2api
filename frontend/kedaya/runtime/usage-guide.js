/**
 * 使用说明路由组件。发布时将本文件与 usage-guide.css 放入 assets/。
 * 与固定 base 资源共同发布；升级 base 后重新核实以下两个导入的文件名及导出名。
 * default: 含 AppLayout 的路由页面；UsageGuideContent: 可独立挂载的正文。
 * 无 API 请求，不访问存储，不读取用户密钥。示例中的凭据和模型均为占位符。
 */
import {
  h as h,
  d as defineComponent,
  r as ref,
  o as onMounted,
  N as onBeforeUnmount,
  n as nextTick,
} from './vendor-vue-B3zOfik3.js';
import { _ as AppLayout } from './AppLayout.vue_vue_type_script_setup_true_lang-C4ysyc_b.js';

const STYLE_URL = new URL('./usage-guide.css', import.meta.url).href;
const SECTIONS = [
  ['start', '快速开始'],
  ['connect', '接入 API'],
  ['explore', '功能导航'],
  ['faq', '常见问题'],
  ['support', '联系支持'],
];
const LANGUAGES = [
  ['curl', 'cURL'],
  ['python', 'Python'],
  ['node', 'Node.js'],
];
const FEATURES = [
  ['key', 'API 密钥', '创建密钥、选择分组，管理调用权限与额度。', '/keys'],
  ['grid', '模型广场', '查看可用模型与模型标识，选择适合任务的能力。', '/model-plaza'],
  ['canvas', '无限画布', '进入可视化创作空间，组织灵感与图像任务。', '/infinite-canvas'],
  ['chart', '使用记录', '查看请求、Token 用量与费用，排查调用问题。', '/usage'],
];
const FAQS = [
  ['基础地址应该填什么？', '使用本页展示的当前站点地址，以 /v1 结尾。若客户端要求填写完整接口地址，请使用基础地址加 /chat/completions；避免重复拼接 /v1。不同协议的客户端请先确认其接口要求。'],
  ['示例中的模型名称怎么选？', '前往模型广场查看模型标识，并确认它属于密钥所选分组支持的模型。将 YOUR_MODEL_ID 替换为支持 OpenAI 兼容 Chat Completions 接口的模型标识；展示名称与实际请求标识可能不同。'],
  ['遇到 401、403 或模型不可用怎么办？', '先检查密钥是否完整、有效，Authorization 是否使用 Bearer 格式；再检查密钥的分组、模型权限、有效期及额度。模型不可用时，核对模型标识与分组是否匹配；仍无法解决可附上脱敏后的错误信息提交工单。'],
  ['请求超时或返回 429 怎么办？', '429 通常表示请求频率或配额限制，请先检查响应中的错误说明与使用记录，降低并发并稍后重试。超时请检查网络与渠道状态。自动重试前应评估是否会重复产生任务或费用。'],
  ['可以把密钥写进网页或公开仓库吗？', '请将密钥保存在自己的服务端环境变量中，避免放入前端代码、截图或公开仓库。若密钥已经泄露，请立即前往 API 密钥页面撤销并重新创建。'],
];
const ICONS = {
  arrow: ['M7 17 17 7', 'M7 7h10v10'],
  chevron: ['m9 5 7 7-7 7'],
  copy: ['M9 9h11v11H9z', 'M5 15H3V3h12v2'],
  check: ['m5 12 4 4L19 6'],
  book: ['M4 3h12a3 3 0 0 1 3 3v15H7a3 3 0 0 1-3-3V3Z', 'M4 17h15', 'M8 7h7', 'M8 11h5'],
  key: ['M14 7a4 4 0 1 1-2 7L5 21H2v-3l7-7a4 4 0 0 1 5-4Z', 'M16 9h.01'],
  grid: ['M3 3h7v7H3z', 'M14 3h7v7h-7z', 'M3 14h7v7H3z', 'M14 14h7v7h-7z'],
  canvas: ['M3 4h18v15H3z', 'm3 15 5-5 4 4 3-3 6 6', 'M15 8h.01'],
  chart: ['M4 3v17h17', 'M8 15v-4', 'M13 15V7', 'M18 15v-6'],
  support: ['M4 14v-3a8 8 0 0 1 16 0v3', 'M4 11H2v7h4v-7H4Z', 'M20 11h2v7h-4v-7h2Z', 'M20 18v3h-7'],
  info: ['M12 8h.01', 'M12 11v6'],
};

function icon(name, className = '') {
  return h('svg', { class: `ug-icon ${className}`, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': 1.7, 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'aria-hidden': 'true' }, [
    ...(name === 'info' ? [h('circle', { cx: 12, cy: 12, r: 9 })] : []),
    ...(ICONS[name] || ICONS.arrow).map(d => h('path', { d })),
  ]);
}

/** Standard HTTP examples deliberately avoid a client SDK or CLI configuration contract. */
function examples(base) {
  return {
    curl: `# 将下方占位符替换为自己的密钥与模型标识\nexport API_BASE='${base}'\nexport API_KEY='YOUR_API_KEY'\n\ncurl --fail-with-body "$API_BASE/chat/completions" \\\n  -H "Authorization: Bearer $API_KEY" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "model": "YOUR_MODEL_ID",\n    "messages": [{"role": "user", "content": "你好，请介绍一下自己。"}]\n  }'`,
    python: `# 先在运行环境中设置 API_KEY；仅使用 Python 标准库\nimport json\nimport os\nimport urllib.request\n\npayload = {\n    "model": "YOUR_MODEL_ID",\n    "messages": [{"role": "user", "content": "你好，请介绍一下自己。"}],\n}\nrequest = urllib.request.Request(\n    "${base}/chat/completions",\n    data=json.dumps(payload).encode("utf-8"),\n    headers={\n        "Authorization": f"Bearer {os.environ['API_KEY']}",\n        "Content-Type": "application/json",\n    },\n    method="POST",\n)\nwith urllib.request.urlopen(request, timeout=60) as response:\n    print(json.load(response))`,
    node: `// 先在运行环境中设置 API_KEY；使用 Node.js 内置 fetch\nasync function main() {\n  const apiKey = process.env.API_KEY;\n  if (!apiKey) throw new Error("请先设置 API_KEY 环境变量");\n\n  const response = await fetch("${base}/chat/completions", {\n    method: "POST",\n    headers: {\n      Authorization: \`Bearer \${apiKey}\`,\n      "Content-Type": "application/json",\n    },\n    body: JSON.stringify({\n      model: "YOUR_MODEL_ID",\n      messages: [{ role: "user", content: "你好，请介绍一下自己。" }],\n    }),\n  });\n  if (!response.ok) {\n    throw new Error(\`HTTP \${response.status}: \${await response.text()}\`);\n  }\n  console.log(await response.json());\n}\nmain().catch(console.error);`,
  };
}

function loadStyles(onError) {
  let link = [...document.querySelectorAll('link[rel="stylesheet"]')].find(item => item.href === STYLE_URL);
  if (link?.sheet) return () => {};
  const isNew = !link;
  if (!link) {
    link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = STYLE_URL;
  }
  link.addEventListener('error', onError);
  if (isNew) document.head.append(link);
  return () => link.removeEventListener('error', onError);
}

export const UsageGuideContent = defineComponent({
  name: 'UsageGuideContent',
  setup() {
    const root = ref(null);
    const language = ref('curl');
    const activeSection = ref('start');
    const clipboard = ref({ target: '', status: 'idle', message: '', text: '' });
    let copyAttempt = 0;
    const styleError = ref(false);
    const base = typeof window !== 'undefined' ? `${window.location.origin}/v1` : '/v1';
    const snippets = examples(base);
    let observer;
    let resetTimer;
    let detachStyles;
    let alive = true;

    async function copyText(target, text) {
      if (clipboard.value.status === 'pending') return;
      const attempt = ++copyAttempt;
      clearTimeout(resetTimer);
      clipboard.value = { target, status: 'pending', message: '正在复制…', text: '' };
      try {
        if (navigator.clipboard?.writeText && window.isSecureContext) {
          await navigator.clipboard.writeText(text);
        } else {
          const previousFocus = document.activeElement;
          const field = document.createElement('textarea');
          field.value = text;
          field.readOnly = true;
          field.style.cssText = 'position:fixed;left:0;top:0;width:1px;height:1px;opacity:0;';
          document.body.append(field);
          field.select();
          try {
            if (!document.execCommand('copy')) throw new Error('Clipboard unavailable');
          } finally {
            field.remove();
            previousFocus?.focus({ preventScroll: true });
          }
        }
        if (!alive || attempt !== copyAttempt) return;
        clipboard.value = { target, status: 'success', message: target === 'base' ? '基础地址已复制。' : '当前示例已复制，请替换占位符后运行。', text: '' };
        resetTimer = setTimeout(() => { clipboard.value = { target: '', status: 'idle', message: '', text: '' }; }, 5000);
      } catch {
        if (!alive || attempt !== copyAttempt) return;
        clipboard.value = { target, status: 'error', message: '浏览器未允许复制。请选中下方内容，使用 Ctrl+C 或 ⌘C 手动复制。', text };
      }
    }

    function goToSection(event, id) {
      const target = root.value?.querySelector(`#ug-${id}`);
      if (!target) return;
      event.preventDefault();
      activeSection.value = id;
      target.focus({ preventScroll: true });
      target.scrollIntoView({ behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth', block: 'start' });
    }

    async function selectLanguage(id, focus = false) {
      if (language.value !== id && clipboard.value.target === 'code') {
        copyAttempt++;
        clearTimeout(resetTimer);
        clipboard.value = { target: '', status: 'idle', message: '', text: '' };
      }
      language.value = id;
      if (focus) {
        await nextTick();
        root.value?.querySelector(`#ug-tab-${id}`)?.focus();
      }
    }

    function tabKey(event, index) {
      let next;
      if (event.key === 'ArrowRight') next = (index + 1) % LANGUAGES.length;
      if (event.key === 'ArrowLeft') next = (index + LANGUAGES.length - 1) % LANGUAGES.length;
      if (event.key === 'Home') next = 0;
      if (event.key === 'End') next = LANGUAGES.length - 1;
      if (next === undefined) return;
      event.preventDefault();
      selectLanguage(LANGUAGES[next][0], true);
    }

    onMounted(() => {
      detachStyles = loadStyles(() => { styleError.value = true; });
      if (typeof IntersectionObserver === 'undefined') return;
      observer = new IntersectionObserver(entries => {
        const visible = entries.filter(entry => entry.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible[0]) activeSection.value = visible[0].target.id.slice(3);
      }, { rootMargin: '-8% 0px -55% 0px', threshold: 0 });
      root.value?.querySelectorAll('[data-ug-section]').forEach(section => observer.observe(section));
    });
    onBeforeUnmount(() => {
      alive = false;
      clearTimeout(resetTimer);
      observer?.disconnect();
      detachStyles?.();
    });

    const paragraph = text => h('p', null, text);
    const routeLink = (href, label, primary = false) => h('a', { href, class: `ug-button${primary ? ' ug-button-primary' : ''}` }, [label, icon('arrow')]);
    const heading = (number, title, description) => h('div', { class: 'ug-section-heading' }, [
      h('div', { class: 'ug-heading-line' }, [h('span', { class: 'ug-section-index', 'aria-hidden': 'true' }, number), h('h2', null, title)]),
      description ? paragraph(description) : null,
    ]);
    const section = (id, content) => h('section', { id: `ug-${id}`, class: 'ug-section', tabindex: -1, 'data-ug-section': '', 'aria-label': SECTIONS.find(item => item[0] === id)[1] }, content);
    const copyButton = (target, text, label) => {
      const state = clipboard.value;
      const current = state.target === target;
      return h('button', { type: 'button', class: 'ug-button ug-copy', disabled: state.status === 'pending', 'aria-label': label, 'aria-busy': current && state.status === 'pending', onClick: () => copyText(target, text) }, [
        icon(current && state.status === 'success' ? 'check' : 'copy'),
        current && state.status === 'pending' ? '复制中…' : current && state.status === 'success' ? '已复制' : current && state.status === 'error' ? '重试复制' : '复制',
      ]);
    };
    const feedback = target => {
      const state = clipboard.value;
      const current = state.target === target;
      return h('div', { class: ['ug-feedback', current && `is-${state.status}`] }, [
        h('p', { role: 'status', 'aria-live': 'polite', 'aria-atomic': 'true' }, current ? state.message : ''),
        current && state.status === 'error' ? h('label', { class: 'ug-manual-copy' }, [
          h('span', null, '手动复制内容'),
          h('textarea', { readonly: true, value: state.text, rows: target === 'base' ? 2 : 6, onFocus: event => event.target.select() }),
        ]) : null,
      ]);
    };

    return () => h('article', { ref: root, class: 'usage-guide', 'aria-labelledby': 'ug-title' }, [
      styleError.value ? h('p', { role: 'alert' }, '页面样式加载失败，请刷新页面重试。正文与复制功能仍可使用。') : null,
      h('header', { class: 'ug-header' }, [
        h('div', { class: 'ug-eyebrow' }, [icon('book'), '接入与使用指南']),
        h('h1', { id: 'ug-title' }, '使用说明'),
        paragraph('从创建密钥到发起第一次请求，在这里找到接入方式与常用功能。'),
        h('div', { class: 'ug-header-meta' }, [h('span', { class: 'ug-badge' }, 'OpenAI 兼容 API'), h('span', null, '标准 HTTP · 无需额外 SDK')]),
      ]),
      h('div', { class: 'ug-layout' }, [
        h('aside', { class: 'ug-sidebar' }, [
          h('nav', { class: 'ug-toc', 'aria-label': '本页目录' }, [h('p', { class: 'ug-toc-title' }, '本页目录'), ...SECTIONS.map(([id, label], index) => h('a', { href: `#ug-${id}`, 'aria-current': activeSection.value === id ? 'location' : undefined, onClick: event => goToSection(event, id) }, [h('span', { 'aria-hidden': 'true' }, `0${index + 1}`), label]))]),
          h('div', { class: 'ug-sidebar-note' }, [icon('key'), h('strong', null, '让密钥留在服务端'), paragraph('示例仅展示占位符。请在自己的运行环境中配置密钥。')]),
        ]),
        h('div', { class: 'ug-content' }, [
          section('start', [
            heading('01', '快速开始', '完成三步，开始你的第一次调用。'),
            h('ol', { class: 'ug-steps' }, [
              ['创建一把密钥', '选择适合的分组，确认密钥的有效期与可用额度。', '/keys', '前往 API 密钥'],
              ['选定可用模型', '在模型广场查看模型标识，确认分组与接口支持情况。', '/model-plaza', '浏览模型广场'],
              ['发起首次请求', '复制下方基础地址，替换示例中的密钥与模型标识。', '#ug-connect', '查看接入示例'],
            ].map(([title, detail, href, label], index) => h('li', { class: 'ug-step', style: { '--ug-delay': `${index * 35}ms` } }, [
              h('span', { class: 'ug-step-number', 'aria-hidden': 'true' }, `0${index + 1}`),
              h('h3', null, title), paragraph(detail),
              h('a', { href, onClick: index === 2 ? event => goToSection(event, 'connect') : undefined }, [label, icon('chevron')]),
            ]))),
          ]),
          section('connect', [
            heading('02', '接入 API', '适用于支持 OpenAI 兼容 Chat Completions 接口的模型。'),
            h('div', { class: 'ug-endpoint' }, [
              h('div', { class: 'ug-endpoint-heading' }, [h('label', { for: 'ug-base-url' }, '基础地址 / Base URL'), h('span', null, '当前站点')]),
              h('div', { class: 'ug-endpoint-row' }, [h('input', { id: 'ug-base-url', value: base, readonly: true, spellcheck: false, onFocus: event => event.target.select() }), copyButton('base', base, '复制 API 基础地址')]),
              paragraph('客户端填写到 /v1；完整聊天接口为 /v1/chat/completions。'),
              feedback('base'),
            ]),
            h('div', { class: 'ug-example' }, [
              h('div', { class: 'ug-example-toolbar' }, [
                h('div', { role: 'tablist', 'aria-label': 'API 示例语言', class: 'ug-tabs' }, LANGUAGES.map(([id, label], index) => h('button', { type: 'button', role: 'tab', id: `ug-tab-${id}`, 'aria-selected': language.value === id, 'aria-controls': 'ug-code-panel', tabindex: language.value === id ? 0 : -1, class: 'ug-tab', onClick: () => selectLanguage(id), onKeydown: event => tabKey(event, index) }, label))),
                copyButton('code', snippets[language.value], '复制当前 API 示例'),
              ]),
              h('div', { id: 'ug-code-panel', role: 'tabpanel', 'aria-labelledby': `ug-tab-${language.value}`, tabindex: 0, class: 'ug-code-panel' }, [
                h('div', { class: 'ug-code-caption' }, [h('span', null, 'POST'), h('span', null, '/v1/chat/completions'), h('span', { class: 'ug-code-demo' }, '占位示例')]),
                h('pre', { class: 'ug-code', key: language.value }, [h('code', null, snippets[language.value])]),
              ]),
            ]),
            feedback('code'),
            h('div', { class: 'ug-callout' }, [icon('info'), h('p', null, [h('strong', null, '运行前请替换占位符。'), ' YOUR_API_KEY 为你的密钥，YOUR_MODEL_ID 为所选分组支持的模型标识。Python 与 Node.js 从运行环境读取 API_KEY。本页不会读取或提交真实密钥，也不会自动发起调用。'])]),
            h('p', { class: 'ug-success-hint' }, [icon('check'), '请求成功后，响应 JSON 中可查看生成内容；用量与费用可在', h('a', { href: '/usage' }, '使用记录'), '中核对。']),
          ]),
          section('explore', [
            heading('03', '功能导航', '从接入、创作到用量查询，继续探索控制台。'),
            h('div', { class: 'ug-features' }, FEATURES.map(([glyph, title, detail, href]) => h('a', { class: 'ug-feature', href }, [
              h('span', { class: 'ug-feature-icon' }, icon(glyph)),
              h('div', null, [h('h3', null, title), paragraph(detail)]), icon('arrow', 'ug-feature-arrow'),
            ]))),
          ]),
          section('faq', [
            heading('04', '常见问题', '先检查地址、密钥与模型，再查看具体错误信息。'),
            h('div', { class: 'ug-faq-list' }, FAQS.map(([question, answer]) => h('details', { class: 'ug-faq' }, [h('summary', null, [h('span', null, question), icon('chevron')]), paragraph(answer)]))),
          ]),
          section('support', [
            h('div', { class: 'ug-support' }, [
              h('span', { class: 'ug-support-icon' }, icon('support')),
              h('div', { class: 'ug-support-copy' }, [h('h2', null, '需要进一步帮助？'), paragraph('通过工单联系我们。请提供发生时间、模型标识、请求 ID（如有）与脱敏错误信息，便于定位。'), h('p', { class: 'ug-support-note' }, '提交前请移除 API 密钥、Authorization 请求头及个人敏感信息。')]),
              routeLink('/tickets', '提交工单', true),
            ]),
          ]),
          h('footer', { class: 'ug-footer' }, [h('span', null, '基于 Sub2API 开源项目 · 感谢开源作者与社区贡献'), h('a', { href: '#ug-title', onClick: event => { event.preventDefault(); const title = root.value?.querySelector('#ug-title'); title?.scrollIntoView({ block: 'start' }); root.value?.querySelector('#ug-start')?.focus({ preventScroll: true }); } }, ['返回顶部', h('span', { 'aria-hidden': 'true' }, ' ↑')])]),
        ]),
      ]),
    ]);
  },
});

export default defineComponent({
  name: 'UsageGuideView',
  setup() {
    return () => h(AppLayout, null, { default: () => h(UsageGuideContent) });
  },
});
