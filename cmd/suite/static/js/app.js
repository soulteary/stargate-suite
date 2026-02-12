(function () {
  'use strict';

  var LANG_STORAGE_KEY = 'stargate-suite-lang';
  var scenarioExtraOptions = {};
  var DEFAULT_GENERATE_MODES = ['traefik'];
  var KEY_DEFINITIONS = [
    { env: 'WARDEN_API_KEY', labelKey: 'keyLabelWardenApiKey', descKey: 'keyDescWardenApiKey', genType: 'apiKey' },
    { env: 'WARDEN_OTP_SECRET_KEY', labelKey: 'keyLabelWardenOtpSecretKey', descKey: 'keyDescWardenOtpSecretKey', genType: 'apiKey' },
    { env: 'HERALD_API_KEY', labelKey: 'keyLabelHeraldApiKey', descKey: 'keyDescHeraldApiKey', genType: 'apiKey' },
    { env: 'HERALD_HMAC_SECRET', labelKey: 'keyLabelHeraldHmacSecret', descKey: 'keyDescHeraldHmacSecret', genType: 'hmacSecret' },
    { env: 'HERALD_HMAC_KEYS', labelKey: 'keyLabelHeraldHmacKeys', descKey: 'keyDescHeraldHmacKeys', genType: 'hmacKeys' },
    { env: 'HERALD_TOTP_API_KEY', labelKey: 'keyLabelHeraldTotpApiKey', descKey: 'keyDescHeraldTotpApiKey', genType: 'apiKey' },
    { env: 'HERALD_TOTP_HMAC_SECRET', labelKey: 'keyLabelHeraldTotpHmacSecret', descKey: 'keyDescHeraldTotpHmacSecret', genType: 'hmacSecret' },
    { env: 'HERALD_TOTP_ENCRYPTION_KEY', labelKey: 'keyLabelHeraldTotpEncryptionKey', descKey: 'keyDescHeraldTotpEncryptionKey', genType: 'aes32' },
    { env: 'WARDEN_REDIS_PASSWORD', labelKey: 'keyLabelWardenRedisPassword', descKey: 'keyDescWardenRedisPassword', genType: 'password' },
    { env: 'HERALD_REDIS_PASSWORD', labelKey: 'keyLabelHeraldRedisPassword', descKey: 'keyDescHeraldRedisPassword', genType: 'password' },
    { env: 'SESSION_STORAGE_REDIS_PASSWORD', labelKey: 'keyLabelSessionRedisPassword', descKey: 'keyDescSessionRedisPassword', genType: 'password' },
    { env: 'HERALD_DINGTALK_API_KEY', labelKey: 'keyLabelHeraldDingtalkApiKey', descKey: 'keyDescHeraldDingtalkApiKey', genType: 'apiKey' },
    { env: 'HERALD_SMTP_API_KEY', labelKey: 'keyLabelHeraldSmtpApiKey', descKey: 'keyDescHeraldSmtpApiKey', genType: 'apiKey' }
  ];

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

  function getI18N(lang) {
    var dict = window.I18N || {};
    return dict[lang] || dict.zh || {};
  }

  function getLang() {
    var stored = localStorage.getItem(LANG_STORAGE_KEY);
    return stored || 'zh';
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
      var scope = el.closest('.panel-details') || document;
      var input = q('[data-env="' + key + '"]', scope);
      var isOn = false;
      if (input) {
        if (input.type === 'checkbox') isOn = input.checked;
        else isOn = input.value === 'true' || input.value === '1';
      }
      el.style.display = isOn ? '' : 'none';
    });
  }

  function bindDependents() {
    qa('[data-option], [data-env]').forEach(function (el) {
      if (!el._boundChange) {
        el._boundChange = true;
        el.addEventListener('change', updateOptionDependents);
      }
    });
    var redisStoragePath = document.getElementById('redisStoragePath');
    var redisPathInputs = document.getElementById('redisPathInputs');
    if (redisStoragePath && redisPathInputs) {
      function syncRedisPathDisplay() {
        redisPathInputs.style.display = redisStoragePath.checked ? 'block' : 'none';
      }
      qa('input[name="redisStorage"]').forEach(function (el) {
        el.addEventListener('change', syncRedisPathDisplay);
      });
      syncRedisPathDisplay();
    }
    updateOptionDependents();
    var form = document.getElementById('form');
    if (form && typeof refreshEnhancedUI === 'function') {
      form.addEventListener('change', refreshEnhancedUI);
      form.addEventListener('input', refreshEnhancedUI);
    }
  }

  function scenarioDisplayText(s, lang) {
    var isZh = lang === 'zh';
    return {
      name: (isZh ? s.nameZh : s.nameEn) || s.name || '',
      desc: (isZh ? s.descriptionZh : s.descriptionEn) || s.description || '',
      risk: (isZh ? s.riskNoteZh : s.riskNoteEn) || s.riskNote || ''
    };
  }

  function getScenarioById(id) {
    if (!id) return null;
    var scenarios = window.SCENARIOS || {};
    return scenarios[id] || null;
  }

  function resolveModesByScenarioId(id) {
    var scenario = getScenarioById(id);
    var modes = scenario && Array.isArray(scenario.modes) ? scenario.modes.filter(function (m) {
      return !!m;
    }) : [];
    return modes.length ? modes : DEFAULT_GENERATE_MODES.slice();
  }

  function getSelectedModes() {
    var select = document.getElementById('scenario-select');
    var id = select ? (select.value || '') : '';
    return resolveModesByScenarioId(id);
  }

  function renderScenarioPresets() {
    var select = document.getElementById('scenario-select');
    if (!select) return;
    var scenarios = window.SCENARIOS || {};
    var lang = getLang();
    var t = getI18N(lang);
    var current = select.value || '';

    while (select.options.length > 1) select.remove(1);
    Object.keys(scenarios).sort().forEach(function (id) {
      var s = scenarios[id] || {};
      var text = scenarioDisplayText(s, lang);
      var option = document.createElement('option');
      option.value = id;
      option.textContent = text.desc ? (text.name + ' - ' + text.desc) : text.name;
      select.appendChild(option);
    });
    var placeholder = q('option[value=""]', select);
    if (placeholder && t.scenarioPresetPlaceholder) placeholder.textContent = t.scenarioPresetPlaceholder;
    if (current && scenarios[current]) select.value = current;
  }

  function setEnvValueByKey(key, value) {
    qa('[data-env="' + key + '"]').forEach(function (el) {
      if (el.type === 'checkbox') {
        var normalized = String(value).toLowerCase();
        el.checked = normalized === 'true' || normalized === '1';
      } else {
        el.value = String(value);
      }
    });
  }

  function applyScenarioPreset(id) {
    var scenarios = window.SCENARIOS || {};
    var scenario = scenarios[id];
    var descEl = document.getElementById('scenario-desc');
    var riskEl = document.getElementById('scenario-risk');
    var lang = getLang();
    var t = getI18N(lang);

    if (!scenario) {
      scenarioExtraOptions = {};
      if (descEl) descEl.textContent = t.scenarioPresetDesc || '';
      if (riskEl) {
        riskEl.style.display = 'none';
        riskEl.textContent = '';
      }
      return;
    }

    scenarioExtraOptions = {};
    var options = scenario.options || {};
    Object.keys(options).forEach(function (key) {
      var value = options[key];
      var input = q('[data-option="' + key + '"]');
      if (input) {
        if (input.type === 'checkbox') input.checked = !!value;
        else input.value = String(value);
      } else {
        scenarioExtraOptions[key] = value;
      }
    });

    var envs = scenario.envOverrides || {};
    Object.keys(envs).forEach(function (key) {
      setEnvValueByKey(key, envs[key]);
    });

    if (descEl || riskEl) {
      var text = scenarioDisplayText(scenario, lang);
      if (descEl) descEl.textContent = text.desc ? (text.name + ' - ' + text.desc) : text.name;
      if (riskEl) {
        if (text.risk) {
          riskEl.style.display = '';
          riskEl.textContent = (t.scenarioRiskPrefix || '风险提示') + ': ' + text.risk;
        } else {
          riskEl.style.display = 'none';
          riskEl.textContent = '';
        }
      }
    }
    updateOptionDependents();
    if (typeof refreshEnhancedUI === 'function') refreshEnhancedUI();
  }

  function bindScenarioPreset() {
    var select = document.getElementById('scenario-select');
    if (!select) return;
    renderScenarioPresets();
    select.addEventListener('change', function () {
      applyScenarioPreset(this.value || '');
    });
  }

  var REQUIRED_STARGATE_ENV = ['AUTH_HOST', 'WARDEN_URL', 'HERALD_URL'];

  function scrollToStep(stepNum) {
    var idMap = { 1: 'step-compose-type', 2: 'step-general', 3: 'step-stargate', 4: 'step-warden', 5: 'step-herald' };
    var id = idMap[stepNum];
    var el = id ? document.getElementById(id) : null;
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
    qa('.step-nav-item').forEach(function (btn) {
      var num = parseInt(btn.getAttribute('data-step'), 10);
      btn.classList.toggle('active', num === stepNum);
    });
  }

  function updateStepHints() {
    var t = getI18N(getLang());
    var modes = getSelectedModes();
    var payload = collectGeneratePayload();
    var envOverrides = (payload && payload.options && payload.options.envOverrides) ? payload.options.envOverrides : {};

    for (var step = 1; step <= 5; step++) {
      var hintEl = document.getElementById('step-hint-' + step);
      if (!hintEl) continue;
      hintEl.textContent = '';
      hintEl.className = 'step-hint';
      if (step === 1) {
        if (modes.length === 0) {
          hintEl.textContent = t.stepHintNeedMode || '(请至少选择一种类型)';
          hintEl.classList.add('step-missing');
        } else {
          hintEl.textContent = modes.join(', ');
          hintEl.classList.add('step-ok');
        }
      } else if (step === 3 && (modes.indexOf('traefik') !== -1 || modes.indexOf('traefik-stargate') !== -1)) {
        var missing = REQUIRED_STARGATE_ENV.filter(function (k) {
          var v = envOverrides[k];
          return v === undefined || v === null || !String(v).trim();
        });
        if (missing.length) {
          hintEl.textContent = (t.stepHintMissingEnv || '缺少: ') + missing.join(', ');
          hintEl.classList.add('step-missing');
        } else {
          hintEl.textContent = t.stepHintOk || '✓';
          hintEl.classList.add('step-ok');
        }
      } else if (step === 2 || step === 4 || step === 5) {
        hintEl.textContent = '';
      }
    }
  }

  function updateSummaryBar() {
    var bar = document.getElementById('summary-bar');
    if (!bar) return;
    var t = getI18N(getLang());
    var modes = getSelectedModes();
    var payload = collectGeneratePayload();
    var envOverrides = (payload && payload.options && payload.options.envOverrides) ? payload.options.envOverrides : {};
    var scenarioId = getScenarioValueFromPage();
    var scenario = (window.SCENARIOS || {})[scenarioId];
    var isZh = getLang() === 'zh';
    var riskNote = scenario ? ((isZh ? scenario.riskNoteZh : scenario.riskNoteEn) || scenario.riskNote || '') : '';

    var parts = [];
    if (modes.length) parts.push((t.summaryModes || '已选') + ': ' + modes.join(', '));
    var hasStargate = modes.indexOf('traefik') !== -1 || modes.indexOf('traefik-stargate') !== -1;
    var missing = hasStargate ? REQUIRED_STARGATE_ENV.filter(function (k) {
      var v = envOverrides[k];
      return v === undefined || v === null || !String(v).trim();
    }) : [];
    if (missing.length) parts.push((t.validationMissingEnv || '缺少') + ': ' + missing.join(', '));
    if (riskNote) parts.push((t.scenarioRiskPrefix || '风险') + ': ' + riskNote);

    bar.textContent = parts.join(' · ');
    bar.className = 'summary-bar';
    if (missing.length) bar.classList.add('summary-error');
    else if (riskNote) bar.classList.add('summary-warn');
    else if (parts.length) bar.classList.add('summary-ok');
  }

  function syncServiceDetailsOpen() {
    var modes = getSelectedModes();
    var hasTraefik = modes.indexOf('traefik') !== -1 || modes.indexOf('traefik-stargate') !== -1;
    var hasWarden = modes.indexOf('traefik-warden') !== -1 || modes.indexOf('traefik') !== -1;
    var hasHerald = modes.indexOf('traefik-herald') !== -1 || modes.indexOf('traefik') !== -1;

    var stargateEl = document.getElementById('details-stargate');
    if (stargateEl) stargateEl.open = hasTraefik;

    var wardenEl = document.getElementById('details-warden');
    if (wardenEl) wardenEl.open = hasWarden;

    qa('.service-details[id^="details-herald"]').forEach(function (details) {
      details.open = hasHerald;
    });
  }

  function bindStepNav() {
    var nav = document.getElementById('step-nav');
    if (!nav) return;
    nav.addEventListener('click', function (e) {
      var btn = e.target && e.target.closest && e.target.closest('.step-nav-item');
      if (!btn) return;
      var step = parseInt(btn.getAttribute('data-step'), 10);
      if (step) scrollToStep(step);
    });
    syncServiceDetailsOpen();
    updateStepHints();
    updateSummaryBar();
  }

  function refreshEnhancedUI() {
    updateOptionDependents();
    syncServiceDetailsOpen();
    updateStepHints();
    updateSummaryBar();
  }

  function collectGeneratePayload() {
    var modes = getSelectedModes();
    var options = { envOverrides: {} };

    qa('[data-option]').forEach(function (el) {
      var key = el.getAttribute('data-option');
      if (!key) return;
      if (el.type === 'checkbox') {
        options[key] = el.checked;
      } else if (el.tagName === 'SELECT') {
        if (el.value === 'true' || el.value === 'false') options[key] = el.value === 'true';
        else options[key] = (el.value || '').trim();
      } else {
        options[key] = (el.value || '').trim();
      }
    });

    Object.keys(scenarioExtraOptions).forEach(function (key) {
      options[key] = scenarioExtraOptions[key];
    });

    var redisVolume = document.getElementById('redisStorageVolume');
    options.useNamedVolume = redisVolume ? redisVolume.checked : true;
    if (options.useNamedVolume) {
      options.heraldRedisDataPath = '';
      options.wardenRedisDataPath = '';
    } else {
      options.heraldRedisDataPath = options.heraldRedisDataPath || './data/herald-redis';
      options.wardenRedisDataPath = options.wardenRedisDataPath || './data/warden-redis';
    }
    options.traefikNetworkName = options.traefikNetworkName || 'traefik';

    qa('input[name="envBool"][data-env]').forEach(function (el) {
      options.envOverrides[el.getAttribute('data-env')] = el.checked ? 'true' : 'false';
    });

    qa('[data-env]').forEach(function (el) {
      if (el.name === 'envBool') return;
      var key = el.getAttribute('data-env');
      var value = (el.value || '').trim();
      if (key && value) options.envOverrides[key] = value;
    });

    return { modes: modes, options: options };
  }

  function validateGeneratePayload(payload) {
    var t = getI18N(getLang());
    if (payload.modes.indexOf('traefik') !== -1 || payload.modes.indexOf('traefik-stargate') !== -1) {
      var required = ['AUTH_HOST', 'WARDEN_URL', 'HERALD_URL'];
      var missing = required.filter(function (key) {
        var value = payload.options.envOverrides[key];
        return !value || !String(value).trim();
      });
      if (missing.length) {
        return { ok: false, message: (t.validationMissingEnv || '缺少必要环境变量: ') + missing.join(', ') };
      }
    }
    return { ok: true };
  }

  function showResult(message, isError) {
    var resultEl = document.getElementById('result');
    if (!resultEl) return;
    resultEl.textContent = message || '';
    resultEl.className = isError ? 'result error' : 'result';
  }

  function renderDownloads(data) {
    var downloadsEl = document.getElementById('downloads');
    if (!downloadsEl) return;
    downloadsEl.innerHTML = '';
    Object.keys(data.composes || {}).forEach(function (mode) {
      var blob = new Blob([data.composes[mode]], { type: 'application/x-yaml;charset=utf-8' });
      var url = URL.createObjectURL(blob);
      var link = document.createElement('a');
      link.href = url;
      link.download = mode + '/docker-compose.yml';
      link.textContent = mode + '/docker-compose.yml';
      downloadsEl.appendChild(link);
    });
    var envBlob = new Blob([data.env || ''], { type: 'text/plain;charset=utf-8' });
    var envUrl = URL.createObjectURL(envBlob);
    var envLink = document.createElement('a');
    envLink.href = envUrl;
    envLink.download = '.env';
    envLink.textContent = '.env';
    downloadsEl.appendChild(envLink);
  }

  function renderPreview(data) {
    var wrap = document.getElementById('config-preview-wrap');
    var content = document.getElementById('config-preview-content');
    if (!wrap || !content) return;
    var t = getI18N(getLang());
    var selectText = t.previewSelectAll || '全选';
    var copyText = t.previewCopy || '复制';
    var composeLabel = t.previewComposeLabel || 'docker-compose.yml';
    var envLabel = t.previewEnvLabel || '.env';
    var html = '';

    Object.keys(data.composes || {}).forEach(function (mode) {
      html += '<div class="preview-block">' +
        '<p class="preview-title"><strong>' + escapeHtml(mode + '/' + composeLabel) + '</strong></p>' +
        '<div class="preview-actions">' +
        '<button type="button" class="preview-select">' + escapeHtml(selectText) + '</button>' +
        '<button type="button" class="preview-copy">' + escapeHtml(copyText) + '</button>' +
        '</div>' +
        '<pre>' + escapeHtml(data.composes[mode]) + '</pre>' +
        '</div>';
    });

    html += '<div class="preview-block">' +
      '<p class="preview-title"><strong>' + escapeHtml(envLabel) + '</strong></p>' +
      '<div class="preview-actions">' +
      '<button type="button" class="preview-select">' + escapeHtml(selectText) + '</button>' +
      '<button type="button" class="preview-copy">' + escapeHtml(copyText) + '</button>' +
      '</div>' +
      '<pre>' + escapeHtml(data.env || '') + '</pre>' +
      '</div>';

    content.innerHTML = html;
    wrap.style.display = '';
    wrap.setAttribute('aria-hidden', 'false');
  }

  function bindPreviewActions() {
    var content = document.getElementById('config-preview-content');
    var status = document.getElementById('config-preview-status');
    if (!content) return;
    content.addEventListener('click', function (event) {
      var target = event.target;
      if (!target || (!target.classList.contains('preview-select') && !target.classList.contains('preview-copy'))) return;
      var pre = target.closest('.preview-block') && target.closest('.preview-block').querySelector('pre');
      if (!pre) return;

      var selection = window.getSelection();
      var range = document.createRange();
      range.selectNodeContents(pre);
      selection.removeAllRanges();
      selection.addRange(range);

      if (target.classList.contains('preview-select')) {
        if (status) status.textContent = getI18N(getLang()).previewSelectedHint || '已选中，请复制。';
        return;
      }

      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(pre.textContent).then(function () {
          if (status) status.textContent = getI18N(getLang()).keyCopied || '已复制';
          selection.removeAllRanges();
        }).catch(function () {
          try { document.execCommand('copy'); } catch (err) {}
          if (status) status.textContent = getI18N(getLang()).keyCopyFailed || '复制失败';
          selection.removeAllRanges();
        });
      } else {
        try { document.execCommand('copy'); } catch (err) {}
        selection.removeAllRanges();
      }
    });
  }

  function runGenerate() {
    var submit = document.getElementById('btn-generate');
    var t = getI18N(getLang());
    var payload = collectGeneratePayload();
    var check = validateGeneratePayload(payload);
    if (!check.ok) {
      showResult(check.message, true);
      return;
    }

    showResult(t.generating || '生成中...', false);
    if (submit) submit.disabled = true;

    fetch('/api/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    }).then(function (resp) {
      if (!resp.ok) throw new Error(resp.statusText);
      return resp.json();
    }).then(function (data) {
      showResult(t.resultSuccess || '生成成功', false);
      renderDownloads(data);
      renderPreview(data);
    }).catch(function (err) {
      showResult((t.requestFailed || '请求失败: ') + (err && err.message ? err.message : String(err)), true);
    }).finally(function () {
      if (submit) submit.disabled = false;
    });
  }

  function bindGenerate() {
    var form = document.getElementById('form');
    if (!form) return;
    form.addEventListener('submit', function (event) {
      event.preventDefault();
      runGenerate();
    });
  }

  function parseResultHtml(data, t) {
    var html = '';
    if (Array.isArray(data.services) && data.services.length) {
      html += '<p><strong>' + escapeHtml(t.parseServicesLabel || '服务') + '</strong></p><ul>';
      data.services.forEach(function (item) {
        html += '<li>' + escapeHtml(item) + '</li>';
      });
      html += '</ul>';
    }
    if (data.envVars && Object.keys(data.envVars).length) {
      html += '<p><strong>' + escapeHtml(t.parseEnvVarsLabel || '环境变量') + '</strong></p>';
      html += '<pre>';
      Object.keys(data.envVars).sort().forEach(function (key) {
        html += escapeHtml(key + '=' + String(data.envVars[key] || '')) + '\n';
      });
      html += '</pre>';
      html += '<div class="actions"><button type="button" id="btn-load-into-generate">' + escapeHtml(t.loadIntoGenerate || '加载到生成配置') + '</button></div>';
    }
    return html || escapeHtml(t.parseSuccess || '解析成功');
  }

  function bindImportParse() {
    var btn = document.getElementById('btn-parse');
    var result = document.getElementById('parse-result');
    var composeEl = document.getElementById('input-compose');
    var envEl = document.getElementById('input-env');
    if (!btn || !result || !composeEl) return;

    btn.addEventListener('click', function () {
      var t = getI18N(getLang());
      var compose = (composeEl.value || '').trim();
      var env = envEl ? (envEl.value || '').trim() : '';
      if (!compose) {
        result.textContent = t.importComposeRequired || '请粘贴 docker-compose 内容。';
        result.className = 'result error';
        return;
      }

      result.className = 'result';
      result.textContent = t.parsing || '解析中...';
      btn.disabled = true;

      fetch('/api/parse', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ compose: compose, env: env })
      }).then(function (resp) {
        return resp.json().then(function (data) {
          return { ok: resp.ok, data: data };
        });
      }).then(function (res) {
        if (!res.ok || (res.data.errors && res.data.errors.length)) {
          result.textContent = (res.data.errors || [t.parseError || '解析失败']).join('\n');
          result.className = 'result error';
          return;
        }
        result.innerHTML = parseResultHtml(res.data, t);
        var loadBtn = document.getElementById('btn-load-into-generate');
        if (!loadBtn) return;
        loadBtn.addEventListener('click', function () {
          loadBtn.disabled = true;
          fetch('/import/apply', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ compose: compose, env: env })
          }).then(function (resp) {
            if (!resp.ok) throw new Error('apply failed');
            window.location.href = '/wizard/step-1';
          }).catch(function () {
            result.textContent = t.applyFailed || '加载失败';
            result.className = 'result error';
          }).finally(function () {
            loadBtn.disabled = false;
          });
        });
      }).catch(function (err) {
        result.textContent = (t.requestFailed || '请求失败: ') + (err && err.message ? err.message : String(err));
        result.className = 'result error';
      }).finally(function () {
        btn.disabled = false;
      });
    });
  }

  function getRandomBytes(length) {
    var bytes = new Uint8Array(length);
    if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
      crypto.getRandomValues(bytes);
    } else {
      for (var i = 0; i < length; i++) bytes[i] = Math.floor(Math.random() * 256);
    }
    return bytes;
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

  function generateKeyValue(type) {
    if (type === 'aes32') return bytesToBase64(getRandomBytes(32));
    if (type === 'password') return bytesToBase64(getRandomBytes(24)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    if (type === 'hmacKeys') {
      var keyId = 'stargate-' + bytesToHex(getRandomBytes(4));
      var secret = bytesToHex(getRandomBytes(32));
      var obj = {};
      obj[keyId] = secret;
      return JSON.stringify(obj);
    }
    return bytesToHex(getRandomBytes(32));
  }

  function copyText(text, successMsg, failedMsg, statusEl) {
    function setStatus(msg) {
      if (!statusEl) return;
      statusEl.textContent = msg;
      setTimeout(function () { statusEl.textContent = ''; }, 900);
    }
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function () {
        setStatus(successMsg);
      }).catch(function () {
        setStatus(failedMsg);
      });
      return;
    }
    setStatus(failedMsg);
  }

  function renderKeys() {
    var grid = document.getElementById('keys-grid');
    var statusEl = document.getElementById('keys-copy-status');
    if (!grid) return;
    var t = getI18N(getLang());
    grid.innerHTML = '';

    KEY_DEFINITIONS.forEach(function (def) {
      var row = document.createElement('div');
      row.className = 'keys-row';
      row.innerHTML = '' +
        '<p class="keys-label"><code>' + escapeHtml(def.env) + '</code> - ' + escapeHtml(t[def.labelKey] || def.env) + '</p>' +
        '<p class="help">' + escapeHtml(t[def.descKey] || '') + '</p>' +
        '<div class="actions">' +
        '<button type="button" class="key-gen" data-env="' + escapeHtml(def.env) + '">' + escapeHtml(t.keyBtnGenerate || '生成') + '</button>' +
        '<button type="button" class="key-copy" data-env="' + escapeHtml(def.env) + '">' + escapeHtml(t.keyBtnCopy || '复制') + '</button>' +
        '</div>' +
        '<input class="keys-value" data-env="' + escapeHtml(def.env) + '" readonly>';
      grid.appendChild(row);
    });

    grid.addEventListener('click', function (event) {
      var target = event.target;
      if (!target) return;
      var env = target.getAttribute('data-env');
      if (!env) return;
      var input = q('input.keys-value[data-env="' + env + '"]', grid);
      var def = KEY_DEFINITIONS.filter(function (item) { return item.env === env; })[0];
      if (!input || !def) return;
      if (target.classList.contains('key-gen')) {
        input.value = generateKeyValue(def.genType);
      } else if (target.classList.contains('key-copy')) {
        if (!input.value) return;
        copyText(input.value, t.keyCopied || '已复制', t.keyCopyFailed || '复制失败', statusEl);
      }
    });
  }

  function bindKeyActions() {
    var btnAll = document.getElementById('btn-generate-all-keys');
    var btnFill = document.getElementById('btn-fill-keys-into-generate');
    var grid = document.getElementById('keys-grid');
    if (btnAll && grid) {
      btnAll.addEventListener('click', function () {
        KEY_DEFINITIONS.forEach(function (def) {
          var input = q('input.keys-value[data-env="' + def.env + '"]', grid);
          if (input) input.value = generateKeyValue(def.genType);
        });
      });
    }
    if (btnFill && grid) {
      btnFill.addEventListener('click', function () {
        var payload = {};
        KEY_DEFINITIONS.forEach(function (def) {
          var input = q('input.keys-value[data-env="' + def.env + '"]', grid);
          if (input && input.value) payload[def.env] = input.value;
        });
        if (!Object.keys(payload).length) return;
        fetch('/keys/apply', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        }).then(function () {
          window.location.href = '/wizard/step-2';
        });
      });
    }
  }

  bindLangSwitch();
  bindStepNav();
  bindDependents();
  bindScenarioPreset();
  bindGenerate();
  bindPreviewActions();
  bindImportParse();
  renderKeys();
  bindKeyActions();
  refreshEnhancedUI();
})();

// Scenario-driven UX tuning for multi-page wizard:
// - persist selected scenario across steps
// - reorder config groups by scenario/module
// - prefill relevant options/envs on current page
(function () {
  'use strict';

  var SCENARIO_STORAGE_KEY = 'stargate-suite-selected-scenario';

  function q(selector, root) {
    return (root || document).querySelector(selector);
  }

  function qa(selector, root) {
    return Array.prototype.slice.call((root || document).querySelectorAll(selector));
  }

  function getLang() {
    return localStorage.getItem('stargate-suite-lang') || 'zh';
  }

  function getScenarioPreset(id) {
    if (!id) return null;
    var all = window.SCENARIOS || {};
    return all[id] || null;
  }

  function getScenarioValueFromPage() {
    var listGroup = document.getElementById('scenario-list-group');
    if (listGroup) {
      var radio = listGroup.querySelector('input[name="scenario"]:checked');
      return radio ? (radio.value || '') : '';
    }
    var select = q('#scenario-select');
    return select ? (select.value || '') : '';
  }
  function setScenarioValueOnPage(id) {
    var listGroup = document.getElementById('scenario-list-group');
    if (listGroup) {
      var radio = listGroup.querySelector('input[name="scenario"][value="' + (id || '') + '"]');
      if (radio) { radio.checked = true; radio.dispatchEvent(new Event('change', { bubbles: true })); return; }
    }
    var select = q('#scenario-select');
    if (select) { select.value = id || ''; select.dispatchEvent(new Event('change', { bubbles: true })); }
  }
  function readActiveScenarioId() {
    var fromPage = getScenarioValueFromPage();
    if (fromPage) return fromPage;
    return localStorage.getItem(SCENARIO_STORAGE_KEY) || '';
  }

  function persistScenario(id) {
    if (id) localStorage.setItem(SCENARIO_STORAGE_KEY, id);
    else localStorage.removeItem(SCENARIO_STORAGE_KEY);
  }

  function setInputValue(el, val) {
    if (!el) return;
    if (el.type === 'checkbox') {
      var s = String(val).toLowerCase();
      el.checked = s === 'true' || s === '1' || s === 'yes' || s === 'on';
    } else {
      el.value = String(val);
    }
    el.dispatchEvent(new Event('change', { bubbles: true }));
  }

  function applyPresetToCurrentPage(preset) {
    if (!preset) return;
    var opts = preset.options || {};
    Object.keys(opts).forEach(function (key) {
      var el = q('[data-option="' + key + '"]');
      if (el) setInputValue(el, opts[key]);
    });
    var envs = preset.envOverrides || {};
    Object.keys(envs).forEach(function (key) {
      qa('[data-env="' + key + '"]').forEach(function (el) {
        setInputValue(el, envs[key]);
      });
    });
  }

  function reorderElements(container, selector, rankFn) {
    if (!container) return;
    var items = qa(selector, container);
    if (items.length < 2) return;
    items.sort(function (a, b) {
      return rankFn(a) - rankFn(b);
    }).forEach(function (el) {
      container.appendChild(el);
    });
  }

  function reorderStep2Sections(scenarioId) {
    var wrap = q('.config-options-cols');
    if (!wrap) return;
    var orders = {
      's1-solo-gate': [
        'healthCheckSection', 'traefikNetworkSection', 'containerPrefixSection',
        'exposePortsSection', 'redisStorage', 'optionalChannelsSection', 'imageVersionsSection'
      ],
      's2-solo-gate-session-redis': [
        'healthCheckSection', 'traefikNetworkSection', 'redisStorage',
        'containerPrefixSection', 'exposePortsSection', 'optionalChannelsSection', 'imageVersionsSection'
      ],
      's3-gate-warden': [
        'healthCheckSection', 'traefikNetworkSection', 'redisStorage',
        'containerPrefixSection', 'exposePortsSection', 'optionalChannelsSection', 'imageVersionsSection'
      ],
      's4-gate-warden-herald': [
        'healthCheckSection', 'traefikNetworkSection', 'redisStorage',
        'containerPrefixSection', 'optionalChannelsSection', 'exposePortsSection', 'imageVersionsSection'
      ],
      's5-gate-warden-herald-plugins': [
        'optionalChannelsSection', 'healthCheckSection', 'traefikNetworkSection',
        'redisStorage', 'containerPrefixSection', 'exposePortsSection', 'imageVersionsSection'
      ]
    };
    var order = orders[scenarioId];
    if (!order) return;
    var rank = {};
    order.forEach(function (key, idx) { rank[key] = idx; });
    reorderElements(wrap, '.env-group[data-section-key]', function (el) {
      var key = el.getAttribute('data-section-key') || '';
      return rank[key] !== undefined ? rank[key] : 999;
    });
    var noPlugins = scenarioId === 's1-solo-gate' || scenarioId === 's2-solo-gate-session-redis';
    qa('.env-group[data-section-key="optionalChannelsSection"]', wrap).forEach(function (el) {
      el.style.display = noPlugins ? 'none' : '';
    });
  }

  function toggleScenarioStepNote(noteId, text) {
    var note = q('#' + noteId);
    if (!note) return;
    if (!text) {
      note.style.display = 'none';
      note.textContent = '';
      return;
    }
    note.textContent = text;
    note.style.display = '';
  }

  function tuneWardenStep(scenarioId) {
    var block = q('.config-step-block[data-step="4"]');
    var formWrap = block && block.querySelector('.field-group');
    var isLite = scenarioId === 's1-solo-gate' || scenarioId === 's2-solo-gate-session-redis';
    var tip = isLite
      ? (getLang() === 'zh' ? '当前场景不启用 Warden，可直接下一步。' : 'Warden is disabled in this scenario; you can continue.')
      : '';
    toggleScenarioStepNote('scenario-step4-note', tip);
    if (formWrap) formWrap.style.display = isLite ? 'none' : '';
  }

  function tuneStargateStep(scenarioId) {
    var block = q('.config-step-block[data-step="3"]');
    if (!block) return;
    var wardenEnvs = ['WARDEN_URL', 'WARDEN_ENABLED', 'WARDEN_CACHE_TTL', 'WARDEN_OTP_ENABLED'];
    var heraldEnvs = ['HERALD_URL', 'HERALD_ENABLED', 'LOGIN_SMS_ENABLED', 'LOGIN_EMAIL_ENABLED'];
    var noWarden = scenarioId === 's1-solo-gate' || scenarioId === 's2-solo-gate-session-redis';
    var noHerald = noWarden || scenarioId === 's3-gate-warden';
    qa('.env-group[data-section-key="heraldTlsSection"]', block).forEach(function (el) {
      el.style.display = noHerald ? 'none' : '';
    });
    qa('.config-item[data-env-name]', block).forEach(function (el) {
      var env = el.getAttribute('data-env-name') || '';
      var hide = (noWarden && wardenEnvs.indexOf(env) !== -1) || (noHerald && heraldEnvs.indexOf(env) !== -1);
      el.style.display = hide ? 'none' : '';
    });
  }

  function tuneHeraldStep(scenarioId) {
    var block = q('.config-step-block[data-step="5"]');
    var formWrap = block && block.querySelector('.field-group.config-options-cols');
    var providersWrap = q('.providers-fieldset');
    var noHerald = scenarioId === 's1-solo-gate' || scenarioId === 's2-solo-gate-session-redis' || scenarioId === 's3-gate-warden';
    var hideProviders = noHerald || scenarioId === 's4-gate-warden-herald';
    var tip = noHerald
      ? (getLang() === 'zh' ? '当前场景不启用 Herald，可直接下一步。' : 'Herald is disabled in this scenario; you can continue.')
      : '';
    toggleScenarioStepNote('scenario-step5-note', tip);
    if (formWrap) formWrap.style.display = noHerald ? 'none' : '';
    if (providersWrap) providersWrap.style.display = hideProviders ? 'none' : '';

    if (providersWrap && !noHerald && !hideProviders) {
      var providerOrder = scenarioId === 's5-gate-warden-herald-plugins'
        ? ['herald-smtp', 'herald-dingtalk', 'herald-totp']
        : ['herald-smtp', 'herald-dingtalk', 'herald-totp'];
      var rank = {};
      providerOrder.forEach(function (id, idx) { rank[id] = idx; });
      reorderElements(providersWrap, '.provider-row[data-provider-id]', function (el) {
        var id = el.getAttribute('data-provider-id') || '';
        return rank[id] !== undefined ? rank[id] : 999;
      });
      if (scenarioId === 's5-gate-warden-herald-plugins') {
        qa('.provider-row details', providersWrap).forEach(function (d) { d.open = true; });
      }
    }
  }

  function ensureScenarioSelectPersistence() {
    var container = document.getElementById('scenario-list-group');
    var select = q('#scenario-select');
    if (container) {
      var saved = localStorage.getItem(SCENARIO_STORAGE_KEY) || '';
      var current = getScenarioValueFromPage();
      if (!current && saved && (window.SCENARIOS || {})[saved]) {
        setScenarioValueOnPage(saved);
      }
      container.addEventListener('change', function (e) {
        if (e.target && e.target.name === 'scenario') persistScenario(e.target.value || '');
      });
      var form = q('#form-step-1');
      if (form) {
        form.addEventListener('submit', function () {
          persistScenario(getScenarioValueFromPage() || '');
        });
      }
      return;
    }
    if (!select) return;
    var saved = localStorage.getItem(SCENARIO_STORAGE_KEY) || '';
    if (!select.value && saved) {
      var hasSaved = !!q('option[value="' + saved + '"]', select);
      if (hasSaved) {
        select.value = saved;
        select.dispatchEvent(new Event('change', { bubbles: true }));
      }
    }
    select.addEventListener('change', function () {
      persistScenario(this.value || '');
    });
    var form = q('#form-step-1');
    if (form) {
      form.addEventListener('submit', function () {
        persistScenario(select.value || '');
      });
    }
  }

  function applyScenarioTuning() {
    var scenarioId = readActiveScenarioId();
    if (!scenarioId) return;
    var preset = getScenarioPreset(scenarioId);
    if (!preset) return;
    applyPresetToCurrentPage(preset);
    reorderStep2Sections(scenarioId);
    tuneStargateStep(scenarioId);
    tuneWardenStep(scenarioId);
    tuneHeraldStep(scenarioId);
  }

  ensureScenarioSelectPersistence();
  applyScenarioTuning();
  qa('[data-lang]').forEach(function (el) {
    el.addEventListener('click', function () {
      setTimeout(applyScenarioTuning, 0);
    });
  });
})();
/**
 * Stargate Suite compose generator UI.
 * Expects window.I18N to be set by the server-rendered page (from config/page.yaml).
 */
(function () {
	'use strict';

	var LANG_STORAGE_KEY = 'stargate-suite-lang';
	var scenarioExtraOptions = {};
	var DEFAULT_GENERATE_MODES = ['traefik'];

	// 密钥生成：与「生成部署配置」中环境变量对应，genType: apiKey(hex32) | hmacSecret | hmacKeys(JSON) | aes32(base64) | password(base64url)
	var KEY_DEFINITIONS = [
		{ env: 'WARDEN_API_KEY', labelKey: 'keyLabelWardenApiKey', descKey: 'keyDescWardenApiKey', genType: 'apiKey' },
		{ env: 'WARDEN_OTP_SECRET_KEY', labelKey: 'keyLabelWardenOtpSecretKey', descKey: 'keyDescWardenOtpSecretKey', genType: 'apiKey' },
		{ env: 'HERALD_API_KEY', labelKey: 'keyLabelHeraldApiKey', descKey: 'keyDescHeraldApiKey', genType: 'apiKey' },
		{ env: 'HERALD_HMAC_SECRET', labelKey: 'keyLabelHeraldHmacSecret', descKey: 'keyDescHeraldHmacSecret', genType: 'hmacSecret' },
		{ env: 'HERALD_HMAC_KEYS', labelKey: 'keyLabelHeraldHmacKeys', descKey: 'keyDescHeraldHmacKeys', genType: 'hmacKeys' },
		{ env: 'HERALD_TOTP_API_KEY', labelKey: 'keyLabelHeraldTotpApiKey', descKey: 'keyDescHeraldTotpApiKey', genType: 'apiKey' },
		{ env: 'HERALD_TOTP_HMAC_SECRET', labelKey: 'keyLabelHeraldTotpHmacSecret', descKey: 'keyDescHeraldTotpHmacSecret', genType: 'hmacSecret' },
		{ env: 'HERALD_TOTP_ENCRYPTION_KEY', labelKey: 'keyLabelHeraldTotpEncryptionKey', descKey: 'keyDescHeraldTotpEncryptionKey', genType: 'aes32' },
		{ env: 'WARDEN_REDIS_PASSWORD', labelKey: 'keyLabelWardenRedisPassword', descKey: 'keyDescWardenRedisPassword', genType: 'password' },
		{ env: 'HERALD_REDIS_PASSWORD', labelKey: 'keyLabelHeraldRedisPassword', descKey: 'keyDescHeraldRedisPassword', genType: 'password' },
		{ env: 'SESSION_STORAGE_REDIS_PASSWORD', labelKey: 'keyLabelSessionRedisPassword', descKey: 'keyDescSessionRedisPassword', genType: 'password' },
		{ env: 'HERALD_DINGTALK_API_KEY', labelKey: 'keyLabelHeraldDingtalkApiKey', descKey: 'keyDescHeraldDingtalkApiKey', genType: 'apiKey' },
		{ env: 'HERALD_SMTP_API_KEY', labelKey: 'keyLabelHeraldSmtpApiKey', descKey: 'keyDescHeraldSmtpApiKey', genType: 'apiKey' }
	];

	function getRandomBytes(n) {
		var arr = new Uint8Array(n);
		if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
			crypto.getRandomValues(arr);
		} else {
			for (var i = 0; i < n; i++) arr[i] = Math.floor(Math.random() * 256);
		}
		return arr;
	}
	function bytesToHex(arr) {
		return Array.prototype.map.call(arr, function (b) { return ('0' + b.toString(16)).slice(-2); }).join('');
	}
	function bytesToBase64(arr) {
		var binary = '';
		for (var i = 0; i < arr.length; i++) binary += String.fromCharCode(arr[i]);
		return typeof btoa !== 'undefined' ? btoa(binary) : '';
	}
	function bytesToBase64Url(arr) {
		return bytesToBase64(arr).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
	}
	function generateKeyValue(genType) {
		switch (genType) {
			case 'apiKey':
			case 'hmacSecret':
				return bytesToHex(getRandomBytes(32));
			case 'aes32':
				return bytesToBase64(getRandomBytes(32));
			case 'password':
				return bytesToBase64Url(getRandomBytes(24));
			case 'hmacKeys': {
				var keyId = 'key-' + bytesToHex(getRandomBytes(4));
				var secret = bytesToHex(getRandomBytes(32));
				return JSON.stringify({ keyId: keyId, secret: secret });
			}
			default:
				return bytesToHex(getRandomBytes(32));
		}
	}
	function generateHmacKeysJson() {
		var keyId = 'stargate-' + bytesToHex(getRandomBytes(4));
		var secret = bytesToHex(getRandomBytes(32));
		var obj = {};
		obj[keyId] = secret;
		return JSON.stringify(obj);
	}
	// 修正 HERALD_HMAC_KEYS 的生成结果为 JSON 对象字符串
	function generateKeyValueFixed(def) {
		if (def.genType === 'hmacKeys') return generateHmacKeysJson();
		return generateKeyValue(def.genType);
	}

	function renderKeysGrid() {
		var grid = document.getElementById('keys-grid');
		if (!grid) return;
		var lang = getLang();
		var t = window.I18N && window.I18N[lang] ? window.I18N[lang] : {};
		grid.innerHTML = '';
		KEY_DEFINITIONS.forEach(function (def) {
			var row = document.createElement('div');
			row.className = 'keys-row pure-g';
			row.setAttribute('data-env', def.env);
			var label = t[def.labelKey] || def.env;
			var desc = t[def.descKey] || '';
			var genLabel = t.keyBtnGenerate || '生成';
			var copyLabel = t.keyBtnCopy || '复制';
			row.innerHTML =
				'<div class="pure-u-1 pure-u-md-2-5 keys-meta">' +
					'<div class="keys-env"><code>' + escapeHtml(def.env) + '</code></div>' +
					'<div class="keys-label" data-i18n="' + def.labelKey + '">' + escapeHtml(label) + '</div>' +
					'<p class="keys-desc-line config-desc" data-i18n="' + def.descKey + '">' + escapeHtml(desc) + '</p>' +
				'</div>' +
				'<div class="pure-u-1 pure-u-md-3-5 keys-value-wrap">' +
					'<input type="text" class="pure-input-1 keys-value" readonly data-env="' + escapeHtml(def.env) + '" placeholder="">' +
					'<div class="keys-buttons">' +
						'<button type="button" class="btn btn-primary keys-gen" data-env="' + escapeHtml(def.env) + '" data-i18n="keyBtnGenerate">' + escapeHtml(genLabel) + '</button> ' +
						'<button type="button" class="btn btn-outline-secondary keys-copy" data-env="' + escapeHtml(def.env) + '" data-i18n="keyBtnCopy">' + escapeHtml(copyLabel) + '</button>' +
					'</div>' +
				'</div>';
			grid.appendChild(row);
		});
		// 事件委托
		grid.addEventListener('click', function (e) {
			var genBtn = e.target.classList && e.target.classList.contains('keys-gen') ? e.target : null;
			var copyBtn = e.target.classList && e.target.classList.contains('keys-copy') ? e.target : null;
			if (genBtn) {
				var env = genBtn.getAttribute('data-env');
				var def = KEY_DEFINITIONS.filter(function (d) { return d.env === env; })[0];
				if (def) {
					var val = generateKeyValueFixed(def);
					var input = grid.querySelector('input.keys-value[data-env="' + env + '"]');
					if (input) input.value = val;
				}
			} else if (copyBtn) {
				var env = copyBtn.getAttribute('data-env');
				var input = grid.querySelector('input.keys-value[data-env="' + env + '"]');
				var statusEl = document.getElementById('keys-copy-status');
				var lang = getLang();
				var t = window.I18N && window.I18N[lang] ? window.I18N[lang] : {};
				var copiedStr = t.keyCopied || '已复制';
				var failedStr = t.keyCopyFailed || '复制失败';
				var origBtn = copyBtn.textContent;
				function setCopyFeedback(success) {
					var msg = success ? copiedStr : failedStr;
					copyBtn.textContent = msg;
					copyBtn.setAttribute('aria-label', msg);
					if (statusEl) { statusEl.textContent = msg; }
					setTimeout(function () {
						copyBtn.textContent = origBtn;
						copyBtn.removeAttribute('aria-label');
						if (statusEl) statusEl.textContent = '';
					}, 800);
				}
				if (input && input.value) {
					var text = input.value;
					if (navigator.clipboard && navigator.clipboard.writeText) {
						navigator.clipboard.writeText(text).then(function () {
							setCopyFeedback(true);
						}).catch(function () {
							input.select();
							var ok = false;
							try { ok = document.execCommand('copy'); } catch (err) {}
							setCopyFeedback(ok);
						});
					} else {
						input.select();
						var ok = false;
						try { ok = document.execCommand('copy'); } catch (err) {}
						setCopyFeedback(ok);
					}
				}
			}
		});
	}

	function fillKeysIntoGenerate() {
		var grid = document.getElementById('keys-grid');
		if (!grid) return;
		var payload = {};
		KEY_DEFINITIONS.forEach(function (def) {
			var input = grid.querySelector('input.keys-value[data-env="' + def.env + '"]');
			if (!input || !input.value) return;
			payload[def.env] = input.value;
		});
		if (Object.keys(payload).length === 0) return;
		// Multi-page: POST to backend session then redirect
		fetch('/keys/apply', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(payload)
		}).then(function () { window.location.href = '/wizard/step-2'; });
	}

	function getLang() {
		return localStorage.getItem(LANG_STORAGE_KEY) || 'zh';
	}

	function applyLang(lang) {
		var I18N = window.I18N;
		if (!I18N) return;
		lang = I18N[lang] ? lang : 'zh';
		localStorage.setItem(LANG_STORAGE_KEY, lang);
		document.documentElement.lang = lang === 'zh' ? 'zh-CN' : 'en';
		document.title = I18N[lang].title;
		document.querySelectorAll('[data-i18n]').forEach(function (el) {
			var key = el.getAttribute('data-i18n');
			if (key && I18N[lang][key] !== undefined) el.textContent = I18N[lang][key];
		});
		document.querySelectorAll('[data-i18n-placeholder]').forEach(function (el) {
			var key = el.getAttribute('data-i18n-placeholder');
			if (key && I18N[lang][key] !== undefined) el.placeholder = I18N[lang][key];
		});
		document.querySelectorAll('[data-i18n-aria-label]').forEach(function (el) {
			var key = el.getAttribute('data-i18n-aria-label');
			if (key && I18N[lang][key] !== undefined) el.setAttribute('aria-label', I18N[lang][key]);
		});
		document.querySelectorAll('.lang-switch a').forEach(function (a) {
			a.classList.toggle('active', a.getAttribute('data-lang') === lang);
		});
		renderScenarioPresets();
	}

	document.querySelectorAll('.lang-switch a').forEach(function (a) {
		a.addEventListener('click', function (e) {
			e.preventDefault();
			applyLang(this.getAttribute('data-lang'));
		});
	});
	applyLang(getLang());

	// Tab switch
	function showPanel(tabId) {
		var panels = document.querySelectorAll('.tab-panel');
		var triggers = document.querySelectorAll('.tab-trigger');
		panels.forEach(function (p) {
			var isTarget = p.id === 'panel-' + tabId;
			p.style.display = isTarget ? 'block' : 'none';
			p.setAttribute('aria-hidden', isTarget ? 'false' : 'true');
		});
		triggers.forEach(function (t) {
			var isActive = t.getAttribute('data-tab') === tabId;
			t.classList.toggle('active', isActive);
			t.setAttribute('aria-selected', isActive ? 'true' : 'false');
		});
	}
	// 使用事件委托，避免 i18n 替换按钮文字后点击失效
	var tabBar = document.querySelector('.tab-bar');
	if (tabBar) {
		var tabTriggers = tabBar.querySelectorAll('.tab-trigger');
		function setRovingTabindex(activeTabId) {
			tabTriggers.forEach(function (t) {
				t.setAttribute('tabindex', t.getAttribute('data-tab') === activeTabId ? '0' : '-1');
			});
		}
		setRovingTabindex(document.querySelector('.tab-trigger.active') ? document.querySelector('.tab-trigger.active').getAttribute('data-tab') : 'generate');
		tabBar.addEventListener('click', function (e) {
			var t = e.target;
			while (t && t !== tabBar) {
				if (t.classList && t.classList.contains('tab-trigger')) {
					e.preventDefault();
					var tabId = t.getAttribute('data-tab');
					if (tabId) {
						showPanel(tabId);
						setRovingTabindex(tabId);
					}
					return;
				}
				t = t.parentNode;
			}
		});
		tabBar.addEventListener('keydown', function (e) {
			var triggers = Array.prototype.slice.call(tabBar.querySelectorAll('.tab-trigger'));
			var idx = triggers.indexOf(e.target);
			if (idx === -1 || !e.target.classList.contains('tab-trigger')) return;
			var key = e.key;
			if (key === 'ArrowLeft' || key === 'ArrowRight') {
				e.preventDefault();
				var nextIdx = key === 'ArrowLeft' ? (idx - 1 + triggers.length) % triggers.length : (idx + 1) % triggers.length;
				var nextTab = triggers[nextIdx];
				nextTab.focus();
				var tabId = nextTab.getAttribute('data-tab');
				if (tabId) {
					showPanel(tabId);
					setRovingTabindex(tabId);
				}
			} else if (key === 'Enter' || key === ' ') {
				e.preventDefault();
				var tabId = e.target.getAttribute('data-tab');
				if (tabId) {
					showPanel(tabId);
					setRovingTabindex(tabId);
				}
			}
		});
	}

	// Step nav (single-page only: scroll to section; multi-page uses <a> links, no binding)
	function scrollToStep(stepNum) {
		var id = 'step-compose-type';
		if (stepNum === 2) id = 'step-general';
		else if (stepNum === 3) id = 'step-stargate';
		else if (stepNum === 4) id = 'step-warden';
		else if (stepNum === 5) id = 'step-herald';
		var el = document.getElementById(id);
		if (el) {
			el.scrollIntoView({ behavior: 'smooth', block: 'start' });
			document.querySelectorAll('.step-nav-item').forEach(function (a) {
				a.classList.toggle('active', parseInt(a.getAttribute('data-step'), 10) === stepNum);
			});
			document.querySelectorAll('.config-step').forEach(function (s) {
				s.classList.toggle('id-step-active', parseInt(s.getAttribute('data-step'), 10) === stepNum);
			});
		}
	}
	var singlePageForm = document.getElementById('form');
	var stepNav = document.getElementById('step-nav');
	if (stepNav && singlePageForm) {
		stepNav.addEventListener('click', function (e) {
			var a = e.target && e.target.classList && e.target.classList.contains('step-nav-item') ? e.target : null;
			if (!a || a.tagName !== 'BUTTON') return;
			e.preventDefault();
			var step = parseInt(a.getAttribute('data-step'), 10);
			if (step) scrollToStep(step);
		});
		document.querySelectorAll('.step-next').forEach(function (btn) {
			btn.addEventListener('click', function () {
				var next = parseInt(this.getAttribute('data-next'), 10);
				if (next) scrollToStep(next);
			});
		});
		document.querySelectorAll('.step-prev').forEach(function (btn) {
			btn.addEventListener('click', function () {
				var prev = parseInt(this.getAttribute('data-prev'), 10);
				if (prev) scrollToStep(prev);
			});
		});
		document.querySelectorAll('.step-nav-item').forEach(function (a) {
			a.classList.toggle('active', a.getAttribute('data-step') === '1');
		});
		document.querySelectorAll('.config-step').forEach(function (s) {
			s.classList.toggle('id-step-active', s.getAttribute('data-step') === '1');
		});
	}

	document.querySelectorAll('input[name="redisStorage"]').forEach(function (r) {
		r.addEventListener('change', function () {
			document.getElementById('redisPathInputs').style.display =
				document.getElementById('redisStoragePath').checked ? 'block' : 'none';
		});
	});

	function updateOptionDependents() {
		document.querySelectorAll('[data-depends-on-option]').forEach(function (el) {
			var optionId = el.getAttribute('data-depends-on-option');
			var cb = document.getElementById(optionId);
			var on = cb && (cb.type === 'checkbox' ? cb.checked : (cb.tagName === 'SELECT' && cb.value === 'true'));
			el.style.display = on ? '' : 'none';
		});
		document.querySelectorAll('[data-depends-on-env]').forEach(function (el) {
			var envKey = el.getAttribute('data-depends-on-env');
			var scope = el.closest('.service-cell, .provider-row');
			var cb = scope ? scope.querySelector('[data-env="' + envKey + '"]') : null;
			var on = cb && (cb.type === 'checkbox' ? cb.checked : (cb.value === 'true' || cb.value === '1'));
			el.style.display = on ? '' : 'none';
		});
	}
	updateOptionDependents();
	function scenarioDisplayText(s, lang) {
		if (!s) return { name: '', desc: '', risk: '' };
		var isZh = lang === 'zh';
		var name = (isZh ? s.nameZh : s.nameEn) || s.name || (isZh ? s.nameEn : s.nameZh) || '';
		var desc = (isZh ? s.descriptionZh : s.descriptionEn) || s.description || (isZh ? s.descriptionEn : s.descriptionZh) || '';
		var risk = (isZh ? s.riskNoteZh : s.riskNoteEn) || s.riskNote || (isZh ? s.riskNoteEn : s.riskNoteZh) || '';
		return { name: name, desc: desc, risk: risk };
	}
	function getScenarioById(id) {
		if (!id) return null;
		var scenarios = window.SCENARIOS || {};
		return scenarios[id] || null;
	}
	function resolveModesByScenarioId(id) {
		var s = getScenarioById(id);
		var modes = s && Array.isArray(s.modes) ? s.modes.filter(function (m) { return !!m; }) : [];
		return modes.length ? modes : DEFAULT_GENERATE_MODES.slice();
	}
	function getScenarioValue() {
		var listGroup = document.getElementById('scenario-list-group');
		if (listGroup) {
			var radio = listGroup.querySelector('input[name="scenario"]:checked');
			return radio ? (radio.value || '') : '';
		}
		var select = document.getElementById('scenario-select');
		return select ? (select.value || '') : '';
	}
	function setScenarioValue(id) {
		var listGroup = document.getElementById('scenario-list-group');
		if (listGroup) {
			var radio = listGroup.querySelector('input[name="scenario"][value="' + (id || '') + '"]');
			if (radio) { radio.checked = true; return; }
		}
		var select = document.getElementById('scenario-select');
		if (select) select.value = id || '';
	}
	function getSelectedModes() {
		var id = getScenarioValue();
		return resolveModesByScenarioId(id);
	}
	function syncScenarioModeInputs() {
		var wrap = document.getElementById('scenario-mode-inputs');
		if (!wrap) return;
		wrap.innerHTML = '';
		getSelectedModes().forEach(function (mode) {
			var input = document.createElement('input');
			input.type = 'hidden';
			input.name = 'mode';
			input.value = mode;
			wrap.appendChild(input);
		});
	}
	function renderScenarioPresets() {
		var container = document.getElementById('scenario-list-options');
		if (container) {
			var scenarios = window.SCENARIOS || {};
			var lang = getLang();
			var t = window.I18N && window.I18N[lang] ? window.I18N[lang] : {};
			var saved = typeof localStorage !== 'undefined' ? localStorage.getItem('stargate-suite-selected-scenario') || '' : '';
			var current = getScenarioValue() || (saved && scenarios[saved] ? saved : '');
			container.innerHTML = '';
			Object.keys(scenarios).sort().forEach(function (id) {
				var s = scenarios[id] || {};
				var display = scenarioDisplayText(s, lang);
				var name = display.name || id;
				var desc = display.desc || '';
				var div = document.createElement('div');
				div.className = 'position-relative';
				var radio = document.createElement('input');
				radio.className = 'form-check-input position-absolute top-50 end-0 me-3 fs-5';
				radio.type = 'radio';
				radio.name = 'scenario';
				radio.value = id;
				radio.id = 'scenario-radio-' + id;
				if (current === id) radio.checked = true;
				var label = document.createElement('label');
				label.className = 'list-group-item py-3 pe-5';
				label.htmlFor = radio.id;
				label.innerHTML = '<strong class="fw-semibold">' + escapeHtml(name) + '</strong><span class="d-block small opacity-75">' + escapeHtml(desc || '') + '</span>';
				div.appendChild(radio);
				div.appendChild(label);
				container.appendChild(div);
			});
			var emptyRadio = document.getElementById('scenario-radio-empty');
			if (current && scenarios[current]) {
				if (emptyRadio) emptyRadio.checked = false;
			} else {
				if (emptyRadio) emptyRadio.checked = true;
			}
			var placeholder = emptyRadio;
			if (placeholder && placeholder.nextElementSibling && t.scenarioPresetPlaceholder) {
				var strong = placeholder.nextElementSibling.querySelector('.fw-semibold');
				if (strong) strong.textContent = t.scenarioPresetPlaceholder;
			}
			return;
		}
		var select = document.getElementById('scenario-select');
		if (!select) return;
		var scenarios = window.SCENARIOS || {};
		var lang = getLang();
		var t = window.I18N && window.I18N[lang] ? window.I18N[lang] : {};
		var current = select.value || '';
		while (select.options.length > 1) select.remove(1);
		Object.keys(scenarios).sort().forEach(function (id) {
			var s = scenarios[id] || {};
			var display = scenarioDisplayText(s, lang);
			var opt = document.createElement('option');
			opt.value = id;
			var name = display.name || id;
			var desc = display.desc || '';
			opt.textContent = desc ? (name + ' - ' + desc) : name;
			select.appendChild(opt);
		});
		if (current && scenarios[current]) select.value = current;
		var placeholder = select.querySelector('option[value=""]');
		if (placeholder && t.scenarioPresetPlaceholder) placeholder.textContent = t.scenarioPresetPlaceholder;
	}
	function setEnvValueByKey(key, val) {
		document.querySelectorAll('[data-env="' + key + '"]').forEach(function (el) {
			if (el.type === 'checkbox') {
				var v = String(val).toLowerCase();
				el.checked = (v === 'true' || v === '1');
			} else if (el.tagName === 'SELECT') {
				el.value = String(val);
			} else {
				el.value = String(val);
			}
		});
	}
	function applyScenarioPreset(id) {
		var scenarios = window.SCENARIOS || {};
		var s = scenarios[id];
		var descEl = document.getElementById('scenario-desc');
		var riskEl = document.getElementById('scenario-risk');
		var lang = getLang();
		var t = window.I18N && window.I18N[lang] ? window.I18N[lang] : {};
		if (!s) {
			scenarioExtraOptions = {};
			if (descEl) descEl.textContent = t.scenarioPresetDesc || '';
			if (riskEl) {
				riskEl.textContent = '';
				riskEl.style.display = 'none';
			}
			syncScenarioModeInputs();
			return;
		}
		scenarioExtraOptions = {};
		var opts = s.options || {};
		Object.keys(opts).forEach(function (k) {
			var v = opts[k];
			if (k === 'useNamedVolume') {
				var useVolume = !!v;
				var volumeEl = document.getElementById('redisStorageVolume');
				var pathEl = document.getElementById('redisStoragePath');
				if (volumeEl && pathEl) {
					volumeEl.checked = useVolume;
					pathEl.checked = !useVolume;
					document.getElementById('redisPathInputs').style.display = useVolume ? 'none' : 'block';
				}
				return;
			}
			var el = document.querySelector('[data-option="' + k + '"]');
			if (el) {
				if (el.type === 'checkbox') el.checked = !!v;
				else el.value = String(v);
			} else {
				scenarioExtraOptions[k] = v;
			}
		});
		var envs = s.envOverrides || {};
		Object.keys(envs).forEach(function (k) { setEnvValueByKey(k, envs[k]); });
		updateOptionDependents();
		if (descEl || riskEl) {
			var display = scenarioDisplayText(s, lang);
			var name = display.name || id;
			var desc = display.desc || '';
			if (descEl) {
				descEl.textContent = desc ? (name + ' - ' + desc) : name;
			}
			if (riskEl) {
				if (display.risk) {
					var riskPrefix = t.scenarioRiskPrefix || '风险提示';
					riskEl.textContent = riskPrefix + ': ' + display.risk;
					riskEl.style.display = '';
				} else {
					riskEl.textContent = '';
					riskEl.style.display = 'none';
				}
			}
		}
		syncScenarioModeInputs();
	}
	var scenarioContainer = document.getElementById('scenario-list-group');
	if (scenarioContainer) {
		renderScenarioPresets();
		applyScenarioPreset(getScenarioValue() || '');
		syncScenarioModeInputs();
		scenarioContainer.addEventListener('change', function (e) {
			if (e.target && e.target.name === 'scenario') {
				applyScenarioPreset(e.target.value || '');
				syncScenarioModeInputs();
			}
		});
	} else {
		var scenarioSelect = document.getElementById('scenario-select');
		if (scenarioSelect) {
			renderScenarioPresets();
			syncScenarioModeInputs();
			scenarioSelect.addEventListener('change', function () {
				applyScenarioPreset(this.value || '');
				syncScenarioModeInputs();
			});
		}
	}
	document.querySelectorAll('[data-depends-on-option]').forEach(function (el) {
		var optionId = el.getAttribute('data-depends-on-option');
		var cb = document.getElementById(optionId);
		if (cb && !cb._dependentBound) {
			cb._dependentBound = true;
			cb.addEventListener('change', updateOptionDependents);
		}
	});
	document.querySelectorAll('[data-depends-on-env]').forEach(function (el) {
		var envKey = el.getAttribute('data-depends-on-env');
		var scope = el.closest('.service-cell, .provider-row');
		var cb = scope ? scope.querySelector('[data-env="' + envKey + '"]') : null;
		if (cb && !cb._dependentBound) {
			cb._dependentBound = true;
			cb.addEventListener('change', updateOptionDependents);
		}
	});

	if (singlePageForm) {
		singlePageForm.onsubmit = function (e) {
			e.preventDefault();
			runGenerateFromForm();
		};
	}

	function runGenerateFromForm() {
		var I18N = window.I18N;
		if (!I18N) return;
		var lang = getLang();
		var t = I18N[lang] || I18N.zh;
		var modes = getSelectedModes();
		var options = { envOverrides: {} };
		document.querySelectorAll('[data-option]').forEach(function (el) {
			var key = el.getAttribute('data-option');
			if (!key) return;
			if (el.type === 'checkbox') options[key] = el.checked;
			else if (el.tagName === 'SELECT') options[key] = (el.value === 'true' || el.value === 'false') ? (el.value === 'true') : (el.value || '').trim();
			else options[key] = (el.value || '').trim();
		});
		var redisVol = document.getElementById('redisStorageVolume');
		options.useNamedVolume = redisVol ? redisVol.checked : true;
		options.traefikNetworkName = options.traefikNetworkName || 'traefik';
		Object.keys(scenarioExtraOptions).forEach(function (k) {
			options[k] = scenarioExtraOptions[k];
		});
		if (options.useNamedVolume) {
			options.heraldRedisDataPath = '';
			options.wardenRedisDataPath = '';
		} else {
			options.heraldRedisDataPath = options.heraldRedisDataPath || './data/herald-redis';
			options.wardenRedisDataPath = options.wardenRedisDataPath || './data/warden-redis';
		}
		var envOverrides = options.envOverrides;

		document.querySelectorAll('input[name="envBool"]').forEach(function (c) {
			var key = c.getAttribute('data-env');
			if (key) envOverrides[key] = c.checked ? 'true' : 'false';
		});

		document.querySelectorAll('[data-env]').forEach(function (el) {
			var key = el.getAttribute('data-env');
			if (!key || el.getAttribute('name') === 'envBool') return;
			var val = (el.value || '').trim();
			if (el.tagName === 'SELECT') {
				if (val !== '') envOverrides[key] = val;
			} else if (val !== '') {
				envOverrides[key] = val;
			}
		});

		var resultEl = document.getElementById('result');
		if ((modes.indexOf('traefik') !== -1 || modes.indexOf('traefik-stargate') !== -1)) {
			var requiredStargate = ['AUTH_HOST', 'WARDEN_URL', 'HERALD_URL'];
			var missing = requiredStargate.filter(function (k) {
				var v = envOverrides[k];
				return v === undefined || v === null || !String(v).trim();
			});
			if (missing.length > 0) {
				resultEl.textContent = (t.validationMissingEnv || '') + missing.join(', ');
				resultEl.className = 'error';
				document.getElementById('downloads').innerHTML = '';
				var pw = document.getElementById('config-preview-wrap');
				if (pw) { pw.style.display = 'none'; pw.setAttribute('aria-hidden', 'true'); }
				return;
			}
		}
		doGenerateRequest({ modes: modes, options: options });
	}

	function doGenerateRequest(body) {
		var I18N = window.I18N;
		if (!I18N) return;
		var lang = getLang();
		var t = I18N[lang] || I18N.zh;
		var resultEl = document.getElementById('result');
		var downloadsEl = document.getElementById('downloads');
		var previewWrap = document.getElementById('config-preview-wrap');
		var previewContent = document.getElementById('config-preview-content');
		var submitBtn = document.getElementById('btn-generate');
		if (resultEl) resultEl.textContent = t.generating;
		if (downloadsEl) downloadsEl.innerHTML = '';
		if (previewWrap) { previewWrap.style.display = 'none'; previewWrap.setAttribute('aria-hidden', 'true'); }
		if (previewContent) previewContent.innerHTML = '';
		if (submitBtn) submitBtn.disabled = true;
		var hasBody = body && typeof body === 'object' && Object.keys(body).length > 0;
		var url = hasBody ? '/api/generate' : '/generate';
		var opts = { method: 'POST', headers: { 'Content-Type': 'application/json' } };
		opts.body = hasBody ? JSON.stringify(body) : '{}';
		fetch(url, opts)
			.then(function (r) {
				if (!r.ok) throw new Error(r.statusText);
				return r.json();
			})
			.then(function (data) {
				resultEl.className = '';
				downloadsEl.innerHTML = '';
				var fallbackHint = false;
				try {
					for (var mode in data.composes) {
						var blob = new Blob([data.composes[mode]], { type: 'application/x-yaml;charset=utf-8' });
						var url = URL.createObjectURL(blob);
						var a = document.createElement('a');
						a.href = url;
						a.download = mode + '/docker-compose.yml';
						a.textContent = mode + '/docker-compose.yml';
						downloadsEl.appendChild(a);
					}
					var envBlob = new Blob([data.env], { type: 'text/plain;charset=utf-8' });
					var envUrl = URL.createObjectURL(envBlob);
					var envA = document.createElement('a');
					envA.href = envUrl;
					envA.download = '.env';
					envA.textContent = '.env';
					downloadsEl.appendChild(envA);
				} catch (e) {
					fallbackHint = true;
				}
				resultEl.textContent = fallbackHint ? (t.resultSuccess + ' ' + (t.resultDownloadFallbackHint || '')) : t.resultSuccess;
				// 预览区：仅生成成功后显示，默认折叠；每个配置块独立全选与复制按钮
				if (previewWrap && previewContent) {
					var composeLabel = t.previewComposeLabel || 'docker-compose.yml';
					var envLabel = t.previewEnvLabel || '.env';
					var selectAllLabel = t.previewSelectAll || '全选';
					var copyLabel = t.previewCopy || '复制';
					var html = '';
					for (var m in data.composes) {
						html += '<div class="config-preview-block"><div class="config-preview-heading-row"><h4 class="config-preview-heading">' + escapeHtml(m + '/' + composeLabel) + '</h4><button type="button" class="btn btn-sm btn-outline-secondary config-preview-block-select-all">' + escapeHtml(selectAllLabel) + '</button> <button type="button" class="btn btn-sm btn-outline-secondary config-preview-block-copy">' + escapeHtml(copyLabel) + '</button></div><pre class="config-preview-pre">' + escapeHtml(data.composes[m]) + '</pre></div>';
					}
					html += '<div class="config-preview-block"><div class="config-preview-heading-row"><h4 class="config-preview-heading">' + escapeHtml(envLabel) + '</h4><button type="button" class="btn btn-sm btn-outline-secondary config-preview-block-select-all">' + escapeHtml(selectAllLabel) + '</button> <button type="button" class="btn btn-sm btn-outline-secondary config-preview-block-copy">' + escapeHtml(copyLabel) + '</button></div><pre class="config-preview-pre">' + escapeHtml(data.env) + '</pre></div>';
					previewContent.innerHTML = html;
					previewWrap.style.display = '';
					previewWrap.setAttribute('aria-hidden', 'false');
				}
			})
			.catch(function (err) {
				resultEl.textContent = t.requestFailed + (err && err.message ? err.message : String(err));
				resultEl.className = 'error';
				if (previewWrap) {
					previewWrap.style.display = 'none';
					previewWrap.setAttribute('aria-hidden', 'true');
				}
			})
			.finally(function () {
				if (submitBtn) submitBtn.disabled = false;
			});
	}

	// Multi-page review: generate from session (no form)
	if (!singlePageForm) {
		var btnGen = document.getElementById('btn-generate');
		if (btnGen) btnGen.addEventListener('click', function () { doGenerateRequest(null); });
	}

	// Parse (import-parse tab). Convention: API errors and parse messages are plain text only; we use textContent for error display to avoid XSS.
	var btnParse = document.getElementById('btn-parse');
	var parseResultEl = document.getElementById('parse-result');
	if (btnParse && parseResultEl) {
		btnParse.addEventListener('click', function () {
			var I18N = window.I18N;
			if (!I18N) return;
			var lang = getLang();
			var t = I18N[lang] || I18N.zh;
			var composeEl = document.getElementById('input-compose');
			var envEl = document.getElementById('input-env');
			var compose = (composeEl && composeEl.value) ? composeEl.value.trim() : '';
			var env = (envEl && envEl.value) ? envEl.value.trim() : '';
			if (!compose) {
				parseResultEl.innerHTML = '';
				parseResultEl.textContent = t.importComposeRequired || '请粘贴 docker-compose 内容。';
				parseResultEl.className = 'error';
				return;
			}
			parseResultEl.textContent = t.parsing || '解析中…';
			parseResultEl.className = '';
			btnParse.disabled = true;
			fetch('/api/parse', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ compose: compose, env: env })
			})
				.then(function (r) {
					return r.json().then(function (data) {
						return { ok: r.ok, data: data };
					});
				})
				.then(function (res) {
					if (res.ok && (!res.data.errors || res.data.errors.length === 0)) {
						var html = '';
						if (res.data.services && res.data.services.length > 0) {
							html += '<h3 class="parse-result-heading">' + (t.parseServicesLabel || '服务') + '</h3><ul class="parse-services-list">';
							res.data.services.forEach(function (s) {
								html += '<li>' + escapeHtml(s) + '</li>';
							});
							html += '</ul>';
						}
						if (res.data.envVars && Object.keys(res.data.envVars).length > 0) {
							var nameLabel = t.parseEnvNameLabel || '解析项目名称';
							var defaultLabel = t.parseEnvDefaultLabel || '解析的默认数值';
							var editableLabel = t.parseEnvEditableLabel || '可修改的数值';
							html += '<h3 class="parse-result-heading">' + (t.parseEnvVarsLabel || '环境变量') + '</h3><table class="parse-env-table parse-env-table-three pure-table pure-table-bordered"><thead><tr><th>' + escapeHtml(nameLabel) + '</th><th>' + escapeHtml(defaultLabel) + '</th><th>' + escapeHtml(editableLabel) + '</th></tr></thead><tbody>';
							Object.keys(res.data.envVars).sort().forEach(function (k) {
								html += '<tr><td class="parse-env-name"><code>' + escapeHtml(k) + '</code></td><td class="parse-env-default"><code data-env-default-for="' + escapeHtml(k) + '"></code></td><td class="parse-env-editable-cell"><input type="text" class="parse-env-editable pure-input-1" data-env-key="' + escapeHtml(k) + '"></td></tr>';
							});
							html += '</tbody></table>';
						}
						parseResultEl.innerHTML = html || (t.parseSuccess || '解析成功，未识别到服务或环境变量。');
						parseResultEl.className = '';
						// 装填解析的默认数值与可编辑列默认值（在 innerHTML 之后执行，避免特殊字符问题）
						if (res.ok && res.data.envVars && Object.keys(res.data.envVars).length > 0) {
							Object.keys(res.data.envVars).sort().forEach(function (k) {
								var defaultVal = String(res.data.envVars[k] || '');
								var defaultCell = parseResultEl.querySelector('code[data-env-default-for="' + escapeHtml(k) + '"]');
								var editableInput = parseResultEl.querySelector('input.parse-env-editable[data-env-key="' + escapeHtml(k) + '"]');
								if (defaultCell) defaultCell.textContent = defaultVal;
								if (editableInput) { editableInput.value = defaultVal; editableInput.placeholder = defaultVal; }
							});
						}
						// 加载到生成配置按钮
						var actionsDiv = document.createElement('div');
						actionsDiv.className = 'parse-result-actions';
						var loadBtn = document.createElement('button');
						loadBtn.type = 'button';
						loadBtn.id = 'btn-load-into-generate';
						loadBtn.className = 'btn btn-primary';
						loadBtn.setAttribute('data-i18n', 'loadIntoGenerate');
						loadBtn.textContent = t.loadIntoGenerate || '加载到生成配置';
						actionsDiv.appendChild(loadBtn);
						parseResultEl.appendChild(actionsDiv);
						loadBtn.addEventListener('click', function () {
							var compose = (composeEl && composeEl.value) ? composeEl.value.trim() : '';
							if (!compose) {
								parseResultEl.textContent = t.importComposeRequired || '请粘贴 docker-compose 内容。';
								parseResultEl.className = 'error';
								return;
							}
							// 从预览表格的可编辑列收集用户修正后的数值，构建 env 文本
							var envLines = [];
							parseResultEl.querySelectorAll('input.parse-env-editable').forEach(function (input) {
								var key = input.getAttribute('data-env-key');
								if (key) {
									var val = (input.value || '').trim();
									envLines.push(key + '=' + val);
								}
							});
							var envText = envLines.length > 0 ? envLines.join('\n') : ((envEl && envEl.value) ? envEl.value.trim() : '');
							loadBtn.disabled = true;
							loadBtn.textContent = t.applying || '加载中…';
							fetch('/import/apply', {
								method: 'POST',
								headers: { 'Content-Type': 'application/json' },
								body: JSON.stringify({ compose: compose, env: envText })
							})
								.then(function (resp) {
									if (resp.ok) {
										window.location.href = '/wizard/step-1';
										return null;
									}
									return resp.json().then(function (data) { return { ok: false, data: data }; });
								})
								.then(function (res) {
									if (!res) return;
									if (!res.ok) {
										loadBtn.disabled = false;
										loadBtn.textContent = t.loadIntoGenerate || '加载到生成配置';
										parseResultEl.textContent = (res.data && res.data.errors && res.data.errors.length) ? res.data.errors.join('\n') : (t.applyFailed || '加载失败');
										parseResultEl.className = 'error';
									}
								})
								.catch(function (err) {
									loadBtn.disabled = false;
									loadBtn.textContent = t.loadIntoGenerate || '加载到生成配置';
									parseResultEl.textContent = (t.requestFailed || '请求失败: ') + (err && err.message ? err.message : String(err));
									parseResultEl.className = 'error';
								});
						});
					} else {
						var errMsg = (res.data.errors && res.data.errors.length) ? res.data.errors.join('\n') : (t.parseError || '解析失败');
						parseResultEl.textContent = errMsg;
						parseResultEl.className = 'error';
					}
				})
				.catch(function (err) {
					parseResultEl.textContent = (t.requestFailed || '请求失败: ') + (err && err.message ? err.message : String(err));
					parseResultEl.className = 'error';
				})
				.finally(function () {
					btnParse.disabled = false;
				});
		});
	}
	function escapeHtml(s) {
		var div = document.createElement('div');
		div.textContent = s;
		return div.innerHTML;
	}

	// 预览区：每个配置块独立全选（事件委托，因块为动态生成）
	var previewContentEl = document.getElementById('config-preview-content');
	var previewStatusEl = document.getElementById('config-preview-status');
	if (previewContentEl) {
		previewContentEl.addEventListener('click', function (e) {
			var isSelectAll = e.target && e.target.classList && e.target.classList.contains('config-preview-block-select-all');
			var isCopy = e.target && e.target.classList && e.target.classList.contains('config-preview-block-copy');
			if (!isSelectAll && !isCopy) return;
			var block = e.target.closest('.config-preview-block');
			var pre = block && block.querySelector('.config-preview-pre');
			if (!pre || !pre.textContent) return;
			e.preventDefault();
			var lang = getLang();
			var t = window.I18N && window.I18N[lang] ? window.I18N[lang] : {};
			if (isSelectAll) {
				var range = document.createRange();
				range.selectNodeContents(pre);
				var sel = window.getSelection();
				sel.removeAllRanges();
				sel.addRange(range);
				if (previewStatusEl) {
					previewStatusEl.textContent = t.previewSelectedHint || '已选中，请使用 Ctrl+C 复制。';
					setTimeout(function () { previewStatusEl.textContent = ''; }, 2000);
				}
			} else {
				var text = pre.textContent;
				var copiedStr = t.keyCopied || '已复制';
				var failedStr = t.keyCopyFailed || '复制失败';
				function setStatus(success) {
					if (previewStatusEl) previewStatusEl.textContent = success ? copiedStr : failedStr;
					setTimeout(function () { if (previewStatusEl) previewStatusEl.textContent = ''; }, 800);
				}
				if (navigator.clipboard && navigator.clipboard.writeText) {
					navigator.clipboard.writeText(text).then(function () { setStatus(true); }).catch(function () {
						var range = document.createRange();
						range.selectNodeContents(pre);
						window.getSelection().removeAllRanges();
						window.getSelection().addRange(range);
						try { setStatus(document.execCommand('copy')); } catch (err) { setStatus(false); }
						window.getSelection().removeAllRanges();
					});
				} else {
					var range = document.createRange();
					range.selectNodeContents(pre);
					window.getSelection().removeAllRanges();
					window.getSelection().addRange(range);
					try { setStatus(document.execCommand('copy')); } catch (err) { setStatus(false); }
					window.getSelection().removeAllRanges();
				}
			}
		});
	}

	// 密钥生成 Tab：初始化网格、全部生成、填入生成配置
	renderKeysGrid();
	var btnGenerateAllKeys = document.getElementById('btn-generate-all-keys');
	if (btnGenerateAllKeys) {
		btnGenerateAllKeys.addEventListener('click', function () {
			KEY_DEFINITIONS.forEach(function (def) {
				var input = document.querySelector('#keys-grid input.keys-value[data-env="' + def.env + '"]');
				if (input) input.value = generateKeyValueFixed(def);
			});
		});
	}
	var btnFillKeysIntoGenerate = document.getElementById('btn-fill-keys-into-generate');
	if (btnFillKeysIntoGenerate) {
		btnFillKeysIntoGenerate.addEventListener('click', fillKeysIntoGenerate);
	}

})();
