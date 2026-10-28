(() => {
'use strict';

// Email reminders are hidden until a working delivery method exists (see notify.go emailSupported).
const EMAIL_UI = false;
// The DSM login mode is hidden: DSM does not recognise the session from the app (tested), and access is limited to the owner through Tailscale.
const AUTH_UI = false;

// ---------- helpers ----------
const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => Array.from(el.querySelectorAll(s));
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
let lang = 'el';
let settings = null;
const t = (k, ...a) => {
  let s = (I18N[lang] && I18N[lang][k]) ?? I18N.en[k] ?? k;
  a.forEach((v, i) => { s = s.split('{' + i + '}').join(v); });
  return s;
};

const MONTHS = {
  el: ['Ιαν', 'Φεβ', 'Μαρ', 'Απρ', 'Μάι', 'Ιουν', 'Ιουλ', 'Αυγ', 'Σεπ', 'Οκτ', 'Νοε', 'Δεκ'],
  en: ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'],
};
const MONTHS_LONG = {
  el: ['Ιανουάριος', 'Φεβρουάριος', 'Μάρτιος', 'Απρίλιος', 'Μάιος', 'Ιούνιος', 'Ιούλιος', 'Αύγουστος', 'Σεπτέμβριος', 'Οκτώβριος', 'Νοέμβριος', 'Δεκέμβριος'],
  en: ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'],
};
const DOW = { el: ['Δευ', 'Τρι', 'Τετ', 'Πεμ', 'Παρ', 'Σαβ', 'Κυρ'], en: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'] };
const SYMBOL = { EUR: '€', USD: '$', GBP: '£', CHF: 'CHF' };

const pad = (n) => String(n).padStart(2, '0');
const ymd = (y, m, d) => y + '-' + pad(m + 1) + '-' + pad(d);
const parts = (s) => { const p = String(s).split('-').map(Number); return { y: p[0], m: p[1] - 1, d: p[2] }; };
const todayStr = () => { const n = new Date(); return ymd(n.getFullYear(), n.getMonth(), n.getDate()); };

function money(v) {
  if (v === null || v === undefined || v === '') return '';
  const sym = SYMBOL[(settings && settings.currency) || 'EUR'];
  const nf = new Intl.NumberFormat(lang === 'el' ? 'el-GR' : 'en-GB', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  return sym + ' ' + nf.format(v);
}
function fdate(s, withYear) {
  if (!s) return '';
  const p = parts(s);
  const cur = new Date().getFullYear();
  let out = p.d + ' ' + MONTHS[lang][p.m];
  if (withYear || p.y !== cur) out += ' ' + p.y;
  return out;
}
function fdateLong(s) {
  const p = parts(s);
  return p.d + ' ' + MONTHS_LONG[lang][p.m] + ' ' + p.y;
}

const PALETTE = ['rgba(77,111,232,.3)', 'rgba(31,158,143,.3)', 'rgba(224,114,63,.3)', 'rgba(125,107,214,.3)', 'rgba(242,140,91,.3)', 'rgba(154,154,168,.35)'];
const BARS = ['#4d6fe8', '#1f9e8f', '#e0723f', '#7d6bd6', '#9a9aa8', '#c2478a'];
const colorOf = (id) => PALETTE[Math.abs(Number(id) || 0) % PALETTE.length];

// ---------- API ----------
async function api(path, opts = {}) {
  const o = { method: opts.method || 'GET', headers: {} };
  if (opts.body !== undefined) { o.body = JSON.stringify(opts.body); o.headers['Content-Type'] = 'application/json'; }
  let r;
  try { r = await fetch('/api' + path, o); } catch (e) { throw new Error(t('err.network')); }
  if (r.status === 401) { showLogin(); throw new Error('login'); }
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(data.error || 'HTTP ' + r.status);
  return data;
}
const errText = (e) => {
  const k = 'err.' + e.message;
  return I18N[lang] && I18N[lang][k] ? t(k) : t('err.generic') + ' (' + e.message + ')';
};

// ---------- UI helpers ----------
function toast(msg, bad) {
  const el = document.createElement('div');
  el.className = 'toast' + (bad ? ' bad' : '');
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 3200);
}

function modal({ title, body, actions }) {
  return new Promise((resolve) => {
    const m = document.createElement('div');
    m.className = 'modal';
    m.innerHTML = '<div class="box glass" role="dialog" aria-modal="true" aria-label="' + esc(title) + '"><h3>' + esc(title) + '</h3>' + (body || '') +
      '<div class="acts">' + actions.map((a, i) => '<button class="btn ' + (a.cls || '') + '" data-i="' + i + '">' + esc(a.label) + '</button>').join('') + '</div></div>';
    document.body.appendChild(m);
    const done = (v) => { m.remove(); resolve(v); };
    m.addEventListener('click', (e) => {
      if (e.target === m) return done(null);
      const b = e.target.closest('button[data-i]');
      if (!b) return;
      const a = actions[Number(b.dataset.i)];
      done(a.value === undefined ? null : (typeof a.value === 'function' ? a.value(m) : a.value));
    });
    m.addEventListener('keydown', (e) => { if (e.key === 'Escape') done(null); });
    const f = $('input,select', m);
    if (f) f.focus();
  });
}
const confirmBox = (title, text, okLabel, danger) => modal({
  title, body: '<div class="sub">' + esc(text) + '</div>',
  actions: [{ label: t('cancel'), value: false }, { label: okLabel, cls: danger ? 'primary danger' : 'primary', value: true }],
});

function tip(text, label) {
  return '<span class="tip"><button type="button" aria-label="' + esc(label) + '">i</button><span class="tipbox" role="tooltip">' + esc(text) + '</span></span>';
}

function nav(active) {
  const items = [['home', '#/'], ['subs', '#/subs'], ['reports', '#/reports'], ['calendar', '#/calendar'], ['settings', '#/settings']];
  return '<nav class="top glass"><div class="l"><div class="brand"><img src="icon-64.png" alt="">Cyklo</div><div class="tabs">' +
    items.map((i) => '<a href="' + i[1] + '" class="' + (i[0] === active ? 'on' : '') + '">' + t('nav.' + i[0]) + '</a>').join('') +
    '</div></div><div class="r"><div class="lang"><button data-act="lang" data-v="el" class="' + (lang === 'el' ? 'on' : '') + '">EL</button><button data-act="lang" data-v="en" class="' + (lang === 'en' ? 'on' : '') + '">EN</button></div>' +
    '<a class="btn primary" href="#/new">+ ' + t('nav.new') + '</a></div></nav>';
}

function page(active, html) {
  $('#app').innerHTML = nav(active) + html;
}

// ---------- schedule math (same rules as the server) ----------
const stepOf = { weekly: 0, monthly: 1, quarterly: 3, semiannual: 6, annual: 12 };
const dim = (y, m) => new Date(Date.UTC(y, m + 1, 0)).getUTCDate();
function occAt(a, freq, k) {
  const step = stepOf[freq];
  if (step === 0) return new Date(Date.UTC(a.y, a.m, a.d + 7 * k));
  const f = new Date(Date.UTC(a.y, a.m + step * k, 1));
  return new Date(Date.UTC(f.getUTCFullYear(), f.getUTCMonth(), Math.min(a.d, dim(f.getUTCFullYear(), f.getUTCMonth()))));
}
function nextOnOrAfter(startStr, freq, fromStr) {
  const a = parts(startStr);
  const from = Date.UTC(parts(fromStr).y, parts(fromStr).m, parts(fromStr).d);
  if (from <= Date.UTC(a.y, a.m, a.d)) return occAt(a, freq, 0);
  let k = 0;
  const step = stepOf[freq];
  if (step === 0) k = Math.floor((from - Date.UTC(a.y, a.m, a.d)) / 86400000 / 7);
  else { const f = parts(fromStr); k = Math.max(0, Math.floor(((f.y - a.y) * 12 + f.m - a.m) / step) - 1); }
  for (let i = 0; i < 5000; i++) { const o = occAt(a, freq, k); if (o.getTime() >= from) return o; k++; }
  return occAt(a, freq, k);
}
const dstr = (d) => ymd(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate());

// ---------- date fields (locale independent: day / month / year) ----------
function yearOpts(sel) {
  const cy = new Date().getFullYear();
  const lo = Math.min(cy - 10, sel || cy), hi = Math.max(cy + 10, sel || cy);
  let o = '';
  for (let y = lo; y <= hi; y++) o += '<option value="' + y + '"' + (y === sel ? ' selected' : '') + '>' + y + '</option>';
  return o;
}
function monthOpts(sel) { return MONTHS_LONG[lang].map((n, i) => '<option value="' + i + '"' + (i === sel ? ' selected' : '') + '>' + n + '</option>').join(''); }
function dateField(id, val) {
  const p = val ? parts(val) : parts(todayStr());
  let days = '';
  for (let d = 1; d <= 31; d++) days += '<option value="' + d + '"' + (d === p.d ? ' selected' : '') + '>' + d + '</option>';
  return '<span class="datef" id="' + id + '"><select class="sel pill" aria-label="' + esc(t('date.day')) + '" data-p="d">' + days + '</select><select class="sel pill" aria-label="' + esc(t('date.month')) + '" data-p="m">' + monthOpts(p.m) + '</select><select class="sel pill" aria-label="' + esc(t('date.year')) + '" data-p="y">' + yearOpts(p.y) + '</select></span>';
}
function dv(id) {
  const el = $('#' + id);
  const g = (k) => Number($('[data-p=' + k + ']', el).value);
  const y = g('y'), m = g('m');
  return ymd(y, m, Math.min(g('d'), dim(y, m)));
}
function setdv(id, val) {
  const el = $('#' + id), p = parts(val);
  $('[data-p=d]', el).value = p.d; $('[data-p=m]', el).value = p.m;
  const ys = $('[data-p=y]', el);
  if (!Array.from(ys.options).some((o) => Number(o.value) === p.y)) ys.insertAdjacentHTML('beforeend', '<option value="' + p.y + '">' + p.y + '</option>');
  ys.value = p.y;
}
function monthField(id, val) {
  const p = parts(val + '-01');
  return '<span class="datef" id="' + id + '"><select class="sel pill" aria-label="' + esc(t('date.month')) + '" data-p="m">' + monthOpts(p.m) + '</select><select class="sel pill" aria-label="' + esc(t('date.year')) + '" data-p="y">' + yearOpts(p.y) + '</select></span>';
}
function mv(id) {
  const el = $('#' + id);
  return $('[data-p=y]', el).value + '-' + pad(Number($('[data-p=m]', el).value) + 1);
}


// ---------- shared rendering ----------
function methodLabel(method, cardName, customName) {
  let s = customName || t('method.' + method);
  if (method === 'card' && cardName) s += ' · «' + cardName + '»';
  return s;
}
function subMeta(s) {
  return [s.categoryName, t('freq.' + s.frequency), methodLabel(s.method, s.cardName, s.methodName)].join(' · ');
}
function initialOf(name) { const m = String(name || '').trim().match(/[\p{L}\p{N}]/u); return m ? m[0].toUpperCase() : '?'; }
function statusChip(st) { return '<span class="chip ' + st + '">' + t('status.' + st) + '</span>'; }

function subRow(s, opts = {}) {
  let amount, nextLine;
  if (s.status !== 'active') {
    amount = s.amountType === 'fixed' ? money(s.amount) : t('type.variable');
    nextLine = s.status === 'paused' ? t('status.paused') : t('status.cancelled');
  } else if (s.amountType === 'variable') {
    amount = '<span class="chip ' + s.next.state + '">' + t('state.' + s.next.state) + '</span>';
    nextLine = (s.next.state === 'missing' ? t('next.expired') : t('next.expected')) + ': ' + fdate(s.next.date);
  } else {
    amount = money(s.amount);
    nextLine = t('next.label') + ': ' + fdate(s.next.date);
  }
  const canRecord = s.status === 'active' && s.amountType === 'variable' && s.next.date;
  const go = opts.nav ? ' go" data-go="#/edit/' + s.id + '" tabindex="0" role="link"' : '';
  return '<div class="row' + go + '"><div class="main"><div class="ico" aria-hidden="true" style="background:' + colorOf(s.categoryId) + '">' + esc(initialOf(s.company)) + '</div><div style="min-width:0"><div class="t">' + esc(s.title) +
    '</div><div class="m">' + esc(opts.short ? methodLabel(s.method, s.cardName, s.methodName) + ' · ' + t('freq.' + s.frequency) : subMeta(s)) + '</div></div></div>' +
    '<div class="side">' + (opts.status ? statusChip(s.status) : '') + '<div style="min-width:120px"><div class="amt">' + amount + '</div><div class="m">' + esc(nextLine) + '</div></div>' +
    (canRecord ? '<button class="btn" data-act="record" data-id="' + s.id + '">' + t('record.btn') + '</button>' : '') +
    (opts.edit ? '<a class="btn link" href="#/edit/' + s.id + '">' + t('edit') + '</a>' : '') + '</div></div>';
}

async function recordFlow(id, reload) {
  let sub;
  try { sub = await api('/subs/' + id); } catch (e) { return; }
  const body = '<div class="sub">' + esc(sub.title) + ' · ' + esc(fdate(sub.next.date, true)) + '</div>' +
    '<div class="field"><label for="amt">' + t('record.amount') + ' (' + SYMBOL[settings.currency] + ')</label><input class="inp" id="amt" type="text" inputmode="decimal" autocomplete="off" placeholder="0,00"></div>';
  const res = await modal({
    title: t('record.title'), body,
    actions: [
      { label: t('record.skip'), cls: 'link', value: 'skip' },
      { label: t('cancel'), value: null },
      { label: t('save'), cls: 'primary', value: (m) => ({ amount: $('#amt', m).value }) },
    ],
  });
  if (!res) return;
  try {
    if (res === 'skip') await api('/subs/' + id + '/skip', { method: 'POST' });
    else {
      const v = parseFloat(String(res.amount).replace(',', '.'));
      if (!(v >= 0)) { toast(t('err.amount'), true); return; }
      await api('/subs/' + id + '/record', { method: 'POST', body: { amount: v } });
    }
    toast(t('saved'));
    reload();
  } catch (e) { toast(errText(e), true); }
}

// ---------- screens ----------
async function viewHome() {
  const [d, ver] = await Promise.all([api('/dashboard'), api('/version').catch(() => ({ version: '' }))]);
  if (!d.subCount) {
    page('home', '<div class="glass card empty"><b>' + t('home.emptyTitle') + '</b><span>' + t('home.emptyText') + '</span><div style="margin-top:16px"><a class="btn primary" href="#/new">+ ' + t('nav.new') + '</a></div></div>');
    return;
  }
  const next = d.next;
  const nextVal = next ? (next.next.days === 0 ? t('days.today') : t('days.in', next.next.days)) : '—';
  const stat = (k, v, dd, attrs, hint) => '<div class="glass stat' + (attrs ? ' go' : '') + '"' + (attrs ? ' ' + attrs + ' tabindex="0" role="link"' : '') + '><div class="k">' + k + '</div><div class="v">' + v + '</div><div class="d">' + dd + '</div></div>';
  const maxMonth = Math.max(1, ...d.months.map((m) => m.amount));
  const curMonth = todayStr().slice(0, 7);
  const upcoming = d.upcoming.map((s) => subRow(s, { short: true, nav: true })).join('') || '<div class="sub" style="padding:14px">—</div>';
  page('home',
    '<div class="stats">' +
    stat(t('home.month'), money(d.monthTotal), t('home.monthHint')) +
    stat(t('home.year'), money(d.yearTotal), t('home.yearHint')) +
    stat(t('home.active'), String(d.activeCount), t('home.activeHint'), 'data-act="tileactive"', t('home.openActive')) +
    stat(t('home.next'), nextVal, next ? esc(next.title) : '', next ? 'data-go="#/edit/' + next.id + '"' : '', t('home.openNext')) + '</div>' +
    (d.missingCount ? '<div class="note"><i></i>' + t('home.missing', d.missingCount) + '</div>' : '') +
    '<div class="cols"><div class="glass card a"><div class="head"><h2>' + t('home.upcoming') + '</h2><a class="btn link" href="#/subs">' + t('home.all') + '</a></div>' + upcoming + '</div>' +
    '<div class="glass card b"><h2 style="margin-bottom:14px">' + t('home.byCategory') + '</h2>' +
    (d.categories.length ? d.categories.map((c, i) => '<div class="barrow"><div class="l"><span>' + esc(c.name) + '</span><span>' + c.pct + '%</span></div><div class="track"><div class="fill" style="width:' + Math.max(c.pct, 2) + '%;background:' + BARS[i % BARS.length] + '"></div></div></div>').join('') : '<div class="sub">' + t('home.noData') + '</div>') +
    '</div></div>' +
    '<div class="glass card"><div class="head"><h2>' + t('home.byMonth') + '</h2><span class="sub">' + t('home.last12') + '</span></div><div class="months">' +
    d.months.map((m) => '<div class="c ' + (m.month === curMonth ? 'cur' : '') + '" title="' + esc(m.month + ': ' + money(m.amount)) + '"><div class="bar" style="height:' + Math.round(m.amount / maxMonth * 100) + '%"></div><div class="lb">' + MONTHS[lang][Number(m.month.slice(5)) - 1] + '</div></div>').join('') +
    '</div></div>' +
    (ver.version ? '<div class="verline"><button class="verbtn" data-act="notes">Cyklo ' + esc(ver.version) + '</button><button class="starbtn" data-act="egg" aria-label="' + esc(t('egg.btn')) + '">★</button></div>' : ''));
}

const ui = {
  returnTo: '',
  subs: { status: 'all', q: '', cat: '' },
  rep: { view: 'method', period: 'month', date: '', status: 'active', open: null },
  cal: { month: null, sel: null },
};

async function viewSubs() {
  const [list, lists] = await Promise.all([api('/subs'), api('/lists')]);
  const f = ui.subs;
  const count = (s) => list.filter((x) => s === 'all' || x.status === s).length;
  const rows = list.filter((x) => (f.status === 'all' || x.status === f.status) && (!f.cat || String(x.categoryId) === f.cat) &&
    (!f.q || (x.company + ' ' + x.service).toLowerCase().includes(f.q.toLowerCase())));
  const tab = (s) => '<button data-act="substab" data-v="' + s + '" class="' + (f.status === s ? 'on' : '') + '">' + t('subs.tab.' + s) + ' (' + count(s) + ')</button>';
  const qs = '?status=' + f.status + '&cat=' + f.cat + '&q=' + encodeURIComponent(f.q);
  page('subs',
    '<h1>' + t('nav.subs') + '</h1><div class="toolbar"><div class="seg">' + ['all', 'active', 'paused', 'cancelled'].map(tab).join('') + '</div>' +
    '<div class="f"><label for="q">' + t('subs.search') + '</label><input class="inp pill" id="q" type="search" style="width:200px" placeholder="' + esc(t('subs.searchPh')) + '" value="' + esc(f.q) + '">' +
    '<label for="cat">' + t('subs.category') + '</label><select class="sel pill" id="cat"><option value="">' + t('subs.allCats') + '</option>' +
    lists.categories.map((c) => '<option value="' + c.id + '"' + (String(c.id) === f.cat ? ' selected' : '') + '>' + esc(c.name) + '</option>').join('') + '</select></div></div>' +
    '<div class="glass card" style="padding:14px 16px" id="subrows">' + (rows.length ? rows.map((s) => subRow(s, { status: true, edit: true })).join('') : '<div class="empty"><b>' + t('subs.none') + '</b><span>' + t('subs.noneHint') + '</span></div>') + '</div>' +
    '<div class="toolbar"><div class="sub">' + t('subs.count', rows.length, list.length) + '</div><div class="f"><a class="btn" href="/api/export/subs' + qs + '&format=xlsx">' + t('export.excel') + '</a><a class="btn" href="/api/export/subs' + qs + '&format=csv">' + t('export.csv') + '</a></div></div>');
  const q = $('#q');
  q.addEventListener('input', () => {
    f.q = q.value;
    const rows2 = list.filter((x) => (f.status === 'all' || x.status === f.status) && (!f.cat || String(x.categoryId) === f.cat) && (!f.q || (x.company + ' ' + x.service).toLowerCase().includes(f.q.toLowerCase())));
    $('#subrows').innerHTML = rows2.length ? rows2.map((s) => subRow(s, { status: true, edit: true })).join('') : '<div class="empty"><b>' + t('subs.none') + '</b><span>' + t('subs.noneHint') + '</span></div>';
  });
  $('#cat').addEventListener('change', (e) => { f.cat = e.target.value; render(); });
}

async function viewReports() {
  const f = ui.rep;
  const now = todayStr();
  if (!f.date) f.date = f.period === 'year' ? now.slice(0, 4) : now.slice(0, 7);
  const q = '?view=' + f.view + '&period=' + f.period + '&date=' + f.date + '&status=' + f.status;
  const r = await api('/report' + q);
  const label = (row) => {
    if (f.view === 'method') return methodLabel(row.method, row.cardName, /^c\d+$/.test(row.method || '') ? row.label : '');
    return row.label;
  };
  const sub = (row) => f.view === 'company' ? t('rep.services', row.subCount) : t('rep.subs', row.subCount);
  const key = (row) => f.view + ':' + row.key;
  const rows = r.rows.map((row, i) => {
    const open = f.open === key(row);
    return '<div class="rep"><button data-act="repopen" data-k="' + esc(key(row)) + '" aria-expanded="' + open + '"><div class="top"><div><div class="t" style="font-weight:600">' + esc(label(row)) + '</div><div class="m sub">' + sub(row) +
      '</div></div><div style="text-align:right"><div style="font-weight:600">' + money(row.amount) + '</div><div class="sub">' + row.pct + '% · ' + (open ? t('rep.hide') : t('rep.details')) + '</div></div></div>' +
      '<div class="track bar"><div class="fill" style="width:' + Math.max(row.pct, row.amount > 0 ? 2 : 0) + '%;background:' + BARS[i % BARS.length] + '"></div></div></button>' +
      (open ? '<div class="det">' + row.items.map((it) => '<div class="it"><span>' + esc(it.title + (it.service ? ' · ' + it.service : '')) + ' <span class="sub">· ' + t('freq.' + it.frequency) + '</span></span><b>' + (it.amount === null ? '—' : money(it.amount)) + '</b></div>').join('') +
        '<a class="btn link" style="margin-top:6px" href="#/subs">' + t('rep.openList') + '</a></div>' : '') + '</div>';
  }).join('') || '<div class="empty"><b>' + t('rep.none') + '</b></div>';
  const dateInput = f.period === 'year'
    ? '<input class="inp pill" id="rdate" type="number" min="2000" max="2100" style="width:110px" value="' + esc(f.date) + '">'
    : monthField('rdate', f.date);
  page('reports',
    '<div class="toolbar"><h1>' + t('nav.reports') + '</h1><div class="f"><label for="rper">' + t('rep.period') + '</label><select class="sel pill" id="rper"><option value="month"' + (f.period === 'month' ? ' selected' : '') + '>' + t('rep.byMonth') + '</option><option value="year"' + (f.period === 'year' ? ' selected' : '') + '>' + t('rep.byYear') + '</option></select>' + dateInput +
    '<label for="rst">' + t('rep.status') + '</label><select class="sel pill" id="rst">' + [['active', 'rep.st.active'], ['activepaused', 'rep.st.activepaused'], ['all', 'rep.st.all']].map((o) => '<option value="' + o[0] + '"' + (f.status === o[0] ? ' selected' : '') + '>' + t(o[1]) + '</option>').join('') + '</select></div></div>' +
    '<div class="seg">' + ['method', 'company', 'category'].map((v) => '<button data-act="repview" data-v="' + v + '" class="' + (f.view === v ? 'on' : '') + '">' + t('rep.view.' + v) + '</button>').join('') + '</div>' +
    '<div class="cols"><div class="glass card a" style="flex:1.6 1 520px"><div class="head"><h2>' + t('rep.title.' + f.view) + '</h2><span class="sub">' + t('rep.pickRow') + '</span></div>' + rows + '</div>' +
    '<div class="b stack"><div class="glass card"><div class="sub" style="color:var(--ink2)">' + t('rep.total') + '</div><div style="margin-top:6px;font-size:34px;font-weight:600;letter-spacing:-.8px">' + money(r.total) + '</div>' +
    (r.pending ? '<div class="sub" style="margin-top:10px">' + t('rep.pending', r.pending) + '</div>' : '') + '</div>' +
    '<div class="glass card"><h2 style="margin-bottom:6px">' + t('rep.export') + '</h2><div class="sub" style="margin-bottom:14px">' + t('rep.exportHint') + '</div><div class="f" style="display:flex;gap:10px;flex-wrap:wrap"><a class="btn" href="/api/export/report' + q + '&format=xlsx">Excel</a><a class="btn" href="/api/export/report' + q + '&format=csv">CSV</a></div></div></div></div>');
  $('#rper').addEventListener('change', (e) => { f.period = e.target.value; f.date = ''; f.open = null; render(); });
  $('#rdate').addEventListener('change', (e) => { f.date = f.period === 'year' ? e.target.value : mv('rdate'); f.open = null; render(); });
  $('#rst').addEventListener('change', (e) => { f.status = e.target.value; f.open = null; render(); });
}

async function viewCalendar() {
  const f = ui.cal;
  const now = todayStr();
  if (!f.month) f.month = now.slice(0, 7);
  const c = await api('/calendar?month=' + f.month);
  const [y, m] = f.month.split('-').map(Number);
  const first = new Date(Date.UTC(y, m - 1, 1));
  const lead = (first.getUTCDay() + 6) % 7;
  const byDate = {};
  c.events.forEach((e) => { (byDate[e.date] = byDate[e.date] || []).push(e); });
  if (!f.sel || f.sel.slice(0, 7) !== f.month) f.sel = (now.slice(0, 7) === f.month) ? now : f.month + '-01';
  const evLabel = (e) => e.title + (e.amount !== null ? ' ' + money(e.amount) : '');
  let cells = '';
  for (let i = 0; i < 42; i++) {
    const dt = new Date(Date.UTC(y, m - 1, 1 - lead + i));
    const ds = dstr(dt);
    const out = dt.getUTCMonth() !== m - 1;
    const evs = byDate[ds] || [];
    const chips = evs.slice(0, 2).map((e) => '<div class="ev ' + (e.amount === null ? 'pend' : '') + '">' + esc(evLabel(e)) + '</div>').join('') + (evs.length > 2 ? '<div class="ev more">+' + (evs.length - 2) + '</div>' : '');
    cells += '<button class="day ' + (out ? 'out ' : '') + (ds === now ? 'today ' : '') + (ds === f.sel ? 'sel' : '') + '" data-act="calsel" data-d="' + ds + '"><div class="n">' + dt.getUTCDate() + '</div>' + chips + '</button>';
  }
  const selEvents = byDate[f.sel] || [];
  const selHtml = selEvents.length ? selEvents.map((e) => '<div class="row go" data-go="#/edit/' + e.subId + '" tabindex="0" role="link" style="margin-top:6px"><div class="main"><div><div class="t">' + esc(e.title) + '</div><div class="m">' +
    (e.amount === null ? t('state.' + (e.state === 'missing' ? 'missing' : 'awaiting')) : (e.state === 'upcoming' ? t('cal.upcoming') : t('cal.charged'))) + '</div></div></div><div class="amt">' + (e.amount !== null ? money(e.amount) : '—') + '</div></div>').join('') : '<div class="sub">' + t('cal.noPay') + '</div>';
  page('calendar',
    '<div class="toolbar"><div class="monthnav"><h1>' + MONTHS_LONG[lang][m - 1] + ' ' + y + '</h1><button class="btn" data-act="calmove" data-v="-1" aria-label="' + t('cal.prev') + '">‹</button><button class="btn" data-act="calmove" data-v="1" aria-label="' + t('cal.next') + '">›</button><button class="btn" data-act="caltoday">' + t('cal.today') + '</button></div></div>' +
    '<div class="cal"><div class="glass card grid"><div class="dow">' + DOW[lang].map((d) => '<div>' + d + '</div>').join('') + '</div><div class="days">' + cells + '</div>' +
    '<div class="legend"><span><span class="ev"></span> ' + t('cal.legendFixed') + '</span><span><span class="ev pend"></span> ' + t('cal.legendPending') + '</span></div></div>' +
    '<div class="side"><div class="glass card"><div class="sub" style="color:var(--ink2)">' + t('cal.monthTotal') + '</div><div style="margin-top:6px;font-size:34px;font-weight:600;letter-spacing:-.8px">' + money(c.total) + '</div>' +
    (c.pending ? '<div class="sub" style="margin-top:10px">' + t('rep.pending', c.pending) + '</div>' : '') + '</div>' +
    '<div class="glass card"><h2 style="margin-bottom:6px">' + t('cal.selected') + '</h2><div class="sub" style="margin-bottom:10px">' + esc(fdateLong(f.sel)) + '</div>' + selHtml + '</div></div></div>');
}

// ----- new / edit subscription -----
async function viewForm(id) {
  const back = ui.returnTo || '#/subs';
  const lists = await api('/lists');
  let s;
  if (id) s = await api('/subs/' + id);
  else s = {
    company: '', service: '', categoryId: lists.categories[0] ? lists.categories[0].id : 0, amountType: 'fixed', amount: null, frequency: 'monthly',
    startDate: todayStr(), manualNext: false, manualDate: '', method: 'card', cardId: 0, status: 'active',
    remindDays: settings.remindDays, remindEmail: true, remindPush: true, notes: '',
  };
  const sel = (arr, cur) => arr.map((o) => '<option value="' + o[0] + '"' + (String(o[0]) === String(cur) ? ' selected' : '') + '>' + esc(o[1]) + '</option>').join('');
  const nextCalc = () => {
    const sd = dv('start'), fr = $('#freq').value;
    if (!sd) return t('form.noNext');
    return fdateLong(dstr(nextOnOrAfter(sd, fr, todayStr())));
  };
  const field = (lab, inner, tipText) => '<div class="field"><div class="lab">' + lab + (tipText ? tip(tipText, lab) : '') + '</div>' + inner + '</div>';
  page('', '<h1>' + (id ? t('form.editTitle') : t('form.newTitle')) + '</h1><div class="glass formcard" id="form">' +
    '<div class="grid2"><div class="field"><label for="company">' + t('form.company') + '</label><input class="inp" id="company" maxlength="80" placeholder="' + esc(t('form.companyPh')) + '" value="' + esc(s.company) + '"></div>' +
    '<div class="field"><label for="service">' + t('form.service') + '</label><input class="inp" id="service" maxlength="80" placeholder="' + esc(t('form.servicePh')) + '" value="' + esc(s.service) + '"></div></div>' +
    field('<label for="category">' + t('form.category') + '</label>', '<select class="sel" id="category">' + sel(lists.categories.map((c) => [c.id, c.name]), s.categoryId) + '<option value="__new">+ ' + esc(t('form.newCat')) + '</option></select>', t('form.categoryTip')) +
    '<div class="field"><div class="lab">' + t('form.amountType') + tip(t('form.amountTypeTip'), t('form.amountType')) + '</div><div class="seg"><button type="button" data-act="atype" data-v="fixed" id="atFixed">' + t('type.fixed') + '</button><button type="button" data-act="atype" data-v="variable" id="atVar">' + t('type.variable') + '</button></div></div>' +
    '<div class="field" id="costBox"><label for="cost">' + t('form.cost') + ' (' + SYMBOL[settings.currency] + ')</label><input class="inp" id="cost" style="max-width:240px" type="text" inputmode="decimal" autocomplete="off" placeholder="0,00" value="' + (s.amount ?? '') + '"></div>' +
    '<div class="note" id="varNote"><i></i>' + t('form.varNote') + '</div>' +
    '<div class="grid2">' + field('<label for="freq">' + t('form.frequency') + '</label>', '<select class="sel" id="freq">' + sel(['monthly', 'annual', 'quarterly', 'semiannual', 'weekly'].map((f) => [f, t('freq.' + f)]), s.frequency) + '</select>', t('form.frequencyTip')) +
    field('<label for="start">' + t('form.start') + '</label>', dateField('start', s.startDate), t('form.startTip')) + '</div>' +
    '<div id="nextAuto" style="display:flex;flex-wrap:wrap;align-items:center;gap:6px;font-size:14px;margin-top:-8px"><span class="sub">' + t('form.next') + ':</span>' + tip(t('form.nextTip'), t('form.next')) + '<b id="nextText"></b><button type="button" class="btn link" data-act="manual">' + t('form.change') + '</button></div>' +
    '<div id="nextManual" style="display:none;flex-wrap:wrap;align-items:center;gap:10px;margin-top:-8px"><label for="manualDate" class="sub">' + t('form.next') + ':</label>' + dateField('manualDate', s.manualDate) + '<button type="button" class="btn link" data-act="manual">' + t('form.auto') + '</button></div>' +
    field('<label for="method">' + t('form.method') + '</label>', '<select class="sel" id="method">' + sel([['card', t('method.card')], ['paypal', t('method.paypal')], ['applepay', t('method.applepay')]].concat(lists.methods.map((m) => ['c' + m.id, m.name])), s.method) + '<option value="__new">+ ' + esc(t('form.newMethod')) + '</option></select>') +
    '<div class="subbox" id="cardBox"><div class="field"><div class="lab"><label for="cardId">' + t('form.card') + '</label>' + tip(t('form.cardTip'), t('form.card')) + '</div><select class="sel" id="cardId"><option value="0">' + t('form.noCard') + '</option>' +
    lists.cards.map((c) => '<option value="' + c.id + '"' + (c.id === s.cardId ? ' selected' : '') + '>«' + esc(c.name) + '»</option>').join('') + '</select></div></div>' +
    '<div class="field"><div class="lab">' + t('form.remind') + tip(t('form.remindTip'), t('form.remind')) + '</div><div style="display:flex;flex-wrap:wrap;align-items:center;gap:16px"><label for="remind" class="sub" style="font-size:14px;color:var(--ink)">' + t('form.notify') + '</label>' +
    '<select class="sel pill" id="remind" style="width:auto">' + sel([[7, t('remind.n', 7)], [3, t('remind.n', 3)], [1, t('remind.1')], [0, t('remind.0')]], s.remindDays) + '</select>' +
    (EMAIL_UI ? '<label class="chk"><input type="checkbox" id="rEmail"' + (s.remindEmail ? ' checked' : '') + '>' + t('notif.email') + '</label>' : '') + '<label class="chk"><input type="checkbox" id="rPush"' + (s.remindPush ? ' checked' : '') + '>Pushover</label></div></div>' +
    '<div class="field"><label for="notes">' + t('form.notes') + '</label><textarea class="txt" id="notes" rows="3" maxlength="500">' + esc(s.notes) + '</textarea></div>' +
    (id ? '<div class="field"><div class="lab">' + t('form.status') + '</div><div class="seg">' + ['active', 'paused', 'cancelled'].map((st) => '<button type="button" data-act="stset" data-v="' + st + '" class="' + (s.status === st ? 'on' : '') + '">' + t('status.' + st) + '</button>').join('') + '</div></div>' : '') +
    '<div class="err" id="ferr"></div>' +
    '<div style="display:flex;flex-wrap:wrap;justify-content:space-between;gap:10px;padding-top:6px"><div>' + (id ? '<button class="btn danger" data-act="del" type="button">' + t('delete') + '</button>' : '') + '</div><div style="display:flex;gap:10px;flex-wrap:wrap"><a class="btn" href="' + back + '">' + t('cancel') + '</a><button class="btn primary" data-act="savesub" type="button">' + t('save') + '</button></div></div></div>');

  let atype = s.amountType;
  let manual = !!s.manualNext;
  let manualSet = !!s.manualDate;
  const refresh = () => {
    $('#atFixed').classList.toggle('on', atype === 'fixed');
    $('#atVar').classList.toggle('on', atype === 'variable');
    $('#costBox').style.display = atype === 'fixed' ? '' : 'none';
    $('#varNote').style.display = atype === 'variable' ? '' : 'none';
    $('#nextAuto').style.display = manual ? 'none' : 'flex';
    $('#nextManual').style.display = manual ? 'flex' : 'none';
    $('#cardBox').style.display = $('#method').value === 'card' ? '' : 'none';
    $('#nextText').textContent = nextCalc();
    if (manual && !manualSet) { setdv('manualDate', dstr(nextOnOrAfter(dv('start'), $('#freq').value, todayStr()))); manualSet = true; }
  };
  const form = $('#form');
  form.addEventListener('click', async (e) => {
    const b = e.target.closest('[data-act]');
    if (!b) return;
    const act = b.dataset.act;
    if (act === 'atype') { atype = b.dataset.v; refresh(); }
    if (act === 'manual') { manual = !manual; refresh(); }
    if (act === 'stset') {
      try { await api('/subs/' + id + '/status', { method: 'POST', body: { status: b.dataset.v } }); toast(t('saved')); render(); } catch (er) { toast(errText(er), true); }
    }
    if (act === 'del') {
      if (await confirmBox(t('form.delTitle'), t('form.delText'), t('delete'), true)) {
        try { await api('/subs/' + id, { method: 'DELETE' }); toast(t('deleted')); location.hash = back; } catch (er) { toast(errText(er), true); }
      }
    }
    if (act === 'savesub') {
      const body = {
        company: $('#company').value, service: $('#service').value, categoryId: Number($('#category').value), amountType: atype,
        amount: atype === 'fixed' ? parseFloat(String($('#cost').value).replace(',', '.')) : null,
        frequency: $('#freq').value, startDate: dv('start'), manualNext: manual, manualDate: manual ? dv('manualDate') : '',
        method: $('#method').value, cardId: $('#method').value === 'card' ? Number($('#cardId').value) : 0,
        remindDays: Number($('#remind').value), remindEmail: EMAIL_UI ? $('#rEmail').checked : s.remindEmail, remindPush: $('#rPush').checked, notes: $('#notes').value,
      };
      if (atype === 'fixed' && !(body.amount >= 0)) body.amount = null;
      try {
        if (id) await api('/subs/' + id, { method: 'PUT', body }); else await api('/subs', { method: 'POST', body });
        toast(t('saved'));
        location.hash = back;
      } catch (er) { $('#ferr').textContent = errText(er); }
    }
  });
  ['#freq', '#start', '#method', '#manualDate'].forEach((sel2) => $(sel2).addEventListener('change', refresh));
  // "+ New ..." entries of the category and payment method lists
  [['#category', '/categories', 'form.newCat'], ['#method', '/methods', 'form.newMethod']].forEach(([sid, path, label]) => {
    const el = $(sid);
    let prev = el.value;
    el.addEventListener('focus', () => { if (el.value !== '__new') prev = el.value; });
    el.addEventListener('change', async () => {
      if (el.value !== '__new') { prev = el.value; return; }
      const name = await modal({ title: t(label), body: '<div class="field"><label for="nn">' + t('form.newName') + '</label><input class="inp" id="nn" maxlength="60"></div>', actions: [{ label: t('cancel'), value: null }, { label: t('add'), cls: 'primary', value: (m) => $('#nn', m).value }] });
      if (!name || !name.trim()) { el.value = prev; refresh(); return; }
      try {
        const r = await api(path, { method: 'POST', body: { name } });
        const key = sid === '#method' ? r.key : String(r.id);
        const opt = document.createElement('option');
        opt.value = key; opt.textContent = name.trim();
        el.insertBefore(opt, el.querySelector('option[value="__new"]'));
        el.value = key; prev = key;
      } catch (er) { toast(errText(er), true); el.value = prev; }
      refresh();
    });
  });
  refresh();
}

// ----- settings -----
async function viewSettings() {
  const L = await api('/lists');
  const s = L.settings;
  const sel = (arr, cur) => arr.map((o) => '<option value="' + o[0] + '"' + (String(o[0]) === String(cur) ? ' selected' : '') + '>' + esc(o[1]) + '</option>').join('');
  const listRows = (items, kind) => items.map((it) => '<div class="setrow"><div class="t">' + (kind === 'card' ? '«' + esc(it.name) + '»' : esc(it.name)) + ' <span>· ' + t('set.uses', it.count) + '</span></div><div><button class="btn link" data-act="ren' + kind + '" data-id="' + it.id + '" data-name="' + esc(it.name) + '">' + t('set.rename') + '</button><button class="btn link danger" data-act="del' + kind + '" data-id="' + it.id + '" data-name="' + esc(it.name) + '" data-n="' + it.count + '">' + t('delete') + '</button></div></div>').join('');
  page('settings',
    '<h1>' + t('nav.settings') + '</h1>' +
    '<div class="glass card"><h2 style="margin-bottom:14px">' + t('set.general') + '</h2><div class="grid2" style="grid-template-columns:repeat(auto-fit,minmax(260px,1fr))">' +
    '<div class="field"><label for="slang">' + t('set.lang') + '</label><select class="sel" id="slang">' + sel([['el', 'Ελληνικά'], ['en', 'English']], s.lang) + '</select></div>' +
    '<div class="field"><label for="scur">' + t('set.currency') + '</label><select class="sel" id="scur">' + sel([['EUR', t('cur.EUR')], ['USD', t('cur.USD')], ['GBP', t('cur.GBP')], ['CHF', t('cur.CHF')]], s.currency) + '</select><div class="hint">' + t('set.currencyHint') + '</div></div></div></div>' +
    '<div class="cols" style="align-items:flex-start"><div class="glass card a" style="flex:1 1 420px"><h2>' + t('set.categories') + '</h2><div class="sub" style="margin:4px 0 12px">' + t('set.categoriesHint') + '</div>' + listRows(L.categories, 'cat') +
    '<div class="addrow"><label for="newcat" class="sr" style="position:absolute;width:1px;height:1px;overflow:hidden">' + t('set.newCat') + '</label><input class="inp" id="newcat" placeholder="' + esc(t('set.newCat')) + '" maxlength="60"><button class="btn" data-act="addcat">' + t('add') + '</button></div><div class="hint" style="margin-top:10px">' + t('set.catDelHint') + '</div></div>' +
    '<div class="glass card b" style="flex:1 1 420px"><h2>' + t('set.cards') + '</h2><div class="sub" style="margin:4px 0 12px">' + t('set.cardsHint') + '</div>' + listRows(L.cards, 'card') +
    '<div class="addrow"><label for="newcard" style="position:absolute;width:1px;height:1px;overflow:hidden">' + t('set.newCard') + '</label><input class="inp" id="newcard" placeholder="' + esc(t('set.newCardPh')) + '" maxlength="60"><button class="btn" data-act="addcard">' + t('add') + '</button></div><div class="hint" style="margin-top:10px">' + t('set.cardWarn') + '</div></div></div>' +
    '<div class="glass card"><h2>' + t('set.methods') + '</h2><div class="sub" style="margin:4px 0 12px">' + t('set.methodsHint') + '</div>' +
    ['card', 'paypal', 'applepay'].map((k) => '<div class="setrow"><div class="t">' + esc(t('method.' + k)) + ' <span>· ' + t('set.builtin') + '</span></div></div>').join('') + listRows(L.methods, 'method') +
    '<div class="addrow"><label for="newmethod" style="position:absolute;width:1px;height:1px;overflow:hidden">' + t('set.newMethod') + '</label><input class="inp" id="newmethod" placeholder="' + esc(t('set.newMethodPh')) + '" maxlength="60"><button class="btn" data-act="addmethod">' + t('add') + '</button></div></div>' +
    '<div class="glass card stack"><div><h2>' + t('set.notif') + '</h2><div class="sub" style="margin-top:4px">' + t('set.notifHint') + '</div></div>' +
    '<div class="grid2" style="grid-template-columns:repeat(auto-fit,minmax(260px,1fr))"><div class="field"><div class="lab"><label for="sdays">' + t('set.days') + '</label>' + tip(t('set.daysTip'), t('set.days')) + '</div><select class="sel" id="sdays">' + sel([[7, t('remind.n', 7)], [3, t('remind.n', 3)], [1, t('remind.1')]], s.remindDays) + '</select></div>' +
    '<div class="field"><div class="lab"><label for="shour">' + t('set.hour') + '</label>' + tip(t('set.hourTip'), t('set.hour')) + '</div><select class="sel" id="shour">' + [9, 8, 12, 18].map((h) => '<option value="' + h + '"' + (s.sendHour === h ? ' selected' : '') + '>' + pad(h) + ':00</option>').join('') + '</select></div></div>' +
    '<div class="stack" style="gap:6px"><div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap"><label class="chk"><input type="checkbox" id="smiss"' + (s.missingBillReminder ? ' checked' : '') + '>' + t('set.missing') + '</label>' + tip(t('set.missingTip'), t('set.missing')) + '</div>' +
    '<div class="grid2 subopts" id="smissbox" style="grid-template-columns:repeat(auto-fit,minmax(240px,1fr))"><div class="field"><div class="lab"><label for="srdays">' + t('set.repeatDays') + '</label>' + tip(t('set.repeatDaysTip'), t('set.repeatDays')) + '</div><select class="sel" id="srdays">' + sel([[1, t('set.rd.1')], [2, t('set.rd.n', 2)], [3, t('set.rd.n', 3)], [5, t('set.rd.n', 5)], [7, t('set.rd.n', 7)]], s.missingRepeatDays || 3) + '</select></div>' +
    '<div class="field"><div class="lab"><label for="srmax">' + t('set.repeatMax') + '</label>' + tip(t('set.repeatMaxTip'), t('set.repeatMax')) + '</div><select class="sel" id="srmax">' + sel([[1, '1'], [2, '2'], [3, '3'], [5, '5']], s.missingRepeatMax || 3) + '</select></div></div></div>' +
    '<div style="display:flex;flex-wrap:wrap;gap:16px;align-items:flex-start">' + (EMAIL_UI ? '<div class="chan"><label class="chk" style="font-weight:600;font-size:15px"><input type="checkbox" id="semail"' + (s.emailEnabled ? ' checked' : '') + '>' + t('notif.email') + '</label><div class="hint">' + t('set.emailHint') + '</div></div>' : '') +
    '<div class="chan"><label class="chk" style="font-weight:600;font-size:15px"><input type="checkbox" id="spush"' + (s.pushEnabled ? ' checked' : '') + '>' + t('set.pushover') + '</label>' +
    '<div class="field"><label for="puser">' + t('set.puser') + '</label><input class="inp" id="puser" autocomplete="off" placeholder="' + esc(t('set.puserPh')) + '" value="' + esc(s.pushoverUser) + '"></div>' +
    '<div class="field"><label for="ptoken">' + t('set.ptoken') + '</label><input class="inp" id="ptoken" type="password" autocomplete="off" placeholder="' + esc(s.hasToken ? t('set.ptokenSaved') : t('set.ptokenPh')) + '"></div>' +
    (s.hasToken ? '<label class="chk"><input type="checkbox" id="pclear">' + t('set.ptokenClear') + '</label>' : '') + '<div class="hint">' + t('set.pushHint') + '</div></div></div>' +
    '<div style="display:flex;flex-wrap:wrap;gap:10px;align-items:center"><button class="btn" data-act="testnotify">' + t('set.test') + '</button><span id="testres" class="sub"></span></div></div>' +
    '<div class="glass card stack"><div><h2>' + t('set.security') + '</h2><div class="sub" style="margin-top:4px">' + t('set.securityHint') + '</div></div>' +
    (AUTH_UI ? '<div class="grid2" style="grid-template-columns:repeat(auto-fit,minmax(260px,1fr))"><div class="field"><label for="sauth">' + t('set.auth') + '</label><select class="sel" id="sauth">' + sel([['off', t('set.auth.off')], ['dsm', t('set.auth.dsm')]], s.authMode) + '</select></div>' +
    '<div class="field"><div class="lab">' + t('set.authCheck') + '</div><div style="display:flex;gap:10px;align-items:center;flex-wrap:wrap"><button class="btn" data-act="authprobe">' + t('set.authProbe') + '</button><span id="authres" class="sub"></span></div></div></div>'  : '') +
    '<div style="display:flex;flex-wrap:wrap;gap:10px;align-items:center"><a class="btn" href="/api/backup">' + t('set.backup') + '</a><button class="btn" data-act="restore">' + t('set.restore') + '</button><input type="file" id="rfile" accept=".json,application/json" style="display:none"><span class="hint">' + t('set.backupHint') + '</span></div><div class="note"><i></i>' + t('set.backupWarn') + '</div></div>' +
    '<div style="display:flex;flex-wrap:wrap;justify-content:space-between;align-items:center;gap:10px"><span class="sub"><span id="ver"></span> · <button class="btn link" data-act="notes">' + t('notes.all') + '</button></span><div style="display:flex;gap:10px"><a class="btn" href="#/">' + t('cancel') + '</a><button class="btn primary" data-act="savesettings">' + t('save') + '</button></div></div>');
  const syncMiss = () => {
    const on = $('#smiss').checked;
    $('#smissbox').classList.toggle('off', !on);
    $('#srdays').disabled = !on; $('#srmax').disabled = !on;
  };
  $('#smiss').addEventListener('change', syncMiss);
  syncMiss();
  api('/version').then((v) => { const e = $('#ver'); if (e) e.textContent = 'Cyklo ' + v.version; }).catch(() => {});
  $('#rfile').addEventListener('change', async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    if (!(await confirmBox(t('set.restore'), t('set.restoreWarn'), t('set.restore'), true))) return;
    try {
      const txt = await file.text();
      const r = await fetch('/api/restore', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: txt });
      if (!r.ok) throw new Error('backup');
      toast(t('saved')); await loadSettings(); render();
    } catch (er) { toast(errText(er), true); }
  });
}

async function saveSettings() {
  const body = {
    lang: $('#slang').value, currency: $('#scur').value, remindDays: Number($('#sdays').value), sendHour: Number($('#shour').value),
    missingBillReminder: $('#smiss').checked, missingRepeatDays: Number($('#srdays').value), missingRepeatMax: Number($('#srmax').value), emailEnabled: EMAIL_UI ? $('#semail').checked : false, pushEnabled: $('#spush').checked,
    pushoverUser: $('#puser').value, pushoverToken: $('#ptoken').value, clearToken: $('#pclear') ? $('#pclear').checked : false,
    onboardingDone: true, authMode: AUTH_UI ? $('#sauth').value : 'off',
  };
  try {
    await api('/settings', { method: 'PUT', body });
    await loadSettings();
    toast(t('saved'));
    render();
  } catch (e) { toast(errText(e), true); }
}

// ----- onboarding -----
const onb = { step: 1, lang: 'el', currency: 'EUR', days: 7, email: false, push: false };

function viewOnboarding() {
  const names = [t('onb.lang'), t('onb.cur'), t('onb.notif')];
  const steps = names.map((n, i) => {
    const k = i + 1;
    return '<span class="step ' + (onb.step > k ? 'done' : onb.step === k ? 'cur' : '') + '"><b>' + (onb.step > k ? '✓' : k) + '</b>' + n + '</span>';
  }).join('');
  let body = '';
  if (onb.step === 1) body = '<div><div style="font-size:18px;font-weight:600">' + t('onb.langTitle') + '</div><div class="sub" style="margin-top:4px">' + t('onb.langHint') + '</div></div><div class="bigpick"><button data-act="onblang" data-v="el" class="' + (onb.lang === 'el' ? 'on' : '') + '">Ελληνικά</button><button data-act="onblang" data-v="en" class="' + (onb.lang === 'en' ? 'on' : '') + '">English</button></div>';
  if (onb.step === 2) body = '<div><div style="font-size:18px;font-weight:600">' + t('onb.curTitle') + '</div><div class="sub" style="margin-top:4px">' + t('onb.curHint') + '</div></div><div class="field"><label for="ocur">' + t('set.currency') + '</label><select class="sel" id="ocur">' + ['EUR', 'USD', 'GBP', 'CHF'].map((c) => '<option value="' + c + '"' + (onb.currency === c ? ' selected' : '') + '>' + t('cur.' + c) + '</option>').join('') + '</select><div class="hint">' + t('set.currencyHint') + '</div></div>';
  if (onb.step === 3) body = '<div><div style="font-size:18px;font-weight:600">' + t('onb.notifTitle') + ' <span class="sub" style="font-weight:400">(' + t('onb.optional') + ')</span></div><div class="sub" style="margin-top:4px">' + t('onb.notifHint') + '</div></div>' +
    '<div class="field"><label for="odays">' + t('onb.days') + '</label><select class="sel" id="odays">' + [[7, t('remind.n', 7)], [3, t('remind.n', 3)], [1, t('remind.1')]].map((o) => '<option value="' + o[0] + '"' + (onb.days === o[0] ? ' selected' : '') + '>' + o[1] + '</option>').join('') + '</select></div>' +
    (EMAIL_UI ? '<label class="chk"><input type="checkbox" id="oemail"' + (onb.email ? ' checked' : '') + '>' + t('onb.email') + '</label><div class="hint" style="margin-top:-8px">' + t('set.emailHint') + '</div>' : '') +
    '<label class="chk"><input type="checkbox" id="opush"' + (onb.push ? ' checked' : '') + '>' + t('onb.push') + '</label>';
  if (onb.step === 4) body = '<div><div style="font-size:18px;font-weight:600">' + t('onb.doneTitle') + '</div><div class="sub" style="margin-top:4px">' + t('onb.doneHint') + '</div></div><div style="display:flex;flex-wrap:wrap;gap:10px"><a class="btn primary" href="#/new">' + t('onb.first') + '</a><a class="btn" href="#/">' + t('onb.home') + '</a></div>';
  $('#app').innerHTML = '<div class="onb"><div class="glass box"><div><div class="sub">Cyklo</div><h1 style="font-size:26px">' + t('onb.title') + '</h1></div>' +
    (onb.step < 4 ? '<div class="steps">' + steps + '</div>' : '') + '<div class="stack">' + body + '</div>' +
    (onb.step < 4 ? '<div style="display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:10px"><div>' + (onb.step > 1 ? '<button class="btn" data-act="onbback">' + t('back') + '</button>' : '') + '</div><div style="display:flex;gap:10px">' + (onb.step === 3 ? '<button class="btn link" data-act="onbskip">' + t('skip') + '</button>' : '') + '<button class="btn primary" data-act="onbnext">' + (onb.step === 3 ? t('finish') : t('continue')) + '</button></div></div>' : '') + '</div></div>';
}

async function onbFinish(skip) {
  const body = {
    lang: onb.lang, currency: onb.currency, remindDays: skip ? 7 : onb.days, sendHour: 9, missingBillReminder: true, missingRepeatDays: 3, missingRepeatMax: 3,
    emailEnabled: skip ? false : onb.email, pushEnabled: skip ? false : onb.push, pushoverUser: '', pushoverToken: '', clearToken: false,
    onboardingDone: true, authMode: settings.authMode,
  };
  try {
    await api('/settings', { method: 'PUT', body });
    await loadSettings();
    onb.step = 4;
    viewOnboarding();
  } catch (e) { toast(errText(e), true); }
}

function showLogin() {
  const host = location.protocol + '//' + location.hostname + ':5000/';
  $('#app').innerHTML = '<div class="onb"><div class="glass box"><h1 style="font-size:24px">' + t('login.title') + '</h1><div class="sub">' + t('login.text') + '</div><div style="display:flex;gap:10px;flex-wrap:wrap"><a class="btn primary" href="' + host + '" target="_blank" rel="noopener">' + t('login.open') + '</a><button class="btn" onclick="location.reload()">' + t('login.retry') + '</button></div></div></div>';
}

// ---------- easter egg ----------
function confetti(canvas, stop) {
  const ctx = canvas.getContext('2d');
  const W = canvas.width = window.innerWidth, H = canvas.height = window.innerHeight;
  const colors = ['#ff4d6d', '#ffb703', '#2ec4b6', '#4d6fe8', '#9b5de5', '#f15bb5', '#80ed99'];
  const parts = [];
  const burst = (x, y, n) => {
    for (let i = 0; i < n; i++) {
      const a = Math.random() * Math.PI * 2, v = 5 + Math.random() * 11;
      const coin = Math.random() < 0.25;
      parts.push({ x, y, vx: Math.cos(a) * v, vy: Math.sin(a) * v - 6, w: coin ? 11 : 6 + Math.random() * 6, h: coin ? 11 : 4 + Math.random() * 5,
        r: Math.random() * 6.28, vr: (Math.random() - 0.5) * 0.4, c: coin ? '#f6c945' : colors[Math.floor(Math.random() * colors.length)], coin, ph: Math.random() * 6.28, life: 0 });
    }
  };
  burst(W * 0.5, H * 0.35, 90);
  setTimeout(() => burst(W * 0.2, H * 0.45, 60), 350);
  setTimeout(() => burst(W * 0.8, H * 0.45, 60), 650);
  setTimeout(() => burst(W * 0.5, H * 0.25, 80), 1100);
  const t0 = performance.now();
  const tick = (now) => {
    if (stop.done) { canvas.remove(); return; }
    ctx.clearRect(0, 0, W, H);
    parts.forEach((p) => {
      p.vy += 0.28; p.vx *= 0.992; p.x += p.vx; p.y += p.vy; p.r += p.vr; p.life++;
      ctx.save(); ctx.translate(p.x, p.y); ctx.rotate(p.r); ctx.fillStyle = p.c;
      if (p.coin) { ctx.beginPath(); ctx.ellipse(0, 0, p.w / 2, Math.abs(Math.cos(p.ph + p.life * 0.15)) * p.w / 2 + 1, 0, 0, 6.28); ctx.fill(); ctx.strokeStyle = '#b8860b'; ctx.stroke(); }
      else ctx.fillRect(-p.w / 2, -p.h / 2, p.w, p.h);
      ctx.restore();
    });
    if (now - t0 < 5500 && parts.some((p) => p.y < H + 40)) requestAnimationFrame(tick); else canvas.remove();
  };
  requestAnimationFrame(tick);
}

function showEgg() {
  const reduce = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const stop = { done: false };
  const m = document.createElement('div');
  m.className = 'modal eggmodal';
  m.innerHTML = '<div class="box eggbox" role="dialog" aria-modal="true" aria-label="' + esc(t('egg.title')) + '">' +
    '<div class="egghead"><span>👑</span><span>💎</span><span>✨</span></div><div class="eggstripe"><i></i><i></i><i></i><i></i><i></i></div>' +
    '<div class="eggbody"><h3>' + esc(t('egg.title')) + '</h3><div class="sub">' + esc(t('egg.sub')) + '</div>' +
    '<div class="eggrow"><span>' + esc(t('egg.wealth')) + '</span><b id="eggw">' + SYMBOL[settings.currency] + ' 0</b></div>' +
    '<div class="eggrow"><span>' + esc(t('egg.happy')) + '</span><b><span id="eggm">😐</span> <span id="eggp">0%</span></b></div>' +
    '<div class="eggmeter"><i id="eggbar"></i></div>' +
    '<div class="hint">' + esc(t('egg.note')) + '</div>' +
    '<div class="acts"><button class="btn primary" data-i="0">' + esc(t('notes.close')) + '</button></div></div></div>';
  document.body.appendChild(m);
  const close = () => { stop.done = true; m.remove(); document.removeEventListener('keydown', onKey); };
  const onKey = (e) => { if (e.key === 'Escape') close(); };
  document.addEventListener('keydown', onKey);
  m.addEventListener('click', (e) => { if (e.target === m || e.target.closest('button[data-i]')) close(); });
  const target = 1000000000;
  const nf = new Intl.NumberFormat(lang === 'el' ? 'el-GR' : 'en-GB');
  const moods = ['😐', '🙂', '😀', '🤩', '🥳'];
  const paint = (k) => {
    const pct = Math.round(k * 100);
    $('#eggw', m).textContent = SYMBOL[settings.currency] + ' ' + nf.format(Math.round(target * k));
    $('#eggp', m).textContent = pct + '%';
    $('#eggbar', m).style.width = pct + '%';
    $('#eggm', m).textContent = moods[Math.min(moods.length - 1, Math.floor(k * moods.length))];
  };
  if (reduce) { paint(1); return; }
  const cv = document.createElement('canvas');
  cv.className = 'confetti';
  document.body.appendChild(cv);
  confetti(cv, stop);
  const t0 = performance.now(), dur = 3000;
  const step = (now) => {
    if (stop.done) return;
    const k = Math.min(1, (now - t0) / dur);
    paint(1 - Math.pow(1 - k, 3));
    if (k < 1) requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
}

// ---------- release notes ----------
function notesHtml(list) {
  return list.map((c) => '<div style="margin-bottom:12px"><b>' + esc(c.v) + '</b><ul style="margin:4px 0 0;padding-left:20px">' + c[lang].map((x) => '<li>' + esc(x) + '</li>').join('') + '</ul></div>').join('');
}
async function showNotes(all) {
  await modal({ title: t(all ? 'notes.all' : 'notes.title'), body: '<div style="max-height:50vh;overflow:auto;font-size:14px">' + notesHtml(all ? CHANGELOG : CHANGELOG.slice(0, 1)) + '</div>', actions: [{ label: t('notes.close'), cls: 'primary', value: true }] });
}
async function checkNotes() {
  try {
    const v = (await api('/version')).version;
    let seen = null;
    try { seen = localStorage.getItem('cyklo.seenVersion'); } catch (e) { /* storage unavailable */ }
    if (seen === v) return;
    try { localStorage.setItem('cyklo.seenVersion', v); } catch (e) { /* storage unavailable */ }
    if (seen !== null && CHANGELOG[0].v === v) await showNotes(false);
  } catch (e) { /* ignore */ }
}

// ---------- router ----------
async function loadSettings() {
  settings = await api('/settings');
  lang = settings.lang || 'el';
  document.documentElement.lang = lang;
  document.title = 'Cyklo';
}

async function render() {
  const h = location.hash.replace(/^#/, '') || '/';
  try {
    if (!settings) await loadSettings();
    if (!/^\/(edit\/\d+|new)$/.test(h)) ui.returnTo = '';
    if (!settings.onboardingDone && h !== '/onboarding') { onb.lang = settings.lang; onb.currency = settings.currency; viewOnboarding(); return; }
    if (h === '/' ) return await viewHome();
    if (h === '/subs') return await viewSubs();
    if (h === '/reports') return await viewReports();
    if (h === '/calendar') return await viewCalendar();
    if (h === '/settings') return await viewSettings();
    if (h === '/new') return await viewForm(0);
    const m = h.match(/^\/edit\/(\d+)$/);
    if (m) return await viewForm(Number(m[1]));
    location.hash = '#/';
  } catch (e) {
    if (e.message !== 'login') $('#app').innerHTML = '<div class="glass card empty"><b>' + t('err.generic') + '</b><span>' + esc(e.message) + '</span></div>';
  }
}

document.addEventListener('click', async (e) => {
  const go = e.target.closest('[data-go]');
  if (go && !e.target.closest('button, a, input, select')) {
    ui.returnTo = location.hash || '#/';
    location.hash = go.dataset.go;
    return;
  }
  const b = e.target.closest('[data-act]');
  if (!b || b.closest('#form')) return;
  const act = b.dataset.act, v = b.dataset.v;
  try {
    if (act === 'lang') {
      await api('/settings', { method: 'PUT', body: { ...settings, lang: v, pushoverToken: '', clearToken: false } });
      await loadSettings(); render();
    }
    else if (act === 'tileactive') { ui.subs.status = 'active'; ui.subs.cat = ''; ui.subs.q = ''; location.hash = '#/subs'; }
    else if (act === 'egg') showEgg();
    else if (act === 'substab') { ui.subs.status = v; render(); }
    else if (act === 'record') recordFlow(Number(b.dataset.id), render);
    else if (act === 'repview') { ui.rep.view = v; ui.rep.open = null; render(); }
    else if (act === 'repopen') { ui.rep.open = ui.rep.open === b.dataset.k ? null : b.dataset.k; render(); }
    else if (act === 'calsel') { ui.cal.sel = b.dataset.d; render(); }
    else if (act === 'calmove') {
      const [y, m] = ui.cal.month.split('-').map(Number);
      const d = new Date(Date.UTC(y, m - 1 + Number(v), 1));
      ui.cal.month = d.getUTCFullYear() + '-' + pad(d.getUTCMonth() + 1); ui.cal.sel = null; render();
    }
    else if (act === 'caltoday') { ui.cal.month = null; ui.cal.sel = null; render(); }
    else if (act === 'onblang') { onb.lang = v; lang = v; viewOnboarding(); }
    else if (act === 'onbback') { onb.step--; viewOnboarding(); }
    else if (act === 'onbskip') { await onbFinish(true); }
    else if (act === 'onbnext') {
      if (onb.step === 2) onb.currency = $('#ocur').value;
      if (onb.step === 3) { onb.days = Number($('#odays').value); onb.email = EMAIL_UI ? $('#oemail').checked : false; onb.push = $('#opush').checked; await onbFinish(false); return; }
      onb.step++; viewOnboarding();
    }
    else if (act === 'savesettings') await saveSettings();
    else if (act === 'addcat' || act === 'addcard' || act === 'addmethod') {
      const inp = $(act === 'addcat' ? '#newcat' : act === 'addcard' ? '#newcard' : '#newmethod');
      await api(act === 'addcat' ? '/categories' : act === 'addcard' ? '/cards' : '/methods', { method: 'POST', body: { name: inp.value } });
      render();
    }
    else if (act === 'delmethod') {
      const id = b.dataset.id, key = 'c' + id;
      if (Number(b.dataset.n) > 0) {
        const lists = await api('/lists');
        const opts = [['card', t('method.card')], ['paypal', t('method.paypal')], ['applepay', t('method.applepay')]].concat(lists.methods.filter((m) => String(m.id) !== id).map((m) => ['c' + m.id, m.name]));
        const res = await modal({ title: t('set.delMethodTitle'), body: '<div class="sub">' + esc(t('set.delMethodMove', b.dataset.n)) + '</div><div class="field"><select class="sel" id="mv">' + opts.map((o) => '<option value="' + o[0] + '">' + esc(o[1]) + '</option>').join('') + '</select></div>', actions: [{ label: t('cancel'), value: null }, { label: t('delete'), cls: 'primary', value: (m) => $('#mv', m).value }] });
        if (res) { await api('/methods/' + id + '?moveTo=' + res, { method: 'DELETE' }); render(); }
      } else if (await confirmBox(t('set.delMethodTitle'), t('set.delMethodText', b.dataset.name), t('delete'), true)) { await api('/methods/' + id, { method: 'DELETE' }); render(); }
    }
    else if (act === 'rencat' || act === 'rencard' || act === 'renmethod') {
      const kind = act === 'rencat' ? 'categories' : act === 'rencard' ? 'cards' : 'methods';
      const res = await modal({ title: t('set.rename'), body: '<div class="field"><input class="inp" id="rn" maxlength="60" value="' + esc(b.dataset.name) + '"></div>', actions: [{ label: t('cancel'), value: null }, { label: t('save'), cls: 'primary', value: (m) => $('#rn', m).value }] });
      if (res !== null) { await api('/' + kind + '/' + b.dataset.id, { method: 'PUT', body: { name: res } }); render(); }
    }
    else if (act === 'delcard') {
      if (await confirmBox(t('set.delCardTitle'), t('set.delCardText', b.dataset.name), t('delete'), true)) { await api('/cards/' + b.dataset.id, { method: 'DELETE' }); render(); }
    }
    else if (act === 'delcat') {
      const lists = await api('/lists');
      const id = b.dataset.id;
      if (Number(b.dataset.n) > 0) {
        const others = lists.categories.filter((c) => String(c.id) !== id);
        const res = await modal({ title: t('set.delCatTitle'), body: '<div class="sub">' + esc(t('set.delCatMove', b.dataset.n)) + '</div><div class="field"><select class="sel" id="mv">' + others.map((c) => '<option value="' + c.id + '">' + esc(c.name) + '</option>').join('') + '</select></div>', actions: [{ label: t('cancel'), value: null }, { label: t('delete'), cls: 'primary', value: (m) => $('#mv', m).value }] });
        if (res) { await api('/categories/' + id + '?moveTo=' + res, { method: 'DELETE' }); render(); }
      } else if (await confirmBox(t('set.delCatTitle'), t('set.delCatText', b.dataset.name), t('delete'), true)) { await api('/categories/' + id, { method: 'DELETE' }); render(); }
    }
    else if (act === 'testnotify') {
      $('#testres').textContent = '…';
      const r = await api('/notify/test', { method: 'POST' });
      const line = (name, c) => !c.enabled ? name + ': ' + t('set.testOff') : c.ok ? name + ': ' + t('set.testOk') : name + ': ' + t('set.testFail') + (c.error ? ' (' + c.error + ')' : '');
      $('#testres').textContent = (EMAIL_UI ? line(t('notif.email'), r.email) + ' · ' : '') + line('Pushover', r.push);
    }
    else if (act === 'authprobe') {
      const r = await api('/auth/check?probe=1');
      $('#authres').textContent = r.ok && r.user ? t('set.authOk', r.user) : t('set.authFail');
    }
    else if (act === 'restore') $('#rfile').click();
    else if (act === 'notes') await showNotes(true);
  } catch (er) { if (er.message !== 'login') toast(errText(er), true); }
});

document.addEventListener('keydown', (e) => {
  if ((e.key === 'Enter' || e.key === ' ') && e.target.classList && e.target.classList.contains('go')) { e.preventDefault(); e.target.click(); }
});

window.addEventListener('hashchange', render);
render().then(checkNotes);
})();
