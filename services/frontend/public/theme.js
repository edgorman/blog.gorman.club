// Applies the stored colour scheme before first paint, so a visitor who pinned one never sees a
// flash of the other. With nothing stored, <meta name="color-scheme"> stays "light dark" and the
// page follows the OS.
//
// It sits in its own file rather than inline in index.html so the Content-Security-Policy in
// public/_headers can refuse inline scripts outright - which is what stops an `onerror=` smuggled
// into a post from ever running. The alternative, a hash of the inline block, would have to be
// recomputed in a second file every time this one is touched.
//
// It is a classic (non-module, non-deferred) script so it still runs before the body is parsed.
(function () {
  try {
    var theme = localStorage.getItem('gc-theme');
    if (theme === 'light' || theme === 'dark') {
      document.querySelector('meta[name="color-scheme"]').content = theme;
    }
  } catch (e) {
    // Storage can be unavailable entirely (e.g. Safari private mode); the OS default stands.
  }
})();
