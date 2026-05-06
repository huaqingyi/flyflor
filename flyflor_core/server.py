from __future__ import annotations

import json
import os
import shutil
import subprocess
import time
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

from flyflor_core.blackboard import Blackboard
from flyflor_core.config import Config
from flyflor_core.setup import default_flyflor_config, load_flyflor_config, merge_user_setup, save_flyflor_config


APP_HTML = r"""<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Flyflor Console</title>
  <style>
    :root {
      color-scheme: dark;
      --bg: #10131a;
      --panel: #171b24;
      --panel-2: #1e2430;
      --line: #303848;
      --text: #e8edf8;
      --muted: #8d9ab2;
      --cyan: #59d7ff;
      --violet: #b68cff;
      --pink: #ff72d2;
      --green: #78e08f;
      --yellow: #ffd166;
      --danger: #ff6b7a;
      --shadow: 0 18px 60px rgba(0, 0, 0, .35);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      height: 100vh;
      overflow: hidden;
      background:
        linear-gradient(135deg, rgba(89, 215, 255, .10), transparent 32%),
        linear-gradient(225deg, rgba(255, 114, 210, .10), transparent 34%),
        linear-gradient(180deg, #111520 0%, #0b0d13 100%);
      color: var(--text);
      font: 14px/1.5 Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    button, input, select, textarea { font: inherit; }
    .app { height: 100vh; display: grid; grid-template-rows: 56px 1fr; }
    header {
      display: flex; align-items: center; justify-content: space-between; gap: 16px;
      padding: 0 20px; border-bottom: 1px solid rgba(255,255,255,.08);
      background: rgba(13, 16, 22, .72); backdrop-filter: blur(18px);
    }
    .brand { display: flex; align-items: center; gap: 12px; min-width: 260px; }
    .mark {
      width: 34px; height: 34px; display: grid; place-items: center;
      border: 1px solid rgba(89,215,255,.45); border-radius: 8px;
      color: var(--cyan); background: linear-gradient(135deg, rgba(89,215,255,.16), rgba(255,114,210,.12));
      box-shadow: 0 0 24px rgba(89,215,255,.18);
      font-weight: 800;
      overflow: hidden;
    }
    .mark img { width: 100%; height: 100%; object-fit: cover; display: block; }
    .brand h1 { margin: 0; font-size: 16px; letter-spacing: 0; }
    .brand p { margin: -2px 0 0; color: var(--muted); font-size: 12px; }
    .top-controls { display: flex; align-items: center; gap: 10px; color: var(--muted); }
    .pill {
      display: inline-flex; align-items: center; gap: 7px; height: 30px; padding: 0 10px;
      border: 1px solid rgba(255,255,255,.10); border-radius: 7px;
      background: rgba(255,255,255,.04); color: var(--text);
    }
    .dot { width: 7px; height: 7px; border-radius: 99px; background: var(--green); box-shadow: 0 0 14px var(--green); }
    .main { display: grid; grid-template-columns: minmax(360px, 43%) 1fr; gap: 1px; min-height: 0; background: rgba(255,255,255,.08); }
    .pane { min-height: 0; background: rgba(16,19,26,.92); display: grid; grid-template-rows: 48px 1fr; }
    .pane-head {
      display: flex; align-items: center; justify-content: space-between; padding: 0 18px;
      border-bottom: 1px solid rgba(255,255,255,.08); background: rgba(255,255,255,.025);
    }
    .pane-title { font-weight: 700; }
    .pane-sub { color: var(--muted); font-size: 12px; }
    .chat { min-height: 0; display: grid; grid-template-rows: 1fr auto; }
    .messages { min-height: 0; overflow: auto; padding: 18px; }
    .message { display: grid; grid-template-columns: 34px 1fr; gap: 10px; margin-bottom: 14px; }
    .avatar {
      width: 34px; height: 34px; border-radius: 8px; display: grid; place-items: center;
      background: rgba(89,215,255,.13); color: var(--cyan); border: 1px solid rgba(89,215,255,.28); font-weight: 800;
    }
    .message.user .avatar { color: var(--green); background: rgba(120,224,143,.10); border-color: rgba(120,224,143,.26); }
    .bubble {
      border: 1px solid rgba(255,255,255,.08); background: var(--panel);
      border-radius: 8px; padding: 10px 12px; white-space: pre-wrap; overflow-wrap: anywhere;
    }
    .message.user .bubble { background: rgba(120,224,143,.08); }
    .composer { padding: 14px 18px 18px; border-top: 1px solid rgba(255,255,255,.08); background: rgba(0,0,0,.14); }
    .composer textarea {
      width: 100%; min-height: 84px; resize: none; border: 1px solid rgba(255,255,255,.12); outline: none;
      border-radius: 8px; padding: 12px; background: #0d1016; color: var(--text);
    }
    .composer-row { display: flex; justify-content: space-between; align-items: center; margin-top: 10px; gap: 12px; }
    .hint { color: var(--muted); font-size: 12px; }
    .send {
      height: 34px; padding: 0 14px; border: 0; border-radius: 7px; color: #091018;
      background: linear-gradient(135deg, var(--cyan), var(--violet)); font-weight: 800; cursor: pointer;
    }
    .send:disabled { opacity: .52; cursor: wait; }
    .board { min-height: 0; display: grid; grid-template-rows: auto 1fr 132px; }
    .task-strip {
      display: grid; grid-template-columns: 1fr auto; gap: 12px; padding: 14px 18px;
      border-bottom: 1px solid rgba(255,255,255,.08);
    }
    .task-title { font-weight: 800; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .status {
      height: 26px; display: inline-flex; align-items: center; border-radius: 6px; padding: 0 8px;
      background: rgba(255,209,102,.12); color: var(--yellow); border: 1px solid rgba(255,209,102,.24); font-size: 12px; font-weight: 800;
    }
    .status.done { color: var(--green); background: rgba(120,224,143,.10); border-color: rgba(120,224,143,.24); }
    .status.failed, .status.failed_runner { color: var(--danger); background: rgba(255,107,122,.10); border-color: rgba(255,107,122,.24); }
    .pair {
      display: grid; grid-template-columns: 1fr 1fr; gap: 12px; padding: 14px 18px 4px;
    }
    .worker-alert {
      margin: 10px 18px 0; display: none; align-items: center; justify-content: space-between; gap: 12px;
      border: 1px solid rgba(255,209,102,.22); border-radius: 8px; padding: 10px 12px;
      background: rgba(255,209,102,.08); color: var(--text);
    }
    .worker-alert.show { display: flex; }
    .worker-alert p { margin: 0; color: var(--muted); font-size: 12px; }
    .worker-alert b { color: var(--yellow); }
    .worker-alert button {
      height: 30px; border: 1px solid rgba(255,255,255,.12); border-radius: 7px; padding: 0 10px;
      color: var(--text); background: rgba(255,255,255,.06); cursor: pointer; white-space: nowrap;
    }
    .worker {
      border: 1px solid rgba(255,255,255,.10); border-radius: 8px; padding: 10px;
      background: linear-gradient(135deg, rgba(89,215,255,.10), rgba(255,255,255,.02));
    }
    .worker:nth-child(2) { background: linear-gradient(135deg, rgba(255,114,210,.10), rgba(255,255,255,.02)); }
    .worker label { display: block; color: var(--muted); font-size: 12px; margin-bottom: 6px; }
    .worker select { width: 100%; color: var(--text); background: #111722; border: 1px solid rgba(255,255,255,.12); border-radius: 6px; height: 32px; padding: 0 8px; }
    .worker-status { margin-top: 7px; color: var(--muted); font-size: 12px; min-height: 18px; overflow-wrap: anywhere; }
    .worker-status.ok { color: var(--green); }
    .worker-status.missing { color: var(--yellow); }
    .mini-btn {
      height: 28px; border: 1px solid rgba(255,255,255,.12); border-radius: 7px; padding: 0 9px;
      color: var(--text); background: rgba(255,255,255,.055); cursor: pointer;
    }
    .worker-actions { display: flex; justify-content: flex-end; margin-top: 8px; gap: 8px; }
    .transcript { min-height: 0; overflow: auto; padding: 14px 18px 18px; }
    .event { max-width: 72%; margin: 0 0 14px; }
    .event.right { margin-left: auto; }
    .event .who { margin: 0 0 5px; color: var(--muted); font-size: 12px; }
    .event .card {
      border-radius: 8px; padding: 11px 12px; white-space: pre-wrap; overflow-wrap: anywhere;
      border: 1px solid rgba(255,255,255,.10); background: rgba(89,215,255,.08);
    }
    .event.right .card { background: rgba(255,114,210,.08); }
    .event.system { max-width: 100%; }
    .event.system .card { background: rgba(255,255,255,.04); color: var(--muted); }
    .summary {
      border-top: 1px solid rgba(255,255,255,.08); background: rgba(0,0,0,.16);
      padding: 14px 18px; display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px;
    }
    .metric { border: 1px solid rgba(255,255,255,.08); border-radius: 8px; padding: 10px; background: rgba(255,255,255,.035); min-width: 0; }
    .metric b { display: block; font-size: 12px; color: var(--muted); margin-bottom: 4px; }
    .metric span { overflow-wrap: anywhere; }
    .modal-backdrop {
      position: fixed; inset: 0; display: none; align-items: center; justify-content: center;
      padding: 22px; background: rgba(3,6,12,.68); backdrop-filter: blur(12px); z-index: 20;
    }
    .modal-backdrop.show { display: flex; }
    .modal {
      width: min(760px, 100%); max-height: calc(100vh - 44px); overflow: auto;
      border: 1px solid rgba(255,255,255,.12); border-radius: 10px; background: #121722;
      box-shadow: var(--shadow);
    }
    .modal-head, .modal-foot {
      display: flex; align-items: center; justify-content: space-between; gap: 12px;
      padding: 14px 16px; border-bottom: 1px solid rgba(255,255,255,.08);
    }
    .modal-foot { border-top: 1px solid rgba(255,255,255,.08); border-bottom: 0; justify-content: flex-end; }
    .modal-title { font-weight: 800; }
    .form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; padding: 16px; }
    .field { min-width: 0; }
    .field.full { grid-column: 1 / -1; }
    .field label { display: block; margin-bottom: 6px; color: var(--muted); font-size: 12px; }
    .field input, .field select, .field textarea {
      width: 100%; border: 1px solid rgba(255,255,255,.12); border-radius: 8px; outline: none;
      background: #0d1016; color: var(--text); padding: 9px 10px;
    }
    .field textarea { min-height: 88px; resize: vertical; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; }
    .check-row { display: flex; align-items: center; gap: 8px; padding-top: 24px; }
    .check-row input { width: auto; }
    .error-text { color: var(--danger); font-size: 12px; margin-right: auto; }
    @media (max-width: 840px) {
      .main { grid-template-columns: 1fr; grid-template-rows: 42% 1fr; }
      .form-grid { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <div class="app">
    <header>
      <div class="brand"><div class="mark"><img src="/avatar.png" alt="飞花" onerror="this.remove(); this.parentElement.textContent='花';" /></div><div><h1>◇飞花</h1><p>Nanobot gateway · blackboard console</p></div></div>
      <div class="top-controls">
        <span class="pill"><span class="dot"></span><span id="health">connecting</span></span>
        <span class="pill">model <b id="model">flyflor-placeholder</b></span>
      </div>
    </header>
    <main class="main">
      <section class="pane">
        <div class="pane-head"><div><div class="pane-title">Conversation</div><div class="pane-sub">用户输入与飞花最终回复</div></div></div>
        <div class="chat">
          <div id="messages" class="messages"></div>
          <div class="composer">
            <textarea id="input" placeholder="输入任务，Enter 发送，Shift+Enter 换行"></textarea>
            <div class="composer-row"><span class="hint">任务由 nanobot 顶层接入，进入右侧 worker 守护黑板。</span><button class="send" id="send">Send</button></div>
          </div>
        </div>
      </section>
      <section class="pane">
        <div class="pane-head"><div><div class="pane-title">Blackboard</div><div class="pane-sub" id="taskSub">No task selected</div></div><span id="status" class="status">IDLE</span></div>
        <div class="board">
          <div>
            <div class="task-strip"><div class="task-title" id="taskTitle">等待任务</div><div class="pane-sub" id="taskId">#-</div></div>
            <div class="pair">
              <div class="worker"><label>Worker A</label><select id="leftWorker"></select><div id="leftWorkerStatus" class="worker-status"></div><div class="worker-actions"><button class="mini-btn" id="leftConfig">Configure</button></div></div>
              <div class="worker"><label>Worker B</label><select id="rightWorker"></select><div id="rightWorkerStatus" class="worker-status"></div><div class="worker-actions"><button class="mini-btn" id="rightConfig">Configure</button></div></div>
            </div>
            <div id="workerAlert" class="worker-alert"><p id="workerAlertText"></p><button id="workerInstall">Install</button></div>
          </div>
          <div class="transcript" id="transcript"></div>
          <div class="summary">
            <div class="metric"><b>Events</b><span id="metricEvents">0</span></div>
            <div class="metric"><b>Tokens</b><span>pending</span></div>
            <div class="metric"><b>Latest</b><span id="metricLatest">-</span></div>
          </div>
        </div>
      </section>
    </main>
  </div>
  <div id="workerModal" class="modal-backdrop">
    <div class="modal">
      <div class="modal-head"><div><div class="modal-title">Worker Configure</div><div class="pane-sub">配置 CLI/Agent 的启动命令、隔离环境和安装命令</div></div><button class="mini-btn" id="workerClose">Close</button></div>
      <div class="form-grid">
        <div class="field"><label>Name</label><input id="cfgName" placeholder="codex / claude / custom-worker" /></div>
        <div class="field"><label>Kind</label><select id="cfgKind"><option value="cli">cli</option><option value="agent">agent</option></select></div>
        <div class="field check-row"><input id="cfgEnabled" type="checkbox" /><label for="cfgEnabled">Enabled</label></div>
        <div class="field"><label>Description</label><input id="cfgDescription" placeholder="What this worker is used for" /></div>
        <div class="field full"><label>Command JSON array</label><textarea id="cfgCommand" placeholder='["codex"]'></textarea></div>
        <div class="field full"><label>Install command JSON array, optional and never version-pinned unless you type it</label><textarea id="cfgInstall" placeholder='["npm","install","-g","@openai/codex"]'></textarea></div>
        <div class="field full"><label>Environment JSON object</label><textarea id="cfgEnv" placeholder='{"CODEX_HOME":"{home}/agents/codex"}'></textarea></div>
      </div>
      <div class="modal-foot"><span id="cfgError" class="error-text"></span><button class="mini-btn" id="newWorker">New Worker</button><button class="send" id="workerSave">Save</button></div>
    </div>
  </div>
  <script>
    const state = { tasks: [], events: [], selectedId: null, setup: null, workerStatus: {}, missingWorker: null, delivered: new Set(), editingWorker: null };
    const $ = (id) => document.getElementById(id);

    async function getJSON(url, options) {
      const res = await fetch(url, options);
      if (!res.ok) throw new Error(await res.text());
      return await res.json();
    }

    function addMessage(role, text) {
      const node = document.createElement('div');
      node.className = 'message ' + (role === 'user' ? 'user' : 'flyflor');
      node.innerHTML = `<div class="avatar">${role === 'user' ? '你' : '花'}</div><div class="bubble"></div>`;
      node.querySelector('.bubble').textContent = text;
      $('messages').appendChild(node);
      $('messages').scrollTop = $('messages').scrollHeight;
    }

    function workerOptions(select, value) {
      const workers = Object.entries(state.setup?.workers || {});
      select.innerHTML = workers.map(([name, worker]) => `<option value="${name}">${name}${worker.enabled ? '' : ' · off'}</option>`).join('');
      select.value = value;
      if (!select.value && workers.length) select.value = workers[0][0];
    }

    async function loadSetup() {
      state.setup = await getJSON('/api/setup');
      $('model').textContent = state.setup.primary?.model || 'flyflor-placeholder';
      const pair = state.setup.bridge?.guardianPair || {left: 'codex', right: 'claude'};
      workerOptions($('leftWorker'), pair.left);
      workerOptions($('rightWorker'), pair.right);
      $('health').textContent = 'online';
      await refreshWorkerStatus();
    }

    async function refreshWorkerStatus() {
      const data = await getJSON('/api/workers/status');
      state.workerStatus = data.workers || {};
      renderWorkerAlert();
    }

    function renderWorkerAlert() {
      renderOneWorkerStatus('leftWorker', 'leftWorkerStatus');
      renderOneWorkerStatus('rightWorker', 'rightWorkerStatus');
      const selected = [$('leftWorker').value, $('rightWorker').value].filter(Boolean);
      const missing = selected.map(name => state.workerStatus[name]).find(item => item && !item.available);
      state.missingWorker = missing || null;
      if (!missing) {
        $('workerAlert').classList.remove('show');
        return;
      }
      const command = (missing.command || []).join(' ');
      const install = (missing.install || []).join(' ');
      $('workerAlertText').innerHTML = `<b>${missing.name}</b> 未检测到命令 <b>${command}</b>。${install ? '可以自动安装：' + install : '这是自定义/未知工具，请在 setup 中修改 command 或手动安装。'}`;
      $('workerInstall').style.display = install ? 'inline-flex' : 'none';
      $('workerAlert').classList.add('show');
    }

    function renderOneWorkerStatus(selectId, statusId) {
      const item = state.workerStatus[$(selectId).value];
      const node = $(statusId);
      if (!item) { node.textContent = '未检测'; node.className = 'worker-status missing'; return; }
      if (item.available) {
        node.textContent = item.version ? `已安装 · ${item.version}` : `已安装 · ${item.path}`;
        node.className = 'worker-status ok';
        return;
      }
      node.textContent = `未安装 · ${(item.command || []).join(' ')}`;
      node.className = 'worker-status missing';
    }

    function pretty(value) {
      return JSON.stringify(value, null, 2);
    }

    function parseJsonField(id, fallback) {
      const raw = $(id).value.trim();
      if (!raw) return fallback;
      return JSON.parse(raw);
    }

    function openWorkerConfig(name, pairSide = '') {
      state.editingWorker = { name, pairSide };
      const worker = state.setup?.workers?.[name] || { enabled: true, kind: 'cli', command: [name], install: [], env: {}, description: '' };
      $('cfgName').value = name;
      $('cfgName').disabled = Boolean(state.setup?.workers?.[name]);
      $('cfgEnabled').checked = Boolean(worker.enabled);
      $('cfgKind').value = worker.kind || 'cli';
      $('cfgDescription').value = worker.description || '';
      $('cfgCommand').value = pretty(worker.command || [name]);
      $('cfgInstall').value = pretty(worker.install || []);
      $('cfgEnv').value = pretty(worker.env || {});
      $('cfgError').textContent = '';
      $('workerModal').classList.add('show');
    }

    function newWorkerConfig() {
      state.editingWorker = { name: '', pairSide: '' };
      $('cfgName').disabled = false;
      $('cfgName').value = 'custom-worker';
      $('cfgEnabled').checked = true;
      $('cfgKind').value = 'cli';
      $('cfgDescription').value = '';
      $('cfgCommand').value = pretty(['custom-worker']);
      $('cfgInstall').value = pretty([]);
      $('cfgEnv').value = pretty({});
      $('cfgError').textContent = '';
      $('workerModal').classList.add('show');
    }

    async function saveWorkerConfig() {
      $('cfgError').textContent = '';
      let payload;
      try {
        payload = {
          name: $('cfgName').value.trim(),
          enabled: $('cfgEnabled').checked,
          kind: $('cfgKind').value,
          description: $('cfgDescription').value.trim(),
          command: parseJsonField('cfgCommand', []),
          install: parseJsonField('cfgInstall', []),
          env: parseJsonField('cfgEnv', {}),
          pairSide: state.editingWorker?.pairSide || '',
        };
      } catch (err) {
        $('cfgError').textContent = 'JSON 解析失败：' + err.message;
        return;
      }
      try {
        const data = await getJSON('/api/workers/save', { method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload) });
        state.setup = data.setup;
        const pair = state.setup.bridge?.guardianPair || {left: $('leftWorker').value, right: $('rightWorker').value};
        workerOptions($('leftWorker'), pair.left);
        workerOptions($('rightWorker'), pair.right);
        $('workerModal').classList.remove('show');
        await refreshWorkerStatus();
        addMessage('flyflor', `已保存 worker 配置：${payload.name}`);
      } catch (err) {
        $('cfgError').textContent = err.message;
      }
    }

    async function installMissingWorker() {
      if (!state.missingWorker?.name) return;
      const name = state.missingWorker.name;
      $('workerInstall').disabled = true;
      $('workerInstall').textContent = 'Installing';
      addMessage('flyflor', `正在安装 ${name}，安装日志会写入 worker 检测结果。`);
      try {
        const data = await getJSON('/api/workers/install', { method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify({ name }) });
        addMessage('flyflor', data.ok ? `${name} 安装完成。` : `${name} 安装失败：\n${data.output || data.error || 'unknown error'}`);
      } catch (err) {
        addMessage('flyflor', `${name} 安装请求失败：${err.message}`);
      } finally {
        $('workerInstall').disabled = false;
        $('workerInstall').textContent = 'Install';
        await refreshWorkerStatus();
      }
    }

    async function createTask() {
      const text = $('input').value.trim();
      if (!text) return;
      $('input').value = '';
      addMessage('user', text);
      const left = $('leftWorker').value;
      const right = $('rightWorker').value;
      const payload = { title: text.slice(0, 80), input: text, metadata: { source: 'web', guardianPair: { left, right } } };
      const data = await getJSON('/tasks', { method: 'POST', headers: {'content-type': 'application/json'}, body: JSON.stringify(payload) });
      state.selectedId = data.task.id;
      addMessage('flyflor', `已创建任务 #${data.task.id}，正在进入 ${left} / ${right} 守护讨论。`);
      await refresh();
    }

    async function refresh() {
      const tasksData = await getJSON('/tasks');
      state.tasks = tasksData.tasks || [];
      if (!state.selectedId && state.tasks.length) state.selectedId = state.tasks[0].id;
      const task = state.tasks.find(t => t.id === state.selectedId) || state.tasks[0];
      if (!task) return;
      state.selectedId = task.id;
      const eventData = await getJSON(`/tasks/${task.id}/events`);
      state.events = eventData.events || [];
      renderTask(task);
      renderEvents(task);
      syncFinalAnswers(task);
    }

    function renderTask(task) {
      $('taskTitle').textContent = task.title;
      $('taskId').textContent = `#${task.id}`;
      $('taskSub').textContent = task.input;
      $('status').textContent = String(task.status).toUpperCase();
      $('status').className = 'status ' + task.status;
      $('metricEvents').textContent = task.event_count || state.events.length;
      const latest = [...state.events].reverse().find(e => ['final_answer','message','bridge_claimed'].includes(e.event_type));
      $('metricLatest').textContent = latest ? latest.content : '-';
    }

    function renderEvents(task) {
      const left = $('leftWorker').value;
      const right = $('rightWorker').value;
      const root = $('transcript');
      root.innerHTML = '';
      for (const event of state.events.filter(e => ['message','bridge_claimed','final_answer','guardian_pair_selected'].includes(e.event_type))) {
        const side = event.actor === right ? 'right' : event.actor === left ? 'left' : 'system';
        const who = event.actor === left ? `Worker A · ${event.actor}` : event.actor === right ? `Worker B · ${event.actor}` : `飞花 · ${event.event_type}`;
        const div = document.createElement('div');
        div.className = 'event ' + side;
        div.innerHTML = `<div class="who"></div><div class="card"></div>`;
        div.querySelector('.who').textContent = who;
        div.querySelector('.card').textContent = event.content;
        root.appendChild(div);
      }
      root.scrollTop = root.scrollHeight;
    }

    function syncFinalAnswers(task) {
      for (const event of state.events) {
        const key = `${task.id}:${event.id}`;
        if (event.event_type === 'final_answer' && !state.delivered.has(key)) {
          state.delivered.add(key);
          addMessage('flyflor', event.content);
        }
      }
    }

    $('send').addEventListener('click', createTask);
    $('workerInstall').addEventListener('click', installMissingWorker);
    $('leftConfig').addEventListener('click', () => openWorkerConfig($('leftWorker').value, 'left'));
    $('rightConfig').addEventListener('click', () => openWorkerConfig($('rightWorker').value, 'right'));
    $('workerClose').addEventListener('click', () => $('workerModal').classList.remove('show'));
    $('newWorker').addEventListener('click', newWorkerConfig);
    $('workerSave').addEventListener('click', saveWorkerConfig);
    $('leftWorker').addEventListener('change', renderWorkerAlert);
    $('rightWorker').addEventListener('change', renderWorkerAlert);
    $('input').addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); createTask(); }
    });

    loadSetup().then(refresh);
    setInterval(refresh, 1500);
  </script>
</body>
</html>
"""


class FlyflorHandler(BaseHTTPRequestHandler):
    blackboard: Blackboard
    config: Config

    server_version = "FlyflorCore/0.1"

    def do_GET(self) -> None:
        path = urlparse(self.path).path
        if path == "/":
            self.write_html(APP_HTML)
            return

        if path == "/avatar.png":
            self.write_avatar()
            return

        if path == "/health":
            self.write_json(
                {
                    "ok": True,
                    "database": str(self.config.database),
                    "qdrant_url": self.config.qdrant_url,
                    "qdrant_collection": self.config.qdrant_collection,
                }
            )
            return

        if path == "/api/setup":
            self.write_json(self.setup_payload())
            return

        if path == "/api/workers/status":
            self.write_json(self.worker_status_payload())
            return

        if path == "/tasks":
            self.write_json({"tasks": self.blackboard.list_tasks()})
            return

        if path.startswith("/tasks/") and path.endswith("/events"):
            task_id = self.parse_task_id(path, suffix="/events")
            if task_id is None:
                self.write_error(HTTPStatus.BAD_REQUEST, "invalid task id")
                return
            self.write_json({"events": self.blackboard.list_events(task_id)})
            return

        if path.startswith("/tasks/"):
            task_id = self.parse_task_id(path)
            if task_id is None:
                self.write_error(HTTPStatus.BAD_REQUEST, "invalid task id")
                return
            task = self.blackboard.get_task(task_id)
            if task is None:
                self.write_error(HTTPStatus.NOT_FOUND, "task not found")
                return
            self.write_json({"task": task})
            return

        if path == "/v1/models":
            self.write_json(
                {
                    "object": "list",
                    "data": [
                        {
                            "id": "flyflor-placeholder",
                            "object": "model",
                            "created": 0,
                            "owned_by": "flyflor",
                        }
                    ],
                }
            )
            return

        self.write_error(HTTPStatus.NOT_FOUND, "not found")

    def do_POST(self) -> None:
        path = urlparse(self.path).path
        if path == "/v1/chat/completions":
            self.handle_chat_completions()
            return

        if path == "/tasks":
            body = self.read_json()
            title = str(body.get("title") or "Untitled task").strip()
            input_text = str(body.get("input") or body.get("content") or "").strip()
            if not input_text:
                self.write_error(HTTPStatus.BAD_REQUEST, "missing input")
                return
            task = self.blackboard.create_task(
                title=title,
                input_text=input_text,
                metadata=body.get("metadata") if isinstance(body.get("metadata"), dict) else None,
            )
            self.write_json({"task": task.__dict__}, status=HTTPStatus.CREATED)
            return

        if path == "/api/workers/install":
            self.handle_worker_install()
            return

        if path == "/api/workers/save":
            self.handle_worker_save()
            return

        if path.startswith("/tasks/") and path.endswith("/events"):
            task_id = self.parse_task_id(path, suffix="/events")
            if task_id is None:
                self.write_error(HTTPStatus.BAD_REQUEST, "invalid task id")
                return
            body = self.read_json()
            event_id = self.blackboard.append_event(
                task_id=task_id,
                actor=str(body.get("actor") or "unknown"),
                event_type=str(body.get("event_type") or body.get("type") or "note"),
                content=str(body.get("content") or ""),
                payload=body.get("payload") if isinstance(body.get("payload"), dict) else None,
            )
            self.write_json({"event_id": event_id}, status=HTTPStatus.CREATED)
            return

        self.write_error(HTTPStatus.NOT_FOUND, "not found")

    def handle_chat_completions(self) -> None:
        body = self.read_json()
        messages = body.get("messages") if isinstance(body.get("messages"), list) else []
        user_text = self.extract_user_text(messages)
        title = self.title_from_text(user_text)
        task = self.blackboard.create_task(
            title=title,
            input_text=user_text or "Nanobot requested a Flyflor task.",
            metadata={
                "source": "openai_compatible_chat",
                "model": body.get("model"),
                "message_count": len(messages),
            },
        )
        content = (
            f"Flyflor task #{task.id} created: {task.title}\n\n"
            "I have queued this request on the blackboard. Bridge execution will be attached next."
        )
        if body.get("stream"):
            self.write_chat_stream(content)
            return
        self.write_json(
            {
                "id": f"chatcmpl-flyflor-{task.id}",
                "object": "chat.completion",
                "created": int(time.time()),
                "model": body.get("model") or "flyflor-placeholder",
                "choices": [
                    {
                        "index": 0,
                        "message": {"role": "assistant", "content": content},
                        "finish_reason": "stop",
                    }
                ],
                "usage": {
                    "prompt_tokens": 0,
                    "completion_tokens": 0,
                    "total_tokens": 0,
                },
            }
        )

    def read_json(self) -> dict[str, Any]:
        length = int(self.headers.get("content-length") or "0")
        if length == 0:
            return {}
        raw = self.rfile.read(length).decode("utf-8")
        return json.loads(raw)

    def write_json(self, payload: dict[str, Any], status: HTTPStatus = HTTPStatus.OK) -> None:
        encoded = json.dumps(payload, ensure_ascii=False, sort_keys=True).encode("utf-8")
        self.send_response(status)
        self.send_header("content-type", "application/json; charset=utf-8")
        self.send_header("content-length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def write_html(self, html: str, status: HTTPStatus = HTTPStatus.OK) -> None:
        encoded = html.encode("utf-8")
        self.send_response(status)
        self.send_header("content-type", "text/html; charset=utf-8")
        self.send_header("content-length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def write_avatar(self) -> None:
        path = Path(self.config.home / "memory" / "avatar.png")
        fallback = Path("/app/memory/avatar.png")
        if not path.exists() and fallback.exists():
            path = fallback
        if not path.exists():
            self.write_error(HTTPStatus.NOT_FOUND, "avatar not found")
            return
        data = path.read_bytes()
        self.send_response(HTTPStatus.OK)
        self.send_header("content-type", "image/png")
        self.send_header("cache-control", "public, max-age=60")
        self.send_header("content-length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def write_chat_stream(self, content: str) -> None:
        created = int(time.time())
        model = "flyflor-placeholder"
        self.send_response(HTTPStatus.OK)
        self.send_header("content-type", "text/event-stream; charset=utf-8")
        self.send_header("cache-control", "no-cache")
        self.end_headers()

        def send(payload: dict[str, Any]) -> None:
            encoded = f"data: {json.dumps(payload, ensure_ascii=False)}\n\n".encode("utf-8")
            self.wfile.write(encoded)
            self.wfile.flush()

        send(
            {
                "id": f"chatcmpl-flyflor-{created}",
                "object": "chat.completion.chunk",
                "created": created,
                "model": model,
                "choices": [{"index": 0, "delta": {"role": "assistant"}, "finish_reason": None}],
            }
        )
        send(
            {
                "id": f"chatcmpl-flyflor-{created}",
                "object": "chat.completion.chunk",
                "created": created,
                "model": model,
                "choices": [{"index": 0, "delta": {"content": content}, "finish_reason": None}],
            }
        )
        send(
            {
                "id": f"chatcmpl-flyflor-{created}",
                "object": "chat.completion.chunk",
                "created": created,
                "model": model,
                "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}],
                "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
            }
        )
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()

    def write_error(self, status: HTTPStatus, message: str) -> None:
        self.write_json({"ok": False, "error": message}, status=status)

    @staticmethod
    def parse_task_id(path: str, suffix: str = "") -> int | None:
        if suffix and path.endswith(suffix):
            path = path[: -len(suffix)]
        parts = [part for part in path.split("/") if part]
        if len(parts) != 2 or parts[0] != "tasks":
            return None
        try:
            return int(parts[1])
        except ValueError:
            return None

    @staticmethod
    def extract_user_text(messages: list[Any]) -> str:
        for message in reversed(messages):
            if not isinstance(message, dict) or message.get("role") != "user":
                continue
            content = message.get("content")
            if isinstance(content, str):
                return FlyflorHandler.strip_runtime_context(content).strip()
            if isinstance(content, list):
                parts: list[str] = []
                for item in content:
                    if isinstance(item, dict):
                        text = item.get("text")
                        if isinstance(text, str):
                            parts.append(text)
                    elif isinstance(item, str):
                        parts.append(item)
                return FlyflorHandler.strip_runtime_context("\n".join(parts)).strip()
        return ""

    @staticmethod
    def title_from_text(text: str) -> str:
        compact = " ".join(text.split())
        if not compact:
            return "Nanobot task"
        return compact[:80]

    @staticmethod
    def strip_runtime_context(text: str) -> str:
        end_marker = "[/Runtime Context]"
        if text.startswith("[Runtime Context") and end_marker in text:
            return text.split(end_marker, 1)[1].strip()
        return text

    def log_message(self, format: str, *args: Any) -> None:
        print(f"{self.address_string()} - {format % args}")

    def setup_payload(self) -> dict[str, Any]:
        data = self.raw_setup()
        primary = data.get("primary", {})
        if isinstance(primary, dict):
            primary = dict(primary)
            primary["apiKeySet"] = bool(primary.get("apiKey"))
            primary.pop("apiKey", None)
        else:
            primary = {}
        return {
            "primary": primary,
            "workers": data.get("workers", {}),
            "bridge": data.get("bridge", {}),
        }

    def worker_status_payload(self) -> dict[str, Any]:
        setup = self.raw_setup()
        workers = setup.get("workers", {})
        if not isinstance(workers, dict):
            workers = {}
        return {"workers": {name: self.detect_worker(name, value) for name, value in workers.items() if isinstance(value, dict)}}

    def detect_worker(self, name: str, worker: dict[str, Any]) -> dict[str, Any]:
        command = worker.get("command")
        if isinstance(command, str):
            command = command.split()
        if not isinstance(command, list) or not command:
            command = [name]
        command = [str(part) for part in command]
        executable = shutil.which(command[0])
        version = self.detect_version(command[0]) if executable else None
        install = worker.get("install")
        if isinstance(install, str):
            install = install.split()
        if not isinstance(install, list):
            install = []
        return {
            "name": name,
            "enabled": bool(worker.get("enabled")),
            "kind": worker.get("kind", "tui"),
            "command": command,
            "available": executable is not None,
            "path": executable,
            "version": version,
            "install": [str(part) for part in install],
            "description": worker.get("description", ""),
        }

    @staticmethod
    def detect_version(executable: str) -> str | None:
        for args in ([executable, "--version"], [executable, "version"]):
            try:
                result = subprocess.run(args, text=True, capture_output=True, timeout=5)
            except (OSError, subprocess.TimeoutExpired):
                continue
            text = ((result.stdout or "") + "\n" + (result.stderr or "")).strip()
            if result.returncode == 0 and text:
                return text.splitlines()[0][:160]
        return None

    def handle_worker_install(self) -> None:
        body = self.read_json()
        name = str(body.get("name") or "").strip()
        setup = self.raw_setup()
        workers = setup.get("workers", {})
        worker = workers.get(name) if isinstance(workers, dict) else None
        if not isinstance(worker, dict):
            self.write_error(HTTPStatus.NOT_FOUND, "worker not found")
            return
        install = worker.get("install")
        if isinstance(install, str):
            install = install.split()
        if not isinstance(install, list) or not install:
            self.write_error(HTTPStatus.BAD_REQUEST, "worker has no install command")
            return
        command = [str(part) for part in install]
        try:
            result = subprocess.run(command, text=True, capture_output=True, timeout=300, env=os.environ.copy())
        except subprocess.TimeoutExpired as exc:
            output = ((exc.stdout or "") + "\n" + (exc.stderr or "")).strip()
            self.write_json({"ok": False, "name": name, "command": command, "output": output or "install timed out"})
            return
        except OSError as exc:
            self.write_json({"ok": False, "name": name, "command": command, "output": str(exc)})
            return
        output = ((result.stdout or "") + "\n" + (result.stderr or "")).strip()
        self.write_json({"ok": result.returncode == 0, "name": name, "command": command, "output": output[-6000:]})

    def handle_worker_save(self) -> None:
        body = self.read_json()
        name = str(body.get("name") or "").strip()
        if not name:
            self.write_error(HTTPStatus.BAD_REQUEST, "worker name is required")
            return
        if any(ch.isspace() for ch in name) or "/" in name:
            self.write_error(HTTPStatus.BAD_REQUEST, "worker name cannot contain whitespace or slash")
            return

        command = body.get("command")
        install = body.get("install")
        env = body.get("env")
        if not isinstance(command, list) or not command or not all(isinstance(item, str) and item for item in command):
            self.write_error(HTTPStatus.BAD_REQUEST, "command must be a non-empty JSON string array")
            return
        if install is None:
            install = []
        if not isinstance(install, list) or not all(isinstance(item, str) and item for item in install):
            self.write_error(HTTPStatus.BAD_REQUEST, "install must be a JSON string array")
            return
        if env is None:
            env = {}
        if not isinstance(env, dict) or not all(isinstance(key, str) and isinstance(value, str) for key, value in env.items()):
            self.write_error(HTTPStatus.BAD_REQUEST, "env must be a JSON object with string values")
            return

        setup = self.raw_setup()
        generated = default_flyflor_config(core_port=self.config.port, websocket_port=8765)
        setup = merge_user_setup(setup, generated)
        workers = setup.setdefault("workers", {})
        if not isinstance(workers, dict):
            workers = {}
            setup["workers"] = workers
        existing = workers.get(name) if isinstance(workers.get(name), dict) else {}
        assert isinstance(existing, dict)
        workers[name] = {
            **existing,
            "enabled": bool(body.get("enabled")),
            "kind": str(body.get("kind") or "cli"),
            "command": [str(item) for item in command],
            "install": [str(item) for item in install],
            "env": {str(key): str(value) for key, value in env.items()},
            "description": str(body.get("description") or ""),
        }

        bridge = setup.setdefault("bridge", {})
        if isinstance(bridge, dict):
            pair = bridge.setdefault("guardianPair", {})
            if isinstance(pair, dict):
                left = str(body.get("pairSide") or "")
                if left in ("left", "right"):
                    pair[left] = name
        save_flyflor_config(self.config.home, setup)
        self.write_json({"ok": True, "worker": workers[name], "setup": self.setup_payload()})

    def raw_setup(self) -> dict[str, Any]:
        try:
            data = load_flyflor_config(self.config.home)
        except (FileNotFoundError, json.JSONDecodeError):
            return {}
        if not isinstance(data, dict):
            return {}
        generated = default_flyflor_config(core_port=self.config.port, websocket_port=8765)
        merged = merge_user_setup(data, generated)
        workers = merged.get("workers", {})
        if isinstance(workers, dict):
            for worker in workers.values():
                if isinstance(worker, dict) and worker.get("kind") == "tui":
                    worker["kind"] = "cli"
        return merged


def run_server(config: Config) -> None:
    blackboard = Blackboard(config.database)
    FlyflorHandler.blackboard = blackboard
    FlyflorHandler.config = config
    server = ThreadingHTTPServer((config.host, config.port), FlyflorHandler)
    print(f"Flyflor Core listening on http://{config.host}:{config.port}")
    server.serve_forever()
