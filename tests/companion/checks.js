const ASSETS = document.querySelector('meta[name=companion-assets]').content;
const THREE = await import(ASSETS + '/vendor/three.module.js');
const kind = 'wizard';
const results = document.getElementById('results');
const check = (condition, message) => {
  const row = document.createElement('li');
  row.textContent = `${condition ? 'PASS' : 'FAIL'}: ${message}`;
  results.append(row);
  if (!condition) throw new Error(message);
};
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
// Poll an observable result, with a deadline that fails instead of silently continuing.
const waitFor = async (condition, description, timeout = 20000) => {
  const deadline = performance.now() + timeout;
  while (!condition()) {
    if (performance.now() >= deadline) throw new Error(`Timed out: ${description}; animation=${avatar._time}, visibility=${document.visibilityState}`);
    await wait(50);
  }
};
const form = document.getElementById('form');
const avatar = document.querySelector('login-companion');
avatar.setAttribute('character', kind);
if (kind === 'wizard') avatar.setAttribute('action', 'special');
let actions = 0;
avatar.addEventListener('avatar-action', () => actions++);
let submissions = 0;
let replayProgress = 0;
try {
  const { animationRandom } = await import(ASSETS + '/animation-random.js');
  const originalGetRandomValues = globalThis.crypto.getRandomValues;
  try {
    for (const [sample, expected] of [[0, 0], [0x80000000, 0.5], [0xffffffff, 1 - 2 ** -32]]) {
      globalThis.crypto.getRandomValues = values => { values[0] = sample; return values; };
      check(animationRandom() === expected, `Animation randomness preserves the [0, 1) range for ${sample}`);
    }
  } finally {
    globalThis.crypto.getRandomValues = originalGetRandomValues;
  }
  await import(ASSETS + '/wizard-login.js');
  form.addEventListener('submit', event => {
    if (!event.defaultPrevented) { replayProgress = avatar.actionElapsed; submissions++; event.preventDefault(); }
  });
  await waitFor(() => avatar.dataset.renderer && avatar._camera?.aspect > 0 && avatar._renderer?.domElement.width > 0, 'renderer initialization');
  check(THREE.REVISION === '186', 'Pinned Three.js release is loaded');
  check(avatar.dataset.renderer === 'webgl', 'WebGL renders under strict CSP');
  check(avatar.shadowRoot.querySelector('.motion-toggle').hidden, 'No visible pause button');
  const canvas = avatar._renderer.domElement;
  const boxRatio = () => {
    const { width, height } = avatar._button.getBoundingClientRect();
    return width / height;
  };
  // Layout settles after the fallback image loads, so wait for the camera to follow it.
  await waitFor(() => Math.abs(avatar._camera.aspect - boxRatio()) < 0.001, 'camera follows the settled layout');
  const box = avatar._button.getBoundingClientRect();
  check(Math.abs(avatar._camera.aspect - box.width / box.height) < .001, 'Camera uses actual canvas viewport proportions');
  check(Math.abs(canvas.width / canvas.height - box.width / box.height) < .01, 'Canvas is not stretched or squashed');

  if (kind === 'wizard') {
    const firstSubmitAt = performance.now();
    form.requestSubmit();
    form.requestSubmit();
    check(actions === 1 && submissions === 0, 'Login begins one guard animation and suppresses repeated submits');
    await waitFor(() => submissions === 1, 'first login replay');
    check(submissions === 1 && (replayProgress >= 1.4 || performance.now() - firstSubmitAt >= 4000), 'Login replays once after animation impact or the safety deadline');
    check(form.getAttribute('aria-busy') === null, 'Busy state clears after login replay');
    await waitFor(() => avatar._time >= avatar._actionStart + 3.5, 'wizard action settled', 120000);
    const button = form.querySelector('button[type=submit]');
    form.requestSubmit(button);
    await waitFor(() => submissions === 2, 'submitter login replay');
    check(submissions === 2, 'Click submission also preserves the original submitter');
    await waitFor(() => avatar.actionElapsed >= 3.5, 'second wizard action settled', 120000);
    const stalledAt = performance.now();
    form.requestSubmit();
    cancelAnimationFrame(avatar._frame);
    avatar._frame = 0;
    await waitFor(() => submissions === 3, 'stalled renderer safety replay', 6000);
    check(performance.now() - stalledAt >= 4000 && avatar.actionElapsed < 1.4,
      'A stalled renderer releases login at the safety deadline');
    check(form.getAttribute('aria-busy') === null, 'Stalled renderer clears the busy state');
  }
  const image = avatar.querySelector('img');
  canvas.dispatchEvent(new Event('webglcontextlost', {cancelable:true}));
  check(avatar.dataset.renderer === 'fallback', 'Lost WebGL context switches to fallback');
  const slot = avatar.shadowRoot.querySelector('slot');
  check(!slot.hidden && slot.assignedElements().includes(image), 'Original supplied image is visible after WebGL failure');
  check(avatar._button.disabled && avatar.shadowRoot.querySelector('.motion-toggle').hidden, 'Fallback has no false interactive controls');
  if (kind === 'wizard') {
    form.requestSubmit();
    check(submissions === 4, 'WebGL fallback submits immediately');
  }
  avatar.remove();
  check(!avatar._renderer && !avatar._scene, 'Disconnect releases renderer and scene');
  const originalMatchMedia = window.matchMedia;
  window.matchMedia = query => query.includes('prefers-reduced-motion') ? {matches:true,addEventListener(){},removeEventListener(){}} : originalMatchMedia(query);
  const still = document.createElement('login-companion');
  still.setAttribute('character', kind);
  form.prepend(still);
  await waitFor(() => still.dataset.renderer === 'webgl', 'reduced-motion renderer');
  check(still.dataset.motion === 'still' && !still._frame, 'Reduced motion renders a still pose without an animation loop');
  still.remove();
  window.matchMedia = originalMatchMedia;
  document.getElementById('status').textContent = `PASS: ${results.children.length} checks`;
} catch (error) {
  document.getElementById('status').textContent = `FAIL: ${error.message}`;
}
