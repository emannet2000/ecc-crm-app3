'use strict';
(() => {
  // Annotate existing cells rather than copying records or changing Elm's DOM children.
  function prepareTables(root) {
    for (const table of root.querySelectorAll('table')) {
      const headerRows = table.querySelectorAll('thead > tr');
      const rows = [...(table.querySelector('tbody')?.children || [])];
      const headers = headerRows.length === 1 ? [...headerRows[0].children] : [];
      const simple = headers.length > 0 && [...headers, ...rows.flatMap(row => [...row.children])].every(cell => Number(cell.getAttribute('colspan') || 1) === 1 && Number(cell.getAttribute('rowspan') || 1) === 1) && rows.every(row => row.children.length === headers.length);
      table.toggleAttribute('data-mobile-cards', simple);
      table.toggleAttribute('data-mobile-controls', simple && headers.some(header => header.querySelector('button,input,select,a[href]') || header.tabIndex >= 0));
      if (!simple) continue;
      if (!table.hasAttribute('role')) table.setAttribute('role', 'table');
      for (const group of table.querySelectorAll('thead,tbody')) if (!group.hasAttribute('role')) group.setAttribute('role', 'rowgroup');
      for (const row of [headerRows[0], ...rows]) if (!row.hasAttribute('role')) row.setAttribute('role', 'row');
      headers.forEach((header, index) => {
        header.toggleAttribute('data-mobile-control', !!header.querySelector('button,input,select,a[href]') || header.tabIndex >= 0);
        if (!header.hasAttribute('role')) header.setAttribute('role', 'columnheader');
        const label = header.textContent.replace(/[↑↓]/g, '').trim() || (header.querySelector('input[type=checkbox]') ? 'Select' : 'Actions');
        for (const row of rows) {
          const cell = row.children[index];
          if (!cell.hasAttribute('role')) cell.setAttribute('role', 'cell');
          if (cell.getAttribute('data-label') !== label) cell.setAttribute('data-label', label);
          cell.toggleAttribute('data-mobile-actions', label === 'Actions' || label === 'Select');
        }
      });
    }
  }

  const mobile = window.matchMedia('(max-width: 900px)');
  const watched = new WeakSet();
  let pending = false, drawer = null, returnFocus = null, navigated = false;
  const inert = (element, value) => element?.toggleAttribute('inert', value);
  const focusable = root => [...root.querySelectorAll('a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex="0"]')].filter(element => !element.closest('[inert]') && !element.hidden && element.getClientRects().length > 0);

  function syncNavigation() {
    const sidebar = document.querySelector('.sidebar');
    const open = !!(mobile.matches && sidebar?.classList.contains('is-open'));
    const dialog = document.querySelector('[role=dialog],.modal,.ecc-modal,.modal-panel');
    inert(sidebar, mobile.matches && !open);
    if (sidebar) {
      if (mobile.matches && !open) sidebar.setAttribute('aria-hidden', 'true');
      else sidebar.removeAttribute('aria-hidden');
    }
    inert(document.querySelector('.app-main'), open);
    inert(document.querySelector('.mobile-nav'), open || !!dialog);
    inert(document.querySelector('.topbar'), !!dialog);
    document.documentElement.classList.toggle('crm-scroll-locked', open || !!dialog);
    if (open && drawer !== sidebar) {
      returnFocus = document.activeElement;
      drawer = sidebar;
      requestAnimationFrame(() => { if (drawer === sidebar) (sidebar.querySelector('.sidebar__close') || focusable(sidebar)[0])?.focus(); });
    } else if (open && !sidebar.contains(document.activeElement)) {
      (sidebar.querySelector('.sidebar__close') || focusable(sidebar)[0])?.focus();
    } else if (!open && drawer) {
      drawer = null;
      if (navigated) {
        const main = document.querySelector('main.content');
        if (main) { main.setAttribute('tabindex', '-1'); main.focus({preventScroll:true}); window.scrollTo({top:0,behavior:'auto'}); }
      } else if (returnFocus?.isConnected) returnFocus.focus({preventScroll:true});
      navigated = false; returnFocus = null;
    }
  }

  function refresh() {
    pending = false;
    prepareTables(document);
    for (const host of document.querySelectorAll('crm-workspace,crm-record-tools,crm-insights,crm-administration,crm-student-journey,crm-my-work,crm-recent-changes')) {
      const root = host.shadowRoot;
      if (!root) continue;
      if (!watched.has(root)) {
        watched.add(root);
        new MutationObserver(schedule).observe(root, {childList:true,subtree:true,characterData:true});
      }
      prepareTables(root);
    }
    syncNavigation();
  }
  function schedule() { if (!pending) { pending = true; requestAnimationFrame(refresh); } }
  new MutationObserver(schedule).observe(document.body, {childList:true,subtree:true,characterData:true,attributes:true,attributeFilter:['class']});
  mobile.addEventListener('change', schedule);
  document.addEventListener('click', event => {
    if (drawer && event.target.closest('.sidebar .nav-item')) navigated = true;
  }, true);
  document.addEventListener('focusin', event => {
    if (mobile.matches && drawer?.classList.contains('is-open') && !drawer.contains(event.target)) {
      (drawer.querySelector('.sidebar__close') || focusable(drawer)[0])?.focus();
    }
  }, true);
  document.addEventListener('keydown', event => {
    if (!drawer || event.key !== 'Tab') return;
    const items = focusable(drawer), first = items[0], last = items.at(-1);
    if (!first) return;
    if (event.shiftKey && (document.activeElement === first || !drawer.contains(document.activeElement))) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && (document.activeElement === last || !drawer.contains(document.activeElement))) { event.preventDefault(); first.focus(); }
  });

  class ConnectionStatus extends HTMLElement {
    connectedCallback() {
      this.setAttribute('role', 'status'); this.setAttribute('aria-live', 'polite');
      this.update = () => {
        const offline = navigator.onLine === false;
        this.toggleAttribute('data-offline', offline);
        const message = offline ? 'You’re offline. Reconnect before saving changes.' : '';
        if (this.textContent !== message) this.textContent = message;
      };
      window.addEventListener('online', this.update); window.addEventListener('offline', this.update); this.update();
    }
    disconnectedCallback() { window.removeEventListener('online', this.update); window.removeEventListener('offline', this.update); }
  }
  customElements.define('crm-connection-status', ConnectionStatus);

  const installButton = document.getElementById('install-app');
  let installPrompt = null;
  const installed = window.matchMedia('(display-mode: standalone)').matches || window.navigator.standalone === true;
  if (installed) document.documentElement.classList.add('pwa-installed');
  if (installButton && !installed) {
    window.addEventListener('beforeinstallprompt', event => {
      event.preventDefault();
      installPrompt = event;
      installButton.hidden = false;
    });
    installButton.addEventListener('click', async () => {
      if (!installPrompt) return;
      installButton.disabled = true;
      try {
        await installPrompt.prompt();
        await installPrompt.userChoice;
      } catch (error) {
        console.warn('PWA installation was cancelled or unavailable', error);
      } finally {
        installPrompt = null;
        installButton.hidden = true;
        installButton.disabled = false;
      }
    });
    window.addEventListener('appinstalled', () => {
      installPrompt = null;
      installButton.hidden = true;
      document.documentElement.classList.add('pwa-installed');
    });
  }
  if ('serviceWorker' in navigator && window.isSecureContext) {
    window.addEventListener('load', () => navigator.serviceWorker.register('/sw.js').catch(error => console.warn('PWA shell caching is unavailable', error)));
  }
  refresh();
})();
