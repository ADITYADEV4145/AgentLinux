/* SPDX-License-Identifier: Apache-2.0 */
'use strict';
(() => {
  const match = location.hash.slice(1).match(/^([a-f0-9]{64})(?::([a-z-]+))?$/);
  const capability = match ? match[1] : '';
  if (match) history.replaceState(null, '', location.pathname + (match[2] ? '#' + match[2] : ''));
  const dashboard = '/dashboard' + (capability ? '#' + capability : '');
  document.addEventListener('click', e => {
    const a = e.target.closest?.('a');
    if (!a) return;
    const label = a.textContent.trim();
    const url = new URL(a.href);
    if (label === 'Open local console' || label === 'Explore cloud plans') {
      e.preventDefault(); e.stopImmediatePropagation(); location.assign(dashboard);
    } else if (new URL(a.href).pathname === '/pricing' && new URL(a.href).origin === location.origin) {
      e.preventDefault(); e.stopImmediatePropagation(); location.assign('/pricing' + (capability ? '#' + capability : ''));
    } else if (capability && url.origin === location.origin && url.pathname === '/' && location.pathname !== '/') {
      e.preventDefault(); e.stopImmediatePropagation(); location.assign('/#' + capability + (url.hash ? ':' + url.hash.slice(1) : ''));
    }
  }, true);
})();
