(function () {
  'use strict';

  var LANG_STORAGE_KEY = 'stargate-suite-lang';
  var SCENARIO_STORAGE_KEY = 'stargate-suite-scenario';
  var DEFAULT_GENERATE_MODES = ['traefik'];
  var MAX_IMPORT_TOTAL_BYTES = 512 * 1024;

  function q(selector, root) {
    return (root || document).querySelector(selector);
  }

  function qa(selector, root) {
    return Array.prototype.slice.call((root || document).querySelectorAll(selector));
  }

  function escapeHtml(text) {
    var div = document.createElement('div');
    div.textContent = text == null ? '' : String(text);
    return div.innerHTML;
  }

  function formatMessage(message, values) {
    var out = String(message || '');
    Object.keys(values || {}).forEach(function (key) {
      out = out.replace(new RegExp('\\{' + key + '\\}', 'g'), String(values[key]));
    });
    return out;
  }

  function getI18N(lang) {
    var dict = window.I18N || {};
    return dict[lang] || dict.zh || {};
  }

  function getLang() {
    return localStorage.getItem(LANG_STORAGE_KEY) || 'zh';
  }

  function applyLang(lang) {
    var dict = window.I18N || {};
    if (!dict[lang]) lang = 'zh';
    localStorage.setItem(LANG_STORAGE_KEY, lang);
    document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
    var t = getI18N(lang);
    if (t.title) document.title = t.title;
    qa('[data-i18n]').forEach(function (el) {
      var key = el.getAttribute('data-i18n');
      if (key && t[key] !== undefined) el.textContent = t[key];
    });
    qa('[data-i18n-placeholder]').forEach(function (el) {
      var key = el.getAttribute('data-i18n-placeholder');
      if (key && t[key] !== undefined) el.placeholder = t[key];
    });
    qa('[data-i18n-aria-label]').forEach(function (el) {
      var key = el.getAttribute('data-i18n-aria-label');
      if (key && t[key] !== undefined) el.setAttribute('aria-label', t[key]);
    });
    qa('.lang-link').forEach(function (el) {
      el.classList.toggle('active', el.getAttribute('data-lang') === lang);
    });
  }

  function bindLangSwitch() {
    qa('.lang-link').forEach(function (link) {
      link.addEventListener('click', function (event) {
        event.preventDefault();
        applyLang(this.getAttribute('data-lang'));
        renderScenarioPresets();
      });
    });
    applyLang(getLang());
  }

  function updateOptionDependents() {
    qa('[data-depends-on-option]').forEach(function (el) {
      var key = el.getAttribute('data-depends-on-option');
      var input = document.getElementById(key);
      var isOn = false;
      if (input) {
        if (input.type === 'checkbox') isOn = input.checked;
        else if (input.tagName === 'SELECT') isOn = input.value === 'true';
      }
      el.style.display = isOn ? '' : 'none';
    });
    qa('[data-depends-on-env]').forEach(function (el) {
      var key = el.getAttribute('data-depends-on-env');
      var scope = el.closest('.panel-details') || el.closest('form') || document;
      var input = q('[data-env="' + key + '"]', scope);
      var isOn = false;
      if (input) {
        if (input.type === 'checkbox') isOn = input.checked;
        else if (input.tagName === 'SELECT') isOn = input.value === 'true';
        else isOn = !!input.value;
      }
      el.style.display = isOn ? '' : 'none';
    });
  }

  function updateMutexOptions() {
    qa('[data-disables-option]').forEach(function (src) {
      var targetId = src.getAttribute('data-disables-option');
      var target = document.getElementById(targetId);
      if (!target) return;
      var disabled = !!(src.type === 'checkbox' && src.checked);
      target.disabled = disabled;
      if (disabled) {
        target.checked = false;
        var hidden = target.form && target.form.querySelector('input[type="hidden"][name="' + target.name + '"]');
        if (hidden) hidden.disabled = false;
      }
    });
  }

  function bindDependents() {
    document.addEventListener('change', function (event) {
      var t = event.target;
      if (!t) return;
      if (t.hasAttribute('data-option') || t.hasAttribute('data-env') || t.hasAttribute('data-disables-option') || t.id) {
        updateOptionDependents();
        updateMutexOptions();
        updateRedisPathVisibility();
      }
    });
    updateOptionDependents();
    updateMutexOptions();
    updateRedisPathVisibility();
  }

  function updateRedisPathVisibility() {
    var wrap = q('#redisPathInputs');
    if (!wrap) return;
    var volumeRadio = q('input[name="redisVolume"][value="volume"]');
    var pathRadio = q('input[name="redisVolume"][value="path"]');
    var show = pathRadio && pathRadio.checked;
    wrap.style.display = show ? '' : 'none';
    if (volumeRadio || pathRadio) {
      // keep
    }
  }

  function getScenariosMap() {
    var s = window.SCENARIOS;
    return s && typeof s === 'object' && !Array.isArray(s) ? s : {};
  }

  function scenarioDisplayText(s, lang) {
    var isZh = lang === 'zh';
    return {
      name: (isZh ? s.nameZh : s.nameEn) || s.name || s.nameZh || s.nameEn || '',
      desc: (isZh ? s.descriptionZh : s.descriptionEn) || s.description || '',
      risk: (isZh ? s.riskNoteZh : s.riskNoteEn) || s.riskNote || ''
    };
  }

  function getScenarioValue() {
    var checked = q('input[name="scenario"]:checked');
    return checked ? (checked.value || '') : '';
  }

  function syncScenarioModeInputs() {
    var wrap = q('#scenario-mode-inputs');
    if (!wrap) return;
    wrap.innerHTML = '';
    var id = getScenarioValue();
    var scenarios = getScenariosMap();
    var modes = (id && scenarios[id] && scenarios[id].modes) ? scenarios[id].modes : DEFAULT_GENERATE_MODES;
    modes.forEach(function (mode) {
      var input = document.createElement('input');
      input.type = 'hidden';
      input.name = 'mode';
      input.value = mode;
      wrap.appendChild(input);
    });
  }

  function applyScenarioPreset(id) {
    var scenarios = getScenariosMap();
    var scene = id ? scenarios[id] : null;
    var descEl = q('#scenario-desc');
    var riskEl = q('#scenario-risk');
    var lang = getLang();
    var t = getI18N(lang);
    try { localStorage.setItem(SCENARIO_STORAGE_KEY, id || ''); } catch (e) {}
    if (!scene) {
      if (descEl) descEl.textContent = t.scenarioPresetDesc || '';
      if (riskEl) { riskEl.style.display = 'none'; riskEl.textContent = ''; }
      syncScenarioModeInputs();
      return;
    }
    // step-1 页面没有配置控件；选项/env 由服务端在 POST 时按 scenario id 写入 session。
    // 其它页面若存在同名控件，则即时回填以便预览。
    Object.keys(scene.options || {}).forEach(function (key) {
      var el = document.getElementById(key) || q('[data-option="' + key + '"]');
      if (!el) return;
      if (el.hasAttribute('data-session-value')) return;
      var val = scene.options[key];
      if (el.type === 'checkbox') el.checked = !!val;
      else el.value = val == null ? '' : String(val);
    });
    Object.keys(scene.envOverrides || {}).forEach(function (env) {
      var el = q('[data-env="' + env + '"]');
      if (!el) return;
      if (el.hasAttribute('data-session-value')) return;
      var val = scene.envOverrides[env];
      if (el.type === 'checkbox') el.checked = val === 'true' || val === true;
      else el.value = val == null ? '' : String(val);
    });
    var text = scenarioDisplayText(scene, lang);
    if (descEl) descEl.textContent = text.desc ? (text.name + ' — ' + text.desc) : text.name;
    if (riskEl) {
      if (text.risk) {
        riskEl.style.display = '';
        riskEl.textContent = (t.scenarioRiskPrefix || '风险提示') + ': ' + text.risk;
      } else {
        riskEl.style.display = 'none';
        riskEl.textContent = '';
      }
    }
    syncScenarioModeInputs();
    updateOptionDependents();
    updateMutexOptions();
  }

  function renderScenarioPresets() {
    var list = q('#scenario-list-options');
    if (!list) return;
    var lang = getLang();
    var scenarios = getScenariosMap();
    var stored = '';
    try { stored = localStorage.getItem(SCENARIO_STORAGE_KEY) || ''; } catch (e) {}
    list.innerHTML = '';
    Object.keys(scenarios).sort().forEach(function (id) {
      var scene = scenarios[id] || {};
      var text = scenarioDisplayText(scene, lang);
      var wrap = document.createElement('div');
      wrap.className = 'position-relative';
      var input = document.createElement('input');
      input.type = 'radio';
      input.className = 'form-check-input position-absolute top-50 end-0 me-3 fs-5';
      input.name = 'scenario';
      input.id = 'scenario-radio-' + id;
      input.value = id;
      if (stored && stored === id) input.checked = true;
      var label = document.createElement('label');
      label.className = 'list-group-item py-3 pe-5';
      label.setAttribute('for', input.id);
      label.innerHTML = '<strong class="fw-semibold">' + escapeHtml(text.name || id) + '</strong>' +
        (text.desc ? '<span class="d-block small opacity-75">' + escapeHtml(text.desc) + '</span>' : '');
      wrap.appendChild(input);
      wrap.appendChild(label);
      list.appendChild(wrap);
    });
    var empty = q('#scenario-radio-empty');
    if (stored && scenarios[stored]) {
      if (empty) empty.checked = false;
      applyScenarioPreset(stored);
    } else {
      if (empty) empty.checked = true;
      applyScenarioPreset('');
    }
  }

  function bindScenarioStep() {
    var group = q('#scenario-list-group');
    if (!group) return;
    renderScenarioPresets();
    group.addEventListener('change', function (event) {
      if (event.target && event.target.name === 'scenario') {
        applyScenarioPreset(event.target.value || '');
      }
    });
    var form = q('#form-step-1');
    if (form) {
      form.addEventListener('submit', function () {
        syncScenarioModeInputs();
        try { localStorage.setItem(SCENARIO_STORAGE_KEY, getScenarioValue() || ''); } catch (e) {}
      });
    }
  }

  function applyStoredScenarioOnStep2() {
    if (!q('#form-step-2')) return;
    var stored = '';
    try { stored = localStorage.getItem(SCENARIO_STORAGE_KEY) || ''; } catch (e) {}
    if (stored) applyScenarioPreset(stored);
  }

  function getRandomBytes(n) {
    var buf = new Uint8Array(n);
    (window.crypto || window.msCrypto).getRandomValues(buf);
    return buf;
  }

  function bytesToHex(bytes) {
    return Array.prototype.map.call(bytes, function (b) {
      return ('0' + b.toString(16)).slice(-2);
    }).join('');
  }

  function bytesToBase64(bytes) {
    var binary = '';
    for (var i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
    return btoa(binary);
  }

  function bytesToBase64Url(bytes) {
    return bytesToBase64(bytes).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
  }

  function generateKeyValue(genType) {
    if (genType === 'hmacKeys') {
      return JSON.stringify({ 'key-1': bytesToHex(getRandomBytes(16)) });
    }
    if (genType === 'aes32') {
      return bytesToBase64(getRandomBytes(32));
    }
    if (genType === 'password') {
      return bytesToBase64Url(getRandomBytes(24));
    }
    return bytesToHex(getRandomBytes(16));
  }

  function showToast(message) {
    var el = q('#suite-toast');
    if (!el) {
      el = document.createElement('div');
      el.id = 'suite-toast';
      el.className = 'toast align-items-center text-bg-dark border-0 position-fixed bottom-0 end-0 m-3';
      el.setAttribute('role', 'status');
      el.innerHTML = '<div class="d-flex"><div class="toast-body"></div><button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast"></button></div>';
      document.body.appendChild(el);
    }
    var body = q('.toast-body', el);
    if (body) body.textContent = message || '';
    if (window.bootstrap && bootstrap.Toast) {
      bootstrap.Toast.getOrCreateInstance(el, { delay: 2000 }).show();
    }
  }

  function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      return navigator.clipboard.writeText(text);
    }
    return new Promise(function (resolve, reject) {
      var ta = document.createElement('textarea');
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      try {
        document.execCommand('copy');
        resolve();
      } catch (e) {
        reject(e);
      } finally {
        document.body.removeChild(ta);
      }
    });
  }

  function collectKeyPayload(grid) {
    var payload = {};
    qa('.keys-value[data-env]', grid).forEach(function (input) {
      if (input.value) payload[input.getAttribute('data-env')] = input.value;
    });
    return payload;
  }

  function saveKeys(grid) {
    var payload = collectKeyPayload(grid);
    if (!Object.keys(payload).length) return Promise.resolve();
    return fetch('/keys/apply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(payload)
    }).then(function (res) {
      if (!res.ok) throw new Error('key save failed');
    });
  }

  function saveKeysAndNavigate(grid, target) {
    var t = getI18N(getLang());
    saveKeys(grid).then(function () {
      window.location.href = target;
    }).catch(function () {
      showToast(t.keysSaveFailed || 'Could not save keys.');
    });
  }

  function bindKeysPage() {
    var grid = q('#keys-grid');
    if (!grid) return;
    var t = getI18N(getLang());

    function fillKey(env, value) {
      var input = q('.keys-value[data-env="' + env + '"]', grid);
      if (input) input.value = value;
    }

    function genOne(row) {
      var env = row.getAttribute('data-env');
      var genType = row.getAttribute('data-gen-type');
      if (!env || !genType) return;
      fillKey(env, generateKeyValue(genType));
    }

    grid.addEventListener('click', function (event) {
      var btn = event.target.closest('button');
      if (!btn) return;
      var row = btn.closest('.keys-row');
      if (!row) return;
      var env = row.getAttribute('data-env');
      if (btn.classList.contains('keys-gen')) {
        genOne(row);
      } else if (btn.classList.contains('keys-copy')) {
        var input = q('.keys-value[data-env="' + env + '"]', row);
        var val = input ? input.value : '';
        copyText(val).then(function () {
          var status = q('#keys-copy-status');
          if (status) status.textContent = t.keysCopyStatusSr || 'Copied';
          showToast(t.keysCopied || t.keyBtnCopy || 'Copied');
        });
      }
    });

    var allBtn = q('#btn-generate-all-keys');
    if (allBtn) {
      allBtn.addEventListener('click', function () {
        qa('.keys-row[data-gen-type]', grid).forEach(genOne);
      });
    }

    var next = q('.step-actions a[href="/review"]');
    if (next) {
      next.addEventListener('click', function (event) {
        event.preventDefault();
        saveKeysAndNavigate(grid, '/review');
      });
    }
  }

  function bindStepNavigation() {
    var form = q('.wizard-form');
    var keysGrid = q('#keys-grid');
    if (!form && !keysGrid) return;
    qa('.step-nav-item[href]').forEach(function (link) {
      link.addEventListener('click', function (event) {
        var target = link.getAttribute('href');
        if (!target) return;
        event.preventDefault();
        if (form) {
          var next = q('input[name="_next"]', form);
          if (!next) {
            next = document.createElement('input');
            next.type = 'hidden';
            next.name = '_next';
            form.appendChild(next);
          }
          next.value = target;
          if (typeof form.requestSubmit === 'function') {
            form.requestSubmit();
          } else {
            if (form.id === 'form-step-1') syncScenarioModeInputs();
            form.submit();
          }
          return;
        }
        saveKeysAndNavigate(keysGrid, target);
      });
    });
  }

  function collectEnvFromForm(root) {
    var env = {};
    qa('[data-env]', root).forEach(function (el) {
      var key = el.getAttribute('data-env');
      if (!key) return;
      if (el.type === 'checkbox') {
        var uncheck = el.getAttribute('data-uncheck-value');
        env[key] = el.checked ? 'true' : (uncheck != null ? uncheck : 'false');
      } else if (el.type !== 'radio' || el.checked) {
        env[key] = el.value;
      }
    });
    return env;
  }

  function collectOptionsFromForm(root) {
    var options = {};
    qa('[data-option]', root).forEach(function (el) {
      var key = el.getAttribute('data-option');
      if (!key) return;
      if (el.type === 'checkbox') options[key] = !!el.checked;
      else if (el.type === 'radio') {
        if (el.checked) options[key] = el.value;
      } else options[key] = el.value;
    });
    return options;
  }

  function downloadBlob(filename, text) {
    var blob = new Blob([text], { type: 'text/plain;charset=utf-8' });
    var url = URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
  }

  function renderDownloads(composes, envText) {
    var box = q('#downloads');
    if (!box) return;
    box.innerHTML = '';
    Object.keys(composes || {}).forEach(function (mode) {
      var btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'btn btn-outline-primary btn-sm me-2 mb-2';
      btn.textContent = 'docker-compose.yml (' + mode + ')';
      btn.addEventListener('click', function () {
        downloadBlob('docker-compose.' + mode + '.yml', composes[mode] || '');
      });
      box.appendChild(btn);
    });
    if (envText != null) {
      var envBtn = document.createElement('button');
      envBtn.type = 'button';
      envBtn.className = 'btn btn-outline-secondary btn-sm me-2 mb-2';
      envBtn.textContent = '.env';
      envBtn.addEventListener('click', function () {
        downloadBlob('.env', envText);
      });
      box.appendChild(envBtn);
    }
  }

  function renderPreview(composes, envText) {
    var wrap = q('#config-preview-wrap');
    if (wrap) { wrap.style.display = ''; wrap.setAttribute('aria-hidden', 'false'); }
    var status = q('#config-preview-status');
    var content = q('#config-preview-content');
    if (!content) return;
    var parts = [];
    Object.keys(composes || {}).forEach(function (mode) {
      parts.push('### ' + mode + '/docker-compose.yml\n' + (composes[mode] || ''));
    });
    if (envText != null) parts.push('### .env\n' + envText);
    content.textContent = parts.join('\n\n');
    if (status) status.textContent = '';
  }

  function bindReviewGenerate() {
    var btn = q('#btn-generate');
    if (!btn) return;
    var t = getI18N(getLang());
    btn.addEventListener('click', function () {
      var status = q('#config-preview-status');
      if (status) status.textContent = t.generating || 'Generating...';
      fetch('/generate', { method: 'POST', credentials: 'same-origin' })
        .then(function (res) {
          if (!res.ok) throw new Error('generate failed');
          return res.json();
        })
        .then(function (data) {
          renderDownloads(data.composes || {}, data.env || '');
          renderPreview(data.composes || {}, data.env || '');
          if (status) status.textContent = t.generateDone || 'Done';
        })
        .catch(function (err) {
          if (status) status.textContent = (t.generateFailed || 'Failed') + ': ' + (err && err.message ? err.message : err);
        });
    });
  }

  function dragContainsFiles(dataTransfer) {
    if (!dataTransfer || !dataTransfer.types) return false;
    for (var i = 0; i < dataTransfer.types.length; i++) {
      if (dataTransfer.types[i] === 'Files') return true;
    }
    return false;
  }

  function textByteLength(value) {
    return new Blob([String(value || '')]).size;
  }

  function readDroppedTextFile(file) {
    if (file && typeof file.text === 'function') return file.text();
    return new Promise(function (resolve, reject) {
      var reader = new FileReader();
      reader.onload = function () { resolve(String(reader.result || '')); };
      reader.onerror = function () { reject(reader.error || new Error('file read failed')); };
      reader.readAsText(file);
    });
  }

  function setImportDropStatus(status, message, isError) {
    if (!status) return;
    status.textContent = message || '';
    status.classList.toggle('text-danger', !!isError);
  }

  function bindImportDrop() {
    var composeInput = q('#input-compose');
    var envInput = q('#input-env');
    if (!composeInput || !envInput) return;
    var t = getI18N(getLang());
    [
      { input: composeInput, other: envInput, status: q('#input-compose-drop-status') },
      { input: envInput, other: composeInput, status: q('#input-env-drop-status') }
    ].forEach(function (zone) {
      zone.input.addEventListener('dragenter', function (event) {
        if (!dragContainsFiles(event.dataTransfer)) return;
        event.preventDefault();
        zone.input.classList.add('is-drag-over');
      });
      zone.input.addEventListener('dragover', function (event) {
        if (!dragContainsFiles(event.dataTransfer)) return;
        event.preventDefault();
        event.dataTransfer.dropEffect = 'copy';
        zone.input.classList.add('is-drag-over');
      });
      zone.input.addEventListener('dragleave', function () {
        zone.input.classList.remove('is-drag-over');
      });
      zone.input.addEventListener('drop', function (event) {
        if (!dragContainsFiles(event.dataTransfer)) return;
        event.preventDefault();
        zone.input.classList.remove('is-drag-over');
        var file = event.dataTransfer.files && event.dataTransfer.files[0];
        if (!file) return;
        if (file.size + textByteLength(zone.other.value) > MAX_IMPORT_TOTAL_BYTES) {
          setImportDropStatus(zone.status, t.importDropTooLarge || 'Imported content is too large.', true);
          return;
        }
        readDroppedTextFile(file).then(function (text) {
          if (textByteLength(text) + textByteLength(zone.other.value) > MAX_IMPORT_TOTAL_BYTES) {
            setImportDropStatus(zone.status, t.importDropTooLarge || 'Imported content is too large.', true);
            return;
          }
          zone.input.value = text;
          setImportDropStatus(zone.status, formatMessage(t.importDropLoaded || 'Loaded {name}', { name: file.name }), false);
        }).catch(function () {
          setImportDropStatus(zone.status, formatMessage(t.importDropReadFailed || 'Could not read {name}.', { name: file.name }), true);
        });
      });
    });
  }

  function bindImportParse() {
    var btn = q('#btn-parse');
    if (!btn) return;
    var t = getI18N(getLang());
    btn.addEventListener('click', function () {
      var compose = (q('#input-compose') || {}).value || '';
      var env = (q('#input-env') || {}).value || '';
      var out = q('#parse-result');
      if (!String(compose).trim()) {
        if (out) out.innerHTML = '<div class="text-danger">' + escapeHtml(t.importComposeRequired || 'compose is required') + '</div>';
        return;
      }
      btn.disabled = true;
      fetch('/import/apply', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'same-origin',
        body: JSON.stringify({ compose: compose, env: env })
      }).then(function (res) {
        return res.json().then(function (data) {
          if (!res.ok || !data.ok) {
            var msg = (data.errors && data.errors[0]) || data.error || 'parse failed';
            throw new Error(msg);
          }
          return data;
        });
      }).then(function (data) {
        if (data.suggestedScene) {
          try { localStorage.setItem(SCENARIO_STORAGE_KEY, data.suggestedScene); } catch (e) {}
        }
        window.location.href = data.redirect || '/wizard/step-2';
      }).catch(function (err) {
        btn.disabled = false;
        if (out) out.innerHTML = '<div class="text-danger">' + escapeHtml(err && err.message ? err.message : String(err)) + '</div>';
      });
    });
  }

  function init() {
    bindLangSwitch();
    bindDependents();
    bindScenarioStep();
    applyStoredScenarioOnStep2();
    bindKeysPage();
    bindStepNavigation();
    bindReviewGenerate();
    bindImportDrop();
    bindImportParse();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
