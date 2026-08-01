/* DTS Panel — 日志模块 v8 (fetch polling)
 *
 * 修复说明：
 *  - 服务端 /api/logs/sse 用 tail -n N 每次返回全部 N 行，
 *    不能做增量追加，否则 "上一次最后一行" 永远能在返回中找到。
 *  - 改为：首次收到数据时清空预置文本并整体写入，
 *    之后只做 "新内容追加"（服务端返回尾部不变时不动）。
 *  - 忽略 ping 事件（event: ping / data: connected），只取 event: log 的内容。
 */
var DTSLiveLog = {
  _timers: {},
  _instanceId: 0,

  init: function(el, statusEl, opts) {
    if (!el) return null;
    opts = opts || {};
    var source = opts.source || "panel";
    var lines = opts.lines || 200;
    var id = ++this._instanceId;

    var params = "source=" + source + "&lines=" + lines;
    if (source === "cluster" && opts.clusterName && opts.world) {
      params += "&cluster_name=" + encodeURIComponent(opts.clusterName);
      params += "&world=" + encodeURIComponent(opts.world);
    }
    var url = "/api/logs/sse?" + params;

    var instance = {
      id: id, el: el, statusEl: statusEl, url: url,
      timer: null, paused: false, hidden: false,
      buffer: [],         // 暂停/折叠期间缓存的完整数据快照
      _prevText: null,    // 上一次成功渲染的完整文本
      _toggleBtn: null, _pauseBtn: null,
      toggle: function() { this.hidden ? this.show() : this.hide(); },
      hide: function() {
        this.hidden = true; this.el.style.display = "none";
        if (this._toggleBtn) this._toggleBtn.textContent = "▴ 展开";
        if (this.statusEl) this.statusEl.textContent = "已折叠";
      },
      show: function() {
        this.hidden = false; this.el.style.display = "";
        if (this._toggleBtn) this._toggleBtn.textContent = "▾ 折叠";
        if (!this.paused && this.statusEl) this.statusEl.textContent = "● 在线";
        this._flushBuffer();
      },
      pause: function() {
        this.paused = true;
        if (this._pauseBtn) this._pauseBtn.textContent = "▶ 继续";
        if (this.statusEl && !this.hidden) this.statusEl.textContent = "已暂停";
      },
      resume: function() {
        this.paused = false;
        if (this._pauseBtn) this._pauseBtn.textContent = "⏸ 暂停";
        if (this.statusEl && !this.hidden) this.statusEl.textContent = "● 在线";
        this._flushBuffer();
      },
      togglePause: function() { this.paused ? this.resume() : this.pause(); },
      _flushBuffer: function() {
        // 缓冲区只保留最后一帧完整快照
        if (this.buffer.length > 0) {
          this._render(this.el, this.buffer[this.buffer.length - 1]);
          this.buffer = [];
        }
      },
      _cache: function(data) { this.buffer.push(data); },
      close: function() { if (this.timer) { clearTimeout(this.timer); this.timer = null; } }
    };

    var self = this;

    var stepCard = el.closest('.step-card');
    var titleBar = null;
    if (stepCard) {
      titleBar = stepCard.querySelector('div[style*="justify-content:space-between"], div[style*="justify-content: space-between"]');
    }
    var pauseBtn = document.createElement('button');
    pauseBtn.className = 'btn btn-sm'; pauseBtn.textContent = '⏸ 暂停';
    instance._pauseBtn = pauseBtn;
    pauseBtn.addEventListener('click', function() { instance.togglePause(); });
    var foldBtn = document.createElement('button');
    foldBtn.className = 'btn btn-sm'; foldBtn.textContent = '▾ 折叠';
    instance._toggleBtn = foldBtn;
    foldBtn.addEventListener('click', function() { instance.toggle(); });
    if (titleBar) { titleBar.appendChild(pauseBtn); titleBar.appendChild(foldBtn); }

    if (statusEl) statusEl.textContent = "连接中...";

    this._poll(instance);
    this._timers[id] = instance;
    return instance;
  },

  _poll: function(inst) {
    var self = this;
    inst.timer = setTimeout(function() {
      var u = inst.url + "&_=" + Math.random();
      fetch(u).then(function(r) {
        if (!r.ok) {
          console.warn('[DTSLiveLog] fetch failed:', r.status, u);
          if (!inst.hidden && inst.statusEl) inst.statusEl.textContent = "请求失败";
          self._poll(inst);
          return;
        }
        return r.text();
      }).then(function(text) {
        if (!text) { self._poll(inst); return; }

        // 只提取 "event: log" 后面的 data 行，忽略 ping 等其他事件
        var data = self._parseLogEvents(text);
        if (!data) { self._poll(inst); return; }

        if (inst.paused || inst.hidden) {
          inst._cache(data);
          if (!inst.hidden && inst.paused && inst.statusEl)
            inst.statusEl.textContent = "已暂停（后台记录中）";
        } else {
          self._render(inst.el, data);
          inst._prevText = data;
        }
        if (!inst.hidden && !inst.paused && inst.statusEl) inst.statusEl.textContent = "● 在线";
        self._poll(inst);
      }).catch(function(err) {
        console.warn('[DTSLiveLog] error:', err.message);
        if (!inst.hidden && inst.statusEl) inst.statusEl.textContent = "断连，重连中...";
        inst.timer = setTimeout(function() { self._poll(inst); }, 3000);
      });
    }, 1500);
  },

  // 只提取 "event: log" 事件对应的 data 行
  _parseLogEvents: function(text) {
    var lines = text.split("\n");
    var inLog = false;
    var data = "";
    for (var i = 0; i < lines.length; i++) {
      var line = lines[i].trim();
      if (line === "event: log") {
        inLog = true;
        continue;
      }
      // 空行表示一个 SSE 事件结束
      if (line === "") {
        inLog = false;
        continue;
      }
      if (inLog && line.indexOf("data: ") === 0) {
        data += line.substring(6) + "\n";
      }
    }
    return data || null;
  },

  // 首次：替换预置文本；之后：只追加真正的新行
  _render: function(el, newText) {
    // 首次有内容时清空预置文本
    if (this._prevText === null) {
      el.textContent = newText;
      el.scrollTop = el.scrollHeight;
    } else {
      // 从 _prevText 末尾找新增内容
      if (newText.indexOf(this._prevText) === 0 && newText.length > this._prevText.length) {
        var appended = newText.substring(this._prevText.length);
        el.textContent += appended;
        el.scrollTop = el.scrollHeight;
      } else if (newText !== this._prevText) {
        // 文本不同但不是前缀关系 → 整体刷新（日志文件被截断/轮转等情况）
        el.textContent = newText;
        el.scrollTop = el.scrollHeight;
      }
    }
  }
};
