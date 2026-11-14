/* SPDX-License-Identifier: Apache-2.0 */
'use strict';
// Public marketing site only. Never connect to localhost, VM APIs or billing.
document.addEventListener('click', event => {
  const link = event.target.closest?.('a');
  if (!link) return;
  const url = new URL(link.href);
  if (url.origin !== location.origin) return;
  const label = link.textContent.trim();
  if (label === 'Get started' || label === 'Open local console' || url.pathname === '/dashboard') {
    event.preventDefault(); event.stopImmediatePropagation(); location.assign('/get-started/');
  } else if (url.pathname === '/pricing') {
    event.preventDefault(); event.stopImmediatePropagation(); location.assign('/pricing');
  }
}, true);
