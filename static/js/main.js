// DTS Panel - Main JS

// Toast 通知
function showToast(msg, type = 'success') {
  const container = document.getElementById('toast-container') || (() => {
    const el = document.createElement('div');
    el.id = 'toast-container';
    el.className = 'toast-container';
    document.body.appendChild(el);
    return el;
  })();
  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.textContent = msg;
  container.appendChild(toast);
  setTimeout(() => toast.remove(), 3000);
}

// API 封装
async function api(method, url, body = null) {
  const opts = { method, headers: {} };
  if (body) {
    if (body instanceof FormData) {
      opts.body = body;
    } else {
      opts.headers['Content-Type'] = 'application/json';
      opts.body = JSON.stringify(body);
    }
  }
  const res = await fetch(url, opts);
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || `HTTP ${res.status}`);
  }
  return res.json();
}

// 系统环境
async function checkDeps() {
  const result = document.getElementById('deps-result');
  result.classList.remove('hidden');
  result.textContent = '检查中...';
  try {
    const data = await api('GET', '/system/check-deps');
    result.textContent = `OS: ${data.OS} | Arch: ${data.Arch} | CPU: ${data.CPU} | ARM64: ${data.is_arm64}`;
  } catch (e) {
    result.textContent = `错误: ${e.message}`;
  }
}

async function installDeps() {
  if (!confirm('将检查并尝试安装系统依赖，需要 sudo 权限，确认继续？')) return;
  showToast('依赖安装请求已发送，请检查服务器日志');
}

// 游戏安装
async function installSteamCMD() {
  showToast('正在安装 SteamCMD...');
  try {
    await api('POST', '/install/steamcmd');
    showToast('SteamCMD 安装完成');
    checkStatus();
  } catch (e) {
    showToast(`SteamCMD 安装失败: ${e.message}`, 'error');
  }
}

async function installGame() {
  if (!confirm('即将通过 SteamCMD 下载 DST Dedicated Server，可能需要 5-10 分钟，确认继续？')) return;
  showToast('正在安装 DST Dedicated Server...');
  try {
    await api('POST', '/install/game');
    showToast('DST Dedicated Server 安装完成');
    checkStatus();
  } catch (e) {
    showToast(`游戏安装失败: ${e.message}`, 'error');
  }
}

async function checkStatus() {
  try {
    const data = await api('GET', '/api/system/status');
    const sc = document.getElementById('steamcmd-status');
    const gs = document.getElementById('game-status');
    if (sc) sc.textContent = `SteamCMD: ${data.steamcmd ? '✓ 已安装' : '✗ 未安装'}`;
    if (gs) gs.textContent = `DST Server: ${data.game ? '✓ 已安装' : '✗ 未安装'}`;
  } catch (e) {
    showToast(`状态检查失败: ${e.message}`, 'error');
  }
}

async function verifyInstall() {
  try {
    const data = await api('GET', '/install/verify');
    showToast('验证通过');
  } catch (e) {
    showToast(`验证失败: ${e.message}`, 'error');
  }
}

async function updateGame() {
  if (!confirm('即将增量更新游戏，确认继续？')) return;
  showToast('正在更新游戏...');
  try {
    await api('POST', '/install/update');
    showToast('游戏更新完成');
  } catch (e) {
    showToast(`更新失败: ${e.message}`, 'error');
  }
}

// 实例管理
let currentInstances = [];

async function loadInstances() {
  try {
    const data = await api('GET', '/api/instances');
    currentInstances = data;
    renderInstances(data);
  } catch (e) {
    showToast(`加载实例失败: ${e.message}`, 'error');
  }
}

function renderInstances(instances) {
  const tbody = document.getElementById('instance-tbody');
  if (!tbody) return;
  if (instances.length === 0) {
    tbody.innerHTML = '<tr><td colspan="5" class="empty-msg">暂无实例</td></tr>';
    return;
  }
  tbody.innerHTML = instances.map(i => `
    <tr>
      <td>${i.name}</td>
      <td><span class="badge badge-${i.status}">${i.status}</span></td>
      <td>${i.master_port} / ${i.cluster_port}</td>
      <td>${i.max_players}</td>
      <td>
        <button class="btn btn-sm" onclick="instanceAction(${i.id}, 'start')">启动</button>
        <button class="btn btn-sm" onclick="instanceAction(${i.id}, 'stop')">停止</button>
        <button class="btn btn-sm btn-danger" onclick="instanceAction(${i.id}, 'delete')">删除</button>
      </td>
    </tr>
  `).join('');
}

async function instanceAction(id, action) {
  const actions = { start: '启动', stop: '停止', delete: '删除' };
  if (action === 'delete') {
    if (!confirm(`确认删除实例 ID=${id}？`)) return;
  }
  const form = new FormData();
  form.append('action', action);
  form.append('id', String(id));
  try {
    await api('POST', '/instances/action', form);
    showToast(`${actions[action]}成功`);
    loadInstances();
  } catch (e) {
    showToast(`${actions[action]}失败: ${e.message}`, 'error');
  }
}

// 实例创建弹窗
function showCreateModal() {
  document.getElementById('create-modal').classList.remove('hidden');
}
function hideCreateModal() {
  document.getElementById('create-modal').classList.add('hidden');
}

document.addEventListener('DOMContentLoaded', () => {
  const createForm = document.getElementById('create-form');
  if (createForm) {
    createForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const form = new FormData(createForm);
      form.append('json', '1');
      try {
        await api('POST', '/instances/create', form);
        showToast('实例创建成功');
        hideCreateModal();
        loadInstances();
      } catch (e) {
        showToast(`创建失败: ${e.message}`, 'error');
      }
    });
  }

  // 加载数据
  loadInstances();

  // 加载实例选择器
  loadInstanceSelect();
});

// 房间
async function loadInstanceSelect() {
  const select = document.getElementById('instance-select');
  if (!select) return;
  try {
    const data = await api('GET', '/api/instances');
    select.innerHTML = '<option value="">-- 请选择实例 --</option>';
    data.forEach(i => {
      const opt = document.createElement('option');
      opt.value = String(i.id);
      opt.textContent = i.name;
      select.appendChild(opt);
    });
  } catch (e) {
    showToast(`加载实例失败: ${e.message}`, 'error');
  }
}

async function loadRooms() {
  const select = document.getElementById('instance-select');
  if (!select || !select.value) {
    const tbody = document.getElementById('room-tbody');
    if (tbody) tbody.innerHTML = '<tr><td colspan="6" class="empty-msg">请选择实例查看房间</td></tr>';
    return;
  }
  showToast('房间加载功能开发中...');
}

function showCreateRoomModal() {
  document.getElementById('create-room-modal').classList.remove('hidden');
}
function hideCreateRoomModal() {
  document.getElementById('create-room-modal').classList.add('hidden');
}

// 模组
async function loadMods() {
  const tbody = document.getElementById('mod-tbody');
  if (!tbody) return;
  try {
    const data = await api('GET', '/api/mods');
    if (data.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty-msg">暂无模组</td></tr>';
      return;
    }
    tbody.innerHTML = data.map(m => `
      <tr>
        <td><code>${m.mod_id}</code></td>
        <td>${m.mod_name}</td>
        <td>${m.mod_url || '-'}</td>
        <td><span class="badge ${m.enabled ? 'badge-running' : 'badge-stopped'}">${m.enabled ? '启用' : '禁用'}</span></td>
        <td>
          <button class="btn btn-sm" onclick="toggleMod('${m.mod_id}', ${!m.enabled})">${m.enabled ? '禁用' : '启用'}</button>
          <button class="btn btn-sm btn-danger" onclick="deleteMod('${m.mod_id}')">删除</button>
        </td>
      </tr>
    `).join('');
  } catch (e) {
    showToast(`加载模组失败: ${e.message}`, 'error');
  }
}

async function toggleMod(modId, enabled) {
  const form = new FormData();
  form.append('action', enabled ? 'enable' : 'disable');
  form.append('mod_id', modId);
  try {
    await api('POST', '/mods/action', form);
    showToast(`模组已${enabled ? '启用' : '禁用'}`);
    loadMods();
  } catch (e) {
    showToast(`操作失败: ${e.message}`, 'error');
  }
}

async function deleteMod(modId) {
  if (!confirm(`确认删除模组 ${modId}？`)) return;
  const form = new FormData();
  form.append('action', 'delete');
  form.append('mod_id', modId);
  try {
    await api('POST', '/mods/action', form);
    showToast('模组已删除');
    loadMods();
  } catch (e) {
    showToast(`删除失败: ${e.message}`, 'error');
  }
}

function showAddModModal() {
  document.getElementById('add-mod-modal').classList.remove('hidden');
}
function hideAddModModal() {
  document.getElementById('add-mod-modal').classList.add('hidden');
}

document.addEventListener('DOMContentLoaded', () => {
  const addModForm = document.getElementById('add-mod-form');
  if (addModForm) {
    addModForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const form = new FormData(addModForm);
      form.append('action', 'add');
      try {
        await api('POST', '/mods/action', form);
        showToast('模组添加成功');
        hideAddModModal();
        loadMods();
      } catch (e) {
        showToast(`添加失败: ${e.message}`, 'error');
      }
    });
  }

  loadMods();
});
