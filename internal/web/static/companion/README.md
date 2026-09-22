# Local animated companions

Procedural Netresearch robot, keyholder gopher and wizard gopher, copied from the
ScormIQ companion prototype at commit 6faf9f602c24eef67f5d1502ac72ab84e9f24fe6
(2026-09-22). Adaptations include an external shadow stylesheet for strict CSP,
English/German labels, keyring hit testing and correct canvas viewport sizing.
The component does not read form fields or call any network service.

Load `avatar.css` in the page and `scormiq-avatar.js` as a same-origin ES module.
Use `<scormiq-avatar character="keyholder">` with an image child for no-JavaScript,
module-failure and WebGL-failure fallback. Without an image child, WebGL failure
uses a static SVG. No pause button is enabled in these integrations; the system
reduced-motion preference renders a still pose. Hidden/offscreen components stop
requesting frames and disconnect disposes GPU resources. Gestures have replay
guards; the keyholder retains its subtle eye quirks and reduced blink frequency.

Three.js 0.160.1 is bundled under its MIT license (`vendor/THREE-LICENSE.txt`).
The Netresearch symbol retains its original paths and colors. Reference images
are not distributed. Browser regression checks live in `tests/companion/` at the
repository root; their README explains how to run them without real accounts.

The login loads `wizard-login.js`. Clicking the figure plays its guard gesture. On form submit, the native POST waits 1.4 seconds for the staff impact, then replays once with its original submitter and CSRF token. Reduced motion or failed WebGL submits immediately. No credentials are read by this adapter.
