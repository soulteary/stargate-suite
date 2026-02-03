/**
 * Stargate Suite compose generator UI.
 * Expects window.I18N to be set by the server-rendered page (from config/page.yaml).
 */
(function () {
	'use strict';

	var LANG_STORAGE_KEY = 'stargate-suite-lang';

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
		tabBar.addEventListener('click', function (e) {
			var t = e.target;
			while (t && t !== tabBar) {
				if (t.classList && t.classList.contains('tab-trigger')) {
					e.preventDefault();
					var tabId = t.getAttribute('data-tab');
					if (tabId) showPanel(tabId);
					return;
				}
				t = t.parentNode;
			}
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
			el.style.display = cb && cb.checked ? '' : 'none';
		});
		document.querySelectorAll('[data-depends-on-env]').forEach(function (el) {
			var envKey = el.getAttribute('data-depends-on-env');
			var cb = document.getElementById('env_' + envKey);
			el.style.display = cb && cb.checked ? '' : 'none';
		});
	}
	updateOptionDependents();
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
		var cb = document.getElementById('env_' + envKey);
		if (cb && !cb._dependentBound) {
			cb._dependentBound = true;
			cb.addEventListener('change', updateOptionDependents);
		}
	});

	document.getElementById('form').onsubmit = function (e) {
		e.preventDefault();
		var I18N = window.I18N;
		if (!I18N) return;
		var lang = getLang();
		var t = I18N[lang] || I18N.zh;
		var modes = [];
		document.querySelectorAll('input[name="mode"]:checked').forEach(function (c) {
			modes.push(c.value);
		});
		if (modes.length === 0) {
			var resultEl = document.getElementById('result');
			resultEl.textContent = t.resultErrorNeedMode;
			resultEl.className = 'error';
			document.getElementById('downloads').innerHTML = '';
			var pw = document.getElementById('config-preview-wrap');
			if (pw) { pw.style.display = 'none'; pw.setAttribute('aria-hidden', 'true'); }
			return;
		}
		var options = { envOverrides: {} };
		document.querySelectorAll('[data-option]').forEach(function (el) {
			var key = el.getAttribute('data-option');
			if (!key) return;
			if (el.type === 'checkbox') options[key] = el.checked;
			else options[key] = (el.value || '').trim();
		});
		options.useNamedVolume = document.getElementById('redisStorageVolume').checked;
		options.traefikNetworkName = options.traefikNetworkName || 'traefik';
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
		var downloadsEl = document.getElementById('downloads');
		var previewWrap = document.getElementById('config-preview-wrap');
		var previewContent = document.getElementById('config-preview-content');
		var submitBtn = document.getElementById('btn-generate');
		resultEl.textContent = t.generating;
		downloadsEl.innerHTML = '';
		if (previewWrap) {
			previewWrap.style.display = 'none';
			previewWrap.setAttribute('aria-hidden', 'true');
		}
		if (previewContent) previewContent.innerHTML = '';
		if (submitBtn) {
			submitBtn.disabled = true;
		}
		fetch('/api/generate', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ modes: modes, options: options })
		})
			.then(function (r) {
				if (!r.ok) throw new Error(r.statusText);
				return r.json();
			})
			.then(function (data) {
				resultEl.textContent = t.resultSuccess;
				resultEl.className = '';
				downloadsEl.innerHTML = '';
				for (var mode in data.composes) {
					var a = document.createElement('a');
					a.href =
						'data:application/x-yaml;charset=utf-8,' +
						encodeURIComponent(data.composes[mode]);
					a.download = mode + '/docker-compose.yml';
					a.textContent = mode + '/docker-compose.yml';
					downloadsEl.appendChild(a);
				}
				var envA = document.createElement('a');
				envA.href =
					'data:text/plain;charset=utf-8,' + encodeURIComponent(data.env);
				envA.download = '.env';
				envA.textContent = '.env';
				downloadsEl.appendChild(envA);
				// 预览区：仅生成成功后显示，默认折叠
				if (previewWrap && previewContent) {
					var composeLabel = t.previewComposeLabel || 'docker-compose.yml';
					var envLabel = t.previewEnvLabel || '.env';
					var html = '';
					for (var m in data.composes) {
						html += '<div class="config-preview-block"><h4 class="config-preview-heading">' + escapeHtml(m + '/' + composeLabel) + '</h4><pre class="config-preview-pre">' + escapeHtml(data.composes[m]) + '</pre></div>';
					}
					html += '<div class="config-preview-block"><h4 class="config-preview-heading">' + escapeHtml(envLabel) + '</h4><pre class="config-preview-pre">' + escapeHtml(data.env) + '</pre></div>';
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
	};

	// Parse (import-parse tab)
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
							html += '<h3 class="parse-result-heading">' + (t.parseEnvVarsLabel || '环境变量') + '</h3><table class="parse-env-table pure-table pure-table-bordered"><thead><tr><th>' + (t.parseEnvNameLabel || '变量名') + '</th><th>' + (t.parseEnvDefaultLabel || '默认值') + '</th></tr></thead><tbody>';
							Object.keys(res.data.envVars).sort().forEach(function (k) {
								html += '<tr><td><code>' + escapeHtml(k) + '</code></td><td><code>' + escapeHtml(String(res.data.envVars[k] || '')) + '</code></td></tr>';
							});
							html += '</tbody></table>';
						}
						parseResultEl.innerHTML = html || (t.parseSuccess || '解析成功，未识别到服务或环境变量。');
						parseResultEl.className = '';
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
})();
