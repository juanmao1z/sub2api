/** @file @brief Add real administrator workflow controls to compiled ticket pages. */

const ADMIN_TICKET_PATH = /^\/admin\/tickets\/(\d+)$/;
const STATUS_OPTIONS = [
  ['open', '待处理'],
  ['in_progress', '处理中'],
  ['waiting_user', '等待用户'],
  ['resolved', '已解决'],
  ['closed', '已关闭'],
];
const PRIORITY_OPTIONS = [
  ['low', '低'],
  ['normal', '普通'],
  ['high', '高'],
  ['urgent', '紧急'],
];

function ticketID() {
  return window.location.pathname.match(ADMIN_TICKET_PATH)?.[1] || '';
}

async function request(path, options = {}) {
	const token = window.localStorage.getItem('auth_token');
	const authorization = token ? { Authorization: `Bearer ${token}` } : {};
	const response = await fetch(`/api/v1${path}`, {
		credentials: 'include',
		headers: { 'Content-Type': 'application/json', ...authorization, ...(options.headers || {}) },
		...options,
	});
	const payload = await response.json().catch(() => ({}));
	if (!response.ok) throw new Error(payload.message || `Request failed (${response.status})`);
	return payload.data ?? payload;
}

function option(value, label) {
  const element = document.createElement('option');
  element.value = value;
  element.textContent = label;
  return element;
}

function field(labelText, control) {
  const label = document.createElement('label');
  label.className = 'zhouz-ticket-admin-field';
  const text = document.createElement('span');
  text.textContent = labelText;
  label.append(text, control);
  return label;
}

function selectControl(name, choices) {
  const select = document.createElement('select');
  select.className = 'zhouz-ticket-admin-select';
  select.name = name;
  choices.forEach(([value, label]) => select.append(option(value, label)));
  return select;
}


async function mount(id, host) {
  if (host.querySelector('.zhouz-ticket-admin-tools[data-ticket-id="' + id + '"]')) return;
  const panel = document.createElement('section');
  panel.className = 'zhouz-ticket-admin-tools';
  panel.dataset.ticketId = id;
  panel.setAttribute('aria-labelledby', `zhouz-ticket-admin-title-${id}`);
  panel.innerHTML = `
    <div class="zhouz-ticket-admin-heading">
      <div>
        <h3 id="zhouz-ticket-admin-title-${id}">管理员工作流</h3>
        <p>更新路由字段或添加内部备注，内部备注仅管理员可见。</p>
      </div>
      <span class="zhouz-ticket-admin-state" role="status"></span>
    </div>
    <div class="zhouz-ticket-admin-grid">
      <div class="zhouz-ticket-admin-controls"></div>
      <div class="zhouz-ticket-admin-tags">
        <span class="zhouz-ticket-admin-label">标签</span>
        <div class="zhouz-ticket-admin-tag-list"></div>
      </div>
    </div>
    <label class="zhouz-ticket-admin-note">
      <span>内部备注</span>
      <textarea maxlength="10000" rows="3" placeholder="仅管理员可见"></textarea>
    </label>
    <div class="zhouz-ticket-admin-actions">
      <button type="button" class="zhouz-ticket-admin-button is-primary">保存工作流变更</button>
    </div>`;

  const controls = panel.querySelector('.zhouz-ticket-admin-controls');
  const status = selectControl('status', STATUS_OPTIONS);
  const priority = selectControl('priority', PRIORITY_OPTIONS);
  const assignee = document.createElement('select');
  assignee.className = 'zhouz-ticket-admin-select';
  assignee.name = 'assigned_to';
  assignee.append(option('', '未分配'));
  controls.append(
    field('状态', status),
    field('优先级', priority),
    field('负责人', assignee),
  );

  const tagList = panel.querySelector('.zhouz-ticket-admin-tag-list');
  const note = panel.querySelector('textarea');
  const save = panel.querySelector('.zhouz-ticket-admin-actions button');
  const state = panel.querySelector('.zhouz-ticket-admin-state');
  const setState = (message, error = false) => {
    state.textContent = message;
    state.dataset.error = error ? 'true' : 'false';
  };

  host.prepend(panel);
  try {
    const [detail, tags, assignees] = await Promise.all([
      request(`/admin/tickets/${id}`),
      request('/admin/tickets/tags'),
      request('/admin/tickets/assignees'),
    ]);
    const ticket = detail.ticket || detail;
    status.value = ticket.status || 'open';
    priority.value = ticket.priority || 'normal';
    (assignees.items || assignees || []).forEach(item => {
      const label = item.username || item.email || `#${item.id}`;
      assignee.append(option(String(item.id), label));
    });
    assignee.value = ticket.assigned_to ? String(ticket.assigned_to) : '';
    const currentTags = new Set((detail.tags || []).map(item => String(item.id)));
    (tags.items || tags || []).forEach(item => {
      const label = document.createElement('label');
      label.className = 'zhouz-ticket-admin-tag';
      const checkbox = document.createElement('input');
      checkbox.type = 'checkbox';
      checkbox.value = String(item.id);
      checkbox.checked = currentTags.has(String(item.id));
      const text = document.createElement('span');
      text.textContent = item.name || item.slug;
      label.append(checkbox, text);
      tagList.append(label);
    });
    setState('就绪');
  } catch (error) {
    setState(error.message, true);
    save.disabled = true;
  }

  save.addEventListener('click', async () => {
    if (save.disabled) return;
    save.disabled = true;
    setState('保存中...');
    try {
      const metadata = {
        status: status.value,
        priority: priority.value,
        ...(assignee.value ? { assigned_to: Number(assignee.value) } : { clear_owner: true }),
      };
      await request(`/admin/tickets/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(metadata),
      });
      const tagIDs = [...tagList.querySelectorAll('input:checked')].map(input => Number(input.value));
      await request(`/admin/tickets/${id}/tags`, {
        method: 'PUT',
        body: JSON.stringify({ tag_ids: tagIDs }),
      });
      if (note.value.trim()) {
        await request(`/admin/tickets/${id}/internal-notes`, {
          method: 'POST',
          body: JSON.stringify({ content: note.value.trim() }),
        });
      }
      setState('已保存');
      note.value = '';
      window.setTimeout(() => window.location.reload(), 350);
    } catch (error) {
      setState(error.message, true);
      save.disabled = false;
    }
  });
}

function sync() {
  const id = ticketID();
  if (!id) return;
  const detail = document.querySelector('.tickets-detail');
  if (detail) mount(id, detail);
}

export function installTicketAdminTools() {
  if (window.__zhouzTicketAdminToolsInstalled) return;
  window.__zhouzTicketAdminToolsInstalled = true;
  let lastPath = '';
  const tick = () => {
    if (lastPath !== window.location.pathname) {
      lastPath = window.location.pathname;
      window.setTimeout(sync, 50);
    } else {
      sync();
    }
  };
  const observer = new MutationObserver(tick);
  observer.observe(document.body, { childList: true, subtree: true });
  window.setInterval(tick, 500);
  tick();
}
