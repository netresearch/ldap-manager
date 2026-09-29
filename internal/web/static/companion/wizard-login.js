import './login-companion.js';

const avatar = document.querySelector('login-companion[character="wizard"]');
const form = avatar?.closest('form');
let pending = false;
let releasing = false;
let timer;

form?.addEventListener('submit', event => {
  if (releasing || event.defaultPrevented) return;
  if (pending) {
    event.preventDefault();
    return;
  }
  // Reduced motion and failed WebGL keep the ordinary immediate form POST.
  if (matchMedia('(prefers-reduced-motion: reduce)').matches || avatar.dataset.renderer !== 'webgl') return;
  if (!avatar.performAction()) return;
  event.preventDefault();
  pending = true;
  const submitter = event.submitter;
  form.setAttribute('aria-busy', 'true');
  // Follow animation time so slow frames do not cut off the staff strike.
  // A stalled, hidden or failed renderer must never block authentication.
  const deadline = performance.now() + 4000;
  const release = () => {
    if (avatar.isConnected && avatar.dataset.renderer === 'webgl' &&
        !document.hidden && !avatar.paused &&
        !matchMedia('(prefers-reduced-motion: reduce)').matches &&
        avatar.actionElapsed < 1.4 && performance.now() < deadline) {
      timer = setTimeout(release, 50);
      return;
    }
    pending = false;
    releasing = true;
    form.removeAttribute('aria-busy');
    try {
      if (submitter && submitter.form === form) form.requestSubmit(submitter);
      else form.requestSubmit();
    } finally {
      releasing = false;
    }
  };
  timer = setTimeout(release, 50);
});

window.addEventListener('pagehide', () => clearTimeout(timer));
window.addEventListener('pageshow', () => {
  pending = false;
  releasing = false;
  form?.removeAttribute('aria-busy');
});
