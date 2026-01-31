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
		var submitBtn = document.getElementById('btn-generate');
		resultEl.textContent = t.generating;
		downloadsEl.innerHTML = '';
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
			})
			.catch(function (err) {
				resultEl.textContent = t.requestFailed + (err && err.message ? err.message : String(err));
				resultEl.className = 'error';
			})
			.finally(function () {
				if (submitBtn) submitBtn.disabled = false;
			});
	};
})();
