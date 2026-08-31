// Applies the stored theme before first paint, so a dark-mode visitor never sees a light flash.
//
// It sits in its own file rather than inline in index.html so the Content-Security-Policy in
// public/_headers can refuse inline scripts outright - which is what stops an `onerror=` smuggled
// into a post from ever running. The alternative, a hash of the inline block, would have to be
// recomputed in a second file every time this one is touched.
//
// It is a classic (non-module, non-deferred) script so it still runs before the body is parsed.
(function () {
  try {
    if (localStorage.getItem('gc-theme') === 'dark') {
      document.documentElement.setAttribute('data-theme', 'dark');
    }
  } catch (e) {
    // Storage can be unavailable entirely (e.g. Safari private mode); the light default stands.
  }
})();
