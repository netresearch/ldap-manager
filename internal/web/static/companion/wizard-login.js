import './scormiq-avatar.js';

const avatar = document.querySelector('scormiq-avatar[character="wizard"]');
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
  // Let the staff strike land before the browser replaces the login page.
  timer = setTimeout(() => {
    pending = false;
    releasing = true;
    form.removeAttribute('aria-busy');
    try {
      if (submitter && submitter.form === form) form.requestSubmit(submitter);
      else form.requestSubmit();
    } finally {
      releasing = false;
    }
  }, 1400);
});

window.addEventListener('pagehide', () => clearTimeout(timer));
window.addEventListener('pageshow', () => {
  pending = false;
  releasing = false;
  form?.removeAttribute('aria-busy');
});
