import * as THREE from './vendor/three.module.js';

// Both gophers are original, procedural meshes interpreted from the supplied images.
// They use the same lifecycle, motion clock and input guards as the robot.
const mix = (a, b, t) => a + (b - a) * t;
const material = (color, roughness = .7, metalness = 0) => new THREE.MeshStandardMaterial({ color, roughness, metalness });
function mesh(parent, geometry, mat, x = 0, y = 0, z = 0) {
  const object = new THREE.Mesh(geometry, mat);
  object.position.set(x, y, z);
  parent.add(object);
  return object;
}
function group(parent, x = 0, y = 0, z = 0) {
  const object = new THREE.Group();
  object.position.set(x, y, z);
  parent.add(object);
  return object;
}
function ellipsoid(parent, mat, x, y, z, sx, sy, sz) {
  const object = mesh(parent, new THREE.SphereGeometry(1, 36, 28), mat, x, y, z);
  object.scale.set(sx, sy, sz);
  return object;
}
function curve(parent, mat, points, radius = .012) {
  return mesh(parent, new THREE.TubeGeometry(new THREE.CatmullRomCurve3(points.map(p => new THREE.Vector3(...p))), 24, radius, 6, false), mat);
}
function leaf(parent, mat, x, y, z, width, length, angle = 0) {
  const shape = new THREE.Shape();
  shape.moveTo(-width / 2, 0);
  shape.bezierCurveTo(-width * .5, length * .45, width * .32, length * .6, 0, length);
  shape.bezierCurveTo(width * .18, length * .68, width * .65, length * .22, width / 2, 0);
  shape.closePath();
  const geometry = new THREE.ExtrudeGeometry(shape, { depth: .035, bevelEnabled: true, bevelThickness: .028, bevelSize: .028, bevelSegments: 2, steps: 1, curveSegments: 10 });
  const object = mesh(parent, geometry, mat, x, y, z);
  object.rotation.z = angle;
  return object;
}
function paw(parent, fur, pads, side, fingers = true) {
  ellipsoid(parent, fur, side * .04, -.40, .04, .19, .31, .20).rotation.z = side * -.18;
  ellipsoid(parent, pads, side * .07, -.64, .09, .18, .17, .16);
  if (fingers) for (let i = 0; i < 3; i++) {
    ellipsoid(parent, pads, -.08 + i * .085, -.68, .20, .044, .09, .049);
  }
}

function addSquintMorph(pupil) {
  // Deform the solid pupil into a curved closed-eye stroke. No alpha blending.
  const geometry = pupil.geometry;
  const positions = geometry.attributes.position;
  const closed = new Float32Array(positions.array.length);
  for (let i = 0; i < positions.count; i++) {
    const x = positions.getX(i);
    closed[i * 3] = x * .23 / pupil.scale.x;
    closed[i * 3 + 1] = (.10 * (1 - x * x) + positions.getY(i) * .026) / pupil.scale.y;
    closed[i * 3 + 2] = positions.getZ(i) * .023 / pupil.scale.z;
  }
  const target = new THREE.Float32BufferAttribute(closed, 3);
  const closedGeometry = geometry.clone();
  closedGeometry.setAttribute('position', target);
  closedGeometry.computeVertexNormals();
  geometry.morphAttributes.position = [target];
  geometry.morphAttributes.normal = [closedGeometry.attributes.normal.clone()];
  closedGeometry.dispose();
  pupil.updateMorphTargets();
}

function buildKeys(parent, silver, darkSilver) {
  const keys = group(parent, .05, -.44, .30);
  keys.name = 'silver-keyring';
  mesh(keys, new THREE.TorusGeometry(.28, .032, 12, 64), silver, 0, -.10, 0);
  mesh(keys, new THREE.TorusGeometry(.26, .013, 8, 64), darkSilver, .014, -.11, -.012);
  const pieces = [-.62, -.25, .18, .56].map((angle, i) => {
    const key = group(keys, (i - 1.5) * .032, -.31, .055 + i * .028);
    key.rotation.z = angle;
    mesh(key, new THREE.TorusGeometry(.105, .034, 10, 28), silver);
    mesh(key, new THREE.BoxGeometry(.073, .69 + i * .04, .055), silver, 0, -.44, 0);
    mesh(key, new THREE.BoxGeometry(.02, .48, .012), darkSilver, -.009, -.45, .034);
    for (let tooth = 0; tooth < 3; tooth++) {
      mesh(key, new THREE.BoxGeometry(.13 + (tooth % 2) * .04, .066, .057), silver, .043, -.67 + tooth * .10, 0);
    }
    key.userData.rest = angle;
    return key;
  });
  // A tiny gopher medallion hangs inside the ring, as in the keyholder reference.
  ellipsoid(keys, silver, 0, -.11, .065, .115, .145, .025);
  for (const side of [-1, 1]) {
    ellipsoid(keys, silver, side * .076, -.015, .06, .04, .04, .025);
    ellipsoid(keys, darkSilver, side * .041, -.081, .09, .013, .020, .009);
  }
  return { keys, pieces };
}

function buildStaff(parent, wood, stone, leather) {
  // Leather grip midpoint is the wrist socket, shared by the staff and curled fingers.
  const staff = group(parent, -.013, -.04, 0);
  staff.name = 'crooked-wizard-staff';
  curve(staff, wood, [[0, -.86, 0], [.06, -.48, .025], [.015, .05, 0], [-.025, .72, -.025], [.07, 1.48, 0], [.02, 1.86, .015]], .073);
  const grain = material('#b28154');
  for (const side of [-1, 1]) curve(staff, grain, [[side * .037, -.82, .053], [.06 + side * .035, -.47, .074], [side * .038, .06, .065], [-.02 + side * .037, .73, .033], [.07 + side * .036, 1.49, .060]], .008);
  for (const [x, y] of [[.055, -.48], [-.025, .77], [.06, 1.45]]) ellipsoid(staff, wood, x, y, 0, .10, .15, .09);
  curve(staff, wood, [[.04, 1.56, 0], [-.20, 1.81, .025], [-.20, 2.12, .02], [-.12, 2.29, .01]], .065);
  curve(staff, wood, [[.04, 1.59, -.02], [.24, 1.87, -.035], [.26, 2.19, -.03], [.17, 2.40, -.02]], .060);
  const crystal = mesh(staff, new THREE.OctahedronGeometry(.25), stone, .035, 2.09, .01);
  crystal.scale.set(.72, 1.65, .80);
  crystal.rotation.z = -.10;
  crystal.name = 'staff-crystal';
  curve(staff, new THREE.MeshBasicMaterial({ color: '#e0fff5' }), [[-.045, 1.94, .15], [-.025, 2.12, .17], [.035, 2.41, .04]], .011);
  const brass = material('#bba578');
  for (const y of [-.14, .23, 1.57]) {
    const collar = mesh(staff, new THREE.TorusGeometry(.077, .018, 8, 32), brass, .02, y, 0);
    collar.rotation.x = Math.PI / 2;
  }
  for (let i = 0; i < 9; i++) {
    const wrap = mesh(staff, new THREE.TorusGeometry(.079, .014, 6, 20), leather, .013, -.11 + i * .037, 0);
    wrap.rotation.x = Math.PI / 2;
  }
  return staff;
}

function buildStaffGrip(arm, fur, pads) {
  // A bent forearm leads into a vertical fist, rather than an open downward-facing paw.
  const forearm = ellipsoid(arm, fur, -.10, -.37, .12, .19, .25, .20);
  forearm.rotation.z = -.42;
  const wrist = group(arm, -.18, -.52, .21);
  wrist.name = 'staff-gripping-wrist';
  ellipsoid(wrist, pads, .105, -.005, -.040, .16, .19, .13).name = 'gripping-palm';
  for (let i = 0; i < 3; i++) {
    const y = .095 - i * .087;
    // Curl around the front and outer side of the leather-wrapped shaft.
    curve(wrist, pads, [[.16, y, .055], [.07, y, .108], [-.035, y, .11], [-.105, y, .06], [-.10, y, -.025]], .046).name = 'curled-finger';
  }
  const thumb = ellipsoid(wrist, pads, .105, .125, .115, .065, .125, .062);
  thumb.rotation.z = -.70;
  thumb.name = 'opposing-thumb';
  return wrist;
}

function buildWizardHat(head) {
  const hat = group(head);
  hat.name = 'bent-wizard-hat';
  const felt = material('#536c7b');
  const brim = ellipsoid(hat, felt, 0, .91, -.06, 1.32, .075, .86);
  brim.rotation.z = -.055;
  // The crown bends backward in depth, then flops to one side with a drooping tip.
  const spine = new THREE.CatmullRomCurve3([
    new THREE.Vector3(0, .94, -.08), new THREE.Vector3(-.05, 1.27, -.18),
    new THREE.Vector3(.01, 1.65, -.38), new THREE.Vector3(.23, 1.95, -.65),
    new THREE.Vector3(.54, 2.02, -.97), new THREE.Vector3(.85, 1.83, -1.24),
  ]);
  const segments = 32;
  const frames = spine.computeFrenetFrames(segments, false);
  const crown = new THREE.ConeGeometry(1, 1, 48, segments);
  const positions = crown.attributes.position;
  for (let i = 0; i < positions.count; i++) {
    const t = THREE.MathUtils.clamp(positions.getY(i) + .5, 0, 1);
    const ring = Math.round(t * segments);
    const radius = t < 1 ? .84 * Math.pow(1 - t, -.08) : 0;
    const point = spine.getPointAt(t)
      .addScaledVector(frames.normals[ring], positions.getX(i) * radius)
      .addScaledVector(frames.binormals[ring], -positions.getZ(i) * radius * .83);
    positions.setXYZ(i, point.x, point.y, point.z);
  }
  crown.computeVertexNormals();
  mesh(hat, crown, felt).name = 'folded-hat-crown';
  const band = mesh(hat, new THREE.TorusGeometry(.765, .045, 10, 64), material('#344957'), 0, 1.02, -.10);
  band.rotation.x = Math.PI / 2;
  band.scale.y = .81;
  return hat;
}

export function applyCelStyle(root, scene) {
  // A single key light and nearest-sampled ramp give crisp, genuinely stepped lighting.
  const ramp = new THREE.DataTexture(new Uint8Array([58, 123, 195, 255]), 4, 1, THREE.RedFormat);
  ramp.minFilter = ramp.magFilter = THREE.NearestFilter;
  ramp.generateMipmaps = false;
  ramp.needsUpdate = true;
  let keyFound = false;
  scene.traverse(object => {
    if (object.isHemisphereLight) object.intensity = 0;
    if (object.isDirectionalLight) {
      object.intensity = keyFound ? 0 : Math.PI;
      keyFound = true;
    }
  });
  const surfaces = [];
  root.traverse(object => { if (object.isMesh) surfaces.push(object); });
  const materials = new Map();
  surfaces.forEach(object => {
    const previous = object.material;
    if (previous.isMeshBasicMaterial) return; // Preserve eye glints, the robot display, and its logo.
    if (!materials.has(previous)) materials.set(previous, new THREE.MeshToonMaterial({
      color: previous.color, emissive: previous.color.clone().multiplyScalar(.065),
      gradientMap: ramp, toneMapped: false, transparent: previous.transparent,
      opacity: previous.opacity, depthWrite: previous.depthWrite,
    }));
    object.material = materials.get(previous);
    // Ink existing strokes only once; dense contour shells muddy tiny facial details.
    if (object.geometry.type === 'TubeGeometry' || previous.color.r + previous.color.g + previous.color.b < .16 || Math.max(object.scale.x, object.scale.y, object.scale.z) < .17 || previous.color.getHex() === 0xefaea7) return;
    const box = object.geometry.parameters;
    if (object.geometry.type === 'BoxGeometry' && box.widthSegments === 1 && box.heightSegments === 1 && box.depthSegments === 1) {
      const edges = new THREE.LineSegments(new THREE.EdgesGeometry(object.geometry), new THREE.LineBasicMaterial({ color: '#465564', transparent: true, opacity: .75, toneMapped: false }));
      edges.name = 'cel-hard-edges';
      object.add(edges);
      return;
    }
    const outline = new THREE.Mesh(object.geometry, new THREE.ShaderMaterial({
      uniforms: { ink: { value: new THREE.Color('#302735') }, opacity: { value: previous.opacity }, thickness: { value: object.geometry.type === 'TorusGeometry' ? .005 : .012 } },
      side: THREE.BackSide, transparent: true, depthWrite: false, toneMapped: false,
      vertexShader: `uniform float thickness;
        void main() {
          vec4 p = modelViewMatrix * vec4(position, 1.0);
          p.xyz += normalize(normalMatrix * normal) * thickness;
          gl_Position = projectionMatrix * p;
        }`,
      fragmentShader: `uniform vec3 ink; uniform float opacity;
        void main() {
          gl_FragColor = vec4(ink, opacity);
          #include <colorspace_fragment>
        }`,
    }));
    outline.name = 'cel-ink';
    // View-space expansion keeps line weight consistent on flattened eyes and tiny keys.
    outline.onBeforeRender = () => { outline.material.uniforms.opacity.value = object.material.opacity; };
    object.add(outline);
  });
  materials.forEach((replacement, previous) => previous.dispose());
  return ramp;
}

function plushBody(width, height, depth, roundness, taper) {
  // A rounded barrel profile fills the flanks and flattens the base, unlike an ellipsoid.
  const geometry = new THREE.SphereGeometry(1, 56, 48);
  const positions = geometry.attributes.position;
  for (let i = 0; i < positions.count; i++) {
    const y = positions.getY(i);
    const radius = Math.sqrt(Math.max(0, 1 - y * y));
    const profile = Math.pow(Math.max(0, 1 - Math.abs(y) ** roundness), 1 / roundness);
    const factor = radius > .00001 ? profile / radius : 0;
    positions.setXYZ(i, positions.getX(i) * factor * (width - y * taper), y * height, positions.getZ(i) * factor * depth);
  }
  geometry.computeVertexNormals();
  return geometry;
}

export function buildGopher(avatar, character) {
  const wizard = character === 'wizard';
  const root = avatar._robot;
  avatar._torso = group(root);
  avatar._head = group(root, 0, wizard ? .77 : .40, 0);
  avatar._face = group(avatar._head);
  const dark = material(wizard ? '#271c18' : '#372334', .58);
  const fur = material(wizard ? '#65412c' : '#73c4d0');
  const light = material(wizard ? '#bd9263' : '#b4e9ef');
  const cream = material(wizard ? '#dfbf8b' : '#f3debc');
  const white = material('#fffdf7', .28);
  const eyeBlack = material(wizard ? '#100d0c' : '#332030', wizard ? .08 : .4);
  const shine = new THREE.MeshBasicMaterial({ color: '#ffffff' });
  const pupilGroups = [];
  const eyeGroups = [];
  const pupilMeshes = [];
  const glints = [];
  const brows = [];
  let keys, pieces, staff, wrist, beard, cape;
  avatar._arms = [-1, 1].map(side => group(avatar._torso, side * (wizard ? .74 : 1.02), wizard ? -.24 : -.27, wizard ? .02 : .21));
  if (!wizard) {
    // Short front paws belong to the barrel body and follow its gentle gaze/breathing.
    avatar._arms.forEach(arm => avatar._head.add(arm));
    avatar._arms[0].position.set(-.72, -.50, .72);
    avatar._arms[1].position.set(.59, -.41, .80);
  } else avatar._arms[0].position.x = -.86;
  avatar._feet = [-1, 1].map(side => {
    const foot = group(avatar._torso, side * (wizard ? .49 : .65), -1.32, .12);
    if (!wizard) ellipsoid(foot, fur, 0, .16, -.04, .18, .25, .22);
    ellipsoid(foot, wizard ? fur : dark, 0, -.08, .10, wizard ? .27 : .28, .12, .35);
    if (wizard) for (let i = 0; i < 3; i++) ellipsoid(foot, light, -.14 + i * .13, -.075, .36, .075, .065, .12);
    return foot;
  });
  if (!wizard) {
    mesh(avatar._head, plushBody(1.23, 1.45, .74, 3.2, .09), fur, 0, -.28, 0).name = 'keyholder-body';
    mesh(avatar._head, plushBody(1.10, 1.19, .41, 2.8, .06), light, 0, -.49, .34).name = 'keyholder-belly';
    // Dark band behind the two big white eyes is a defining feature of this gopher.
    ellipsoid(avatar._head, dark, 0, .57, .53, .98, .28, .25);
    for (const side of [-1, 1]) {
      ellipsoid(avatar._head, dark, side * .87, 1.12, -.01, .29, .29, .19);
      ellipsoid(avatar._head, fur, side * .87, 1.13, .07, .265, .26, .17);
      const arm = avatar._arms[side === -1 ? 0 : 1];
      ellipsoid(arm, fur, 0, -.07, .02, .15, .17, .115);
      ellipsoid(arm, fur, -side * .025, -.21, .105, .17, .135, .12).name = 'little-front-paw';
      for (let i = 0; i < 2; i++) curve(arm, dark, [[-.055 + i * .065, -.26, .214], [-.049 + i * .065, -.29, .208]], .006);
      ellipsoid(avatar._head, material('#efaea7'), side * .70, .12, .78, .12, .052, .023);
    }
    ellipsoid(avatar._head, cream, 0, .49, .88, .105, .066, .055);
    curve(avatar._head, dark, [[0, .44, .87], [0, .35, .88], [-.08, .31, .875]], .014);
    for (const side of [-1, 1]) {
      const tooth = group(avatar._head, side === -1 ? -.065 : .063, side === -1 ? .298 : .302, .86);
      tooth.rotation.z = side === -1 ? -.045 : .035;
      const width = side === -1 ? .054 : .055;
      const height = side === -1 ? .103 : .099;
      ellipsoid(tooth, dark, 0, -.006, -.012, width + .018, height + .018, .039);
      ellipsoid(tooth, white, 0, .003, .02, width, height, .035);
    }
    // Small asymmetric ink marks echo a hand-inked game character.
    for (let i = 0; i < 3; i++) curve(avatar._head, dark, [[-.99 + i * .035, -.69 - i * .08, .56], [-.94 + i * .035, -.75 - i * .08, .59]], .008);
    const tail = curve(avatar._torso, fur, [[.7, -.91, -.2], [1.14, -.73, -.18], [1.28, -.48, -.14]], .10);
    tail.name = 'tiny-gopher-tail';
    ({ keys, pieces } = buildKeys(avatar._arms[1], material('#c5d6de', .23, .76), material('#5c7c86', .4, .58)));
    keys.position.set(.02, -.29, .22);
    keys.scale.setScalar(.85);
  } else {
    const tunic = material('#526c70', .95);
    const cloak = material('#392d2a', 1);
    ellipsoid(avatar._torso, fur, 0, -.68, 0, .74, .69, .47);
    cape = ellipsoid(avatar._torso, cloak, -.12, -.51, -.31, .91, .90, .30);
    cape.rotation.z = -.14;
    ellipsoid(avatar._torso, tunic, 0, -.53, .24, .68, .70, .38);
    // Visible folds give the cloth its own material language, distinct from fur.
    for (let i = 0; i < 4; i++) curve(avatar._torso, material(i % 2 ? '#647d7e' : '#42595d'), [[-.47 + i * .29, -.23, .52], [-.40 + i * .24, -.57, .61], [-.34 + i * .24, -.95, .47]], .012);
    ellipsoid(avatar._torso, cloak, 0, -1.00, .15, .65, .10, .45);
    mesh(avatar._torso, new THREE.BoxGeometry(.18, .12, .05), material('#a4926b', .5, .35), .04, -1.00, .60);
    for (const side of [-1, 1]) {
      ellipsoid(avatar._torso, fur, side * .45, -1.13, -.02, .27, .30, .29);
      const arm = avatar._arms[side === -1 ? 0 : 1];
      // Overlapping shoulder, sleeve, and upper arm keep the paw attached in every pose.
      ellipsoid(arm, cloak, 0, -.035, .02, .26, .24, .25).name = 'wizard-shoulder';
      ellipsoid(arm, fur, side * .025, -.22, .03, .205, .31, .22).name = 'wizard-upper-arm';
      if (side === -1) wrist = buildStaffGrip(arm, fur, light);
      else paw(arm, fur, light, side);
      ellipsoid(avatar._head, fur, side * .93, .15, 0, .23, .30, .19);
      ellipsoid(avatar._head, light, side * .95, .17, .13, .14, .20, .09);
    }
    ellipsoid(avatar._head, fur, 0, -.06, 0, 1.12, .91, .71);
    ellipsoid(avatar._head, light, -.08, .20, .18, 1.02, .76, .59);
    // Shaped tufts around the silhouette and brow, not a texture pasted on a sphere.
    const furColors = [light, cream, material('#b99770'), fur];
    for (let i = 0; i < 25; i++) {
      const angle = i / 25 * Math.PI * 2;
      const x = Math.sin(angle), y = Math.cos(angle);
      leaf(avatar._head, furColors[i % 4], x * .93, y * .70, .23 + (i % 3) * .025, .15 + (i % 3) * .025, .17 + (i % 4) * .04, -angle + .44);
    }
    const fringe = group(avatar._head);
    fringe.name = 'shaggy-forehead-and-cheeks';
    for (let i = 0; i < 11; i++) {
      const x = -.85 + i * .17;
      leaf(fringe, furColors[i % 3], x, .83 - Math.abs(x) * .08, .64 - Math.abs(x) * .10, .23, .20 + (i % 3) * .045, Math.PI + x * .42);
    }
    for (const side of [-1, 1]) for (let i = 0; i < 6; i++) {
      leaf(fringe, furColors[i % 3], side * (.86 - i * .012), .08 - i * .10, .57 + i * .026, .18, .29 + (i % 2) * .07, -side * (1.12 + i * .13));
    }
    buildWizardHat(avatar._head);
    beard = group(avatar._head, 0, -.31, .47);
    ellipsoid(beard, material('#c5a078'), 0, -.16, .10, .74, .52, .38);
    for (const side of [-1, 1]) {
      ellipsoid(beard, light, side * .37, .02, .17, .49, .34, .34);
      for (let i = 0; i < 7; i++) {
        leaf(beard, furColors[i % 3], side * (.33 + i * .061), .12 - i * .065, .37 - i * .018, .12, .30 + (i % 3) * .06, side * (1.8 + i * .11));
      }
    }
    for (let i = 0; i < 9; i++) leaf(beard, furColors[i % 3], -.45 + i * .11, -.42 + Math.abs(i - 4) * .04, .27, .14, .24 + (i % 3) * .03, Math.PI + (i - 4) * -.13);
    // Fine, swept locks across the brow and cheeks soften the sculpted silhouette.
    for (let i = 0; i < 15; i++) {
      const x = -.75 + (i % 8) * .21;
      const y = .62 + Math.floor(i / 8) * .12;
      curve(avatar._head, furColors[i % 3], [[x, y - .035, .68 - Math.abs(x) * .19], [x + .024, y + .015, .68 - Math.abs(x) * .19], [x + .035, y + .09, .64 - Math.abs(x) * .19]], .010);
    }
    for (const side of [-1, 1]) for (let i = 0; i < 3; i++) curve(avatar._head, dark, [[side * .46, -.17 - i * .05, 1.0], [side * .96, -.09 - i * .10, .93], [side * (1.37 + (i % 2) * .1), .04 - i * .17, .79]], .008);
    const nose = ellipsoid(avatar._head, eyeBlack, 0, -.065, 1.035, .19, .105, .12);
    nose.rotation.z = -.025;
    ellipsoid(avatar._head, shine, -.025, -.029, 1.136, .08, .014, .012);
    curve(avatar._head, dark, [[0, -.15, 1.075], [0, -.31, 1.058], [-.04, -.37, 1.019]], .022);
    staff = buildStaff(wrist, material('#67422a'), material('#829c91', .52, .35), material('#34241d'));
  }
  const eyeY = wizard ? .31 : .62;
  const eyeZ = wizard ? .71 : .74;
  const eyeX = wizard ? .49 : .60;
  for (const side of [-1, 1]) {
    if (wizard) ellipsoid(avatar._head, material('#94704c'), side * eyeX, eyeY, eyeZ - .09, .405, .48, .20);
    const eye = group(avatar._head, side * eyeX, eyeY, eyeZ);
    ellipsoid(eye, dark, 0, 0, -.02, wizard ? .37 : .45, wizard ? .435 : .45, .22);
    ellipsoid(eye, white, 0, .004, .021, wizard ? .324 : .432, wizard ? .383 : .432, .216);
    const pupil = group(eye, wizard ? side * -.015 : side * .095, wizard ? -.026 : -.02, wizard ? .19 : .232);
    pupil.userData.gaze = { x: pupil.position.x, y: pupil.position.y };
    const pupilMesh = ellipsoid(pupil, eyeBlack, 0, 0, 0, wizard ? .265 : .13, wizard ? .33 : .13, wizard ? .126 : .043);
    addSquintMorph(pupilMesh);
    pupilMeshes.push(pupilMesh);
    const highlights = [];
    if (wizard) {
      highlights.push(ellipsoid(pupil, shine, -.079, .123, .106, .081, .089, .026));
      highlights.push(ellipsoid(pupil, shine, .07, -.18, .111, .055, .025, .016));
    } else {
      highlights.push(ellipsoid(pupil, shine, -.032, .040, .039, .025, .028, .010));
    }
    highlights.forEach(highlight => { highlight.userData.openScale = highlight.scale.clone(); });
    glints.push(highlights);
    eye.traverse(object => {
      if (!object.isMesh) return;
      object.material = object.material.clone();
      object.material.transparent = false;
    });
    eyeGroups.push(eye);
    pupilGroups.push(pupil);
    if (wizard) {
      const brow = group(avatar._head, side * eyeX, .73, .65);
      for (let i = 0; i < 4; i++) leaf(brow, cream, -.21 + i * .13, 0, 0, .12, .14, side * -.5);
      brows.push(brow);
    }
  }
  const mouth = ellipsoid(avatar._head, dark, 0, wizard ? -.49 : .21, wizard ? .971 : .825, wizard ? .14 : .11, wizard ? .14 : .06, .029);
  const tongue = ellipsoid(mouth, material('#c1847b'), 0, -.38, .72, .55, .22, .42);
  tongue.name = 'little-tongue';
  const glow = new THREE.MeshBasicMaterial({ color: '#38a7b4', transparent: true, opacity: 0, side: THREE.DoubleSide, depthWrite: false, toneMapped: false });
  const halo = mesh(avatar._scene, new THREE.RingGeometry(.73, .77, 64), glow, -1.23, -1.57, .15);
  halo.rotation.x = -Math.PI / 2;
  halo.name = 'staff-ward';
  halo.visible = false;
  const shockwaves = wizard ? [0, 1].map(() => {
    const ring = mesh(avatar._scene, new THREE.RingGeometry(.94, 1, 64), glow.clone(), 0, -1.51, .1);
    ring.rotation.x = -Math.PI / 2;
    ring.visible = false;
    return ring;
  }) : [];
  const sparks = [];
  let shield;
  if (wizard) {
    const shieldMaterial = glow.clone();
    shield = mesh(avatar._scene, new THREE.RingGeometry(1.43, 1.455, 64), shieldMaterial, 0, .12, -.70);
    shield.name = 'runic-ward';
    shield.visible = false;
    for (let i = 0; i < 12; i++) {
      const angle = i / 12 * Math.PI * 2;
      const rune = mesh(shield, new THREE.BoxGeometry(.035, .12, .012), shieldMaterial, Math.sin(angle) * 1.36, Math.cos(angle) * 1.36, 0);
      rune.rotation.z = -angle;
    }
    const shard = new THREE.OctahedronGeometry(.035);
    const sparkMaterial = new THREE.MeshBasicMaterial({ color: '#e3ad46', transparent: true, opacity: 0, depthWrite: false, toneMapped: false });
    for (let i = 0; i < 12; i++) { const spark = mesh(avatar._scene, shard, sparkMaterial); spark.visible = false; sparks.push(spark); }
  }
  avatar._rig = { wizard, eyes: eyeGroups, pupils: pupilGroups, pupilMeshes, glints, brows, mouth, keys, pieces, staff, wrist, beard, cape, halo, shockwaves, sparks, shield, contact: new THREE.Vector3(), joy: 0 };
  if (!wizard) {
    avatar._rig.quirk = { nextAt: avatar._time + 8 + Math.random() * 6, start: -100, type: null, eye: 0, driftDuration: .72, returnAt: .96, dx: 0, dy: 0, x: 0, y: 0, wink: 0 };
    // Put the bean's pivot at its base so gaze and breathing never lift its body off its feet.
    avatar._head.children.forEach(child => { child.position.y += 1.72; });
    avatar._head.position.y = -1.32;
  }
  avatar.dataset.character = character;
}

function updateEyeQuirk(avatar, blend, staticPose, quietIdle, blink) {
  const quirk = avatar._rig.quirk;
  if (!quirk) return null;
  const now = avatar._time;
  if (staticPose) {
    quirk.type = null;
    quirk.x = quirk.y = quirk.wink = 0;
    quirk.nextAt = Math.max(quirk.nextAt, now + 18);
  } else if (blend > 0) {
    const finished = quirk.type && now - quirk.start >= (quirk.type === 'drift' ? quirk.returnAt + .36 : .48);
    if (quirk.type && (!quietIdle || finished)) {
      quirk.type = null;
      quirk.nextAt = now + 18 + Math.random() * 16;
      avatar._nextBlink = Math.max(avatar._nextBlink, now + .5);
    }
    if (!quietIdle) quirk.nextAt = Math.max(quirk.nextAt, now + 4);
    if (quietIdle && !quirk.type && now >= quirk.nextAt && blink === 1) {
      quirk.type = Math.random() < .65 ? 'drift' : 'wink';
      quirk.eye = Math.random() < .5 ? 0 : 1;
      quirk.start = now;
      quirk.dx = (quirk.eye === 0 ? -1 : 1) * (.035 + Math.random() * .025);
      quirk.dy = (Math.random() - .5) * .024;
      quirk.driftDuration = .72 + Math.pow(Math.random(), 2) * 3.2;
      quirk.returnAt = quirk.driftDuration + .24;
    }
  }
  const age = now - quirk.start;
  const drift = quirk.type === 'drift' ? THREE.MathUtils.smoothstep(age, 0, quirk.driftDuration) * (1 - THREE.MathUtils.smoothstep(age, quirk.returnAt, quirk.returnAt + .12)) : 0;
  const wink = quirk.type === 'wink' ? THREE.MathUtils.smoothstep(age, 0, .09) * (1 - THREE.MathUtils.smoothstep(age, .14, .31)) : 0;
  // Return quickly without changing the underlying pointer-tracking gaze.
  const quickBlend = 1 - Math.pow(1 - blend, 36 / 11);
  quirk.x = mix(quirk.x, drift * quirk.dx, quickBlend);
  quirk.y = mix(quirk.y, drift * quirk.dy, quickBlend);
  quirk.wink = mix(quirk.wink, wink, quickBlend);
  avatar.dataset.quirk = quirk.type || 'none';
  return quirk;
}

export function animateGopher(avatar, { blend, t, blink, happy, thinking, wave, joy, action, staticPose, windup, brace, actionAge, quietIdle }) {
  const rig = avatar._rig;
  const quirk = updateEyeQuirk(avatar, blend, staticPose, quietIdle, blink);
  rig.joy = mix(rig.joy, happy ? 1 - (rig.wizard ? action : 0) : wave * .24, blend);
  rig.eyes.forEach((eye, i) => {
    const winking = quirk && quirk.eye === i ? quirk.wink : 0;
    eye.scale.y = (1 - rig.joy * .22) * (quirk?.type ? 1 : blink) * (1 - winking * .94);
    rig.pupilMeshes[i].morphTargetInfluences[0] = rig.joy;
    rig.glints[i].forEach(highlight => {
      highlight.scale.copy(highlight.userData.openScale).multiplyScalar(1 - rig.joy);
    });
    const pupil = rig.pupils[i];
    // The wizard's open pupils protrude farther; keep the flattened stroke in front of its sclera.
    pupil.position.z = rig.wizard ? .19 + rig.joy * .085 : .232;
    const side = i === 0 ? -1 : 1;
    const gaze = pupil.userData.gaze;
    gaze.x = mix(gaze.x, (rig.wizard ? side * -.015 : side * .095) + avatar._look.x * (rig.wizard ? .035 : .075), blend);
    gaze.y = mix(gaze.y, -.02 - avatar._look.y * (rig.wizard ? .028 : .10), blend);
    pupil.position.x = gaze.x + (quirk && quirk.eye === i ? quirk.x : 0);
    pupil.position.y = gaze.y + (quirk && quirk.eye === i ? quirk.y : 0);
  });
  const mouth = rig.mouth;
  mouth.scale.y = mix(mouth.scale.y, rig.wizard ? .14 + rig.joy * .10 + action * .09 : .04 + rig.joy * .10, blend);
  mouth.scale.x = mix(mouth.scale.x, rig.wizard ? .14 + rig.joy * .13 : .10 + rig.joy * .07, blend);
  rig.brows.forEach((brow, i) => { brow.rotation.z = mix(brow.rotation.z, (i === 0 ? 1 : -1) * (action * -.22 + (thinking ? .12 : 0)), blend); });
  if (rig.keys) {
    const breath = 1 + Math.sin(t * 1.7) * .006;
    avatar._head.scale.set(mix(avatar._head.scale.x, 1 / Math.sqrt(breath), blend), mix(avatar._head.scale.y, breath, blend), 1);
    // Counter-rotation lets the ring hang from the paw rather than rotating rigidly with it.
    rig.keys.rotation.z = mix(rig.keys.rotation.z, -avatar._arms[1].rotation.z + Math.sin(t * 2) * .04 + Math.sin(t * 22) * action * .22, blend);
    rig.pieces.forEach((key, i) => {
      key.rotation.z = mix(key.rotation.z, key.userData.rest + Math.sin(t * 2.3 + i * .6) * .035 + Math.sin(t * 19 + i * .7) * action * .18, blend);
    });
  }
  if (rig.staff) {
    // Hand and staff rotate together around the grip; all lifting comes from the arm.
    rig.wrist.rotation.z = mix(rig.wrist.rotation.z, -avatar._arms[0].rotation.z + .035 - windup * .18 - brace * .035, blend);
    rig.wrist.rotation.x = mix(rig.wrist.rotation.x, -avatar._arms[0].rotation.x - windup * .18, blend);
    rig.cape.rotation.x = mix(rig.cape.rotation.x, -brace * .21 - windup * .1, blend);
    rig.cape.rotation.z = mix(rig.cape.rotation.z, -.14 - brace * .12, blend);
    avatar._robot.updateMatrixWorld(true);
    rig.staff.localToWorld(rig.contact.set(0, -.86, 0));
    const impactAge = actionAge - .78;
    const pulse = !staticPose && impactAge >= 0 && impactAge < 1.2 ? Math.exp(-impactAge * 2.8) : 0;
    rig.halo.position.set(rig.contact.x, -1.50, rig.contact.z);
    rig.halo.material.opacity = mix(rig.halo.material.opacity, staticPose ? 0 : brace * .50 + pulse * .45, blend);
    rig.halo.visible = rig.halo.material.opacity > .005;
    rig.halo.scale.setScalar(1 + brace * .4);
    rig.shield.material.opacity = mix(rig.shield.material.opacity, staticPose ? 0 : brace * .48 + pulse * .40, blend);
    rig.shield.visible = rig.shield.material.opacity > .005;
    rig.shield.scale.setScalar(.90 + brace * .20);
    rig.shield.rotation.z = -brace * .18;
    rig.shockwaves.forEach((ring, i) => {
      const age = impactAge - i * .13;
      const active = !staticPose && age >= 0 && age < 1.15;
      ring.material.opacity = active ? Math.sin(Math.PI * Math.min(age / .12, 1) / 2) * (1 - age / 1.15) * .8 : 0;
      ring.visible = ring.material.opacity > .005;
      ring.position.x = rig.contact.x; ring.position.z = rig.contact.z;
      ring.scale.setScalar(.24 + Math.max(0, age) * 2.8);
    });
    rig.sparks.forEach((spark, i) => {
      const age = Math.max(0, impactAge);
      const angle = i / rig.sparks.length * Math.PI * 2;
      spark.material.opacity = pulse;
      spark.visible = pulse > .005;
      spark.position.set(rig.contact.x + Math.cos(angle) * age * 1.7, -1.47 + age * (1.7 + (i % 3) * .25) - age * age * 1.9, rig.contact.z + Math.sin(angle) * age * 1.7);
      spark.rotation.z = angle + age * 4;
      spark.scale.setScalar(1 + pulse);
    });
    rig.beard.rotation.x = mix(rig.beard.rotation.x, Math.sin(t * 1.8) * .018 + joy * .04, blend);
  }
}

export function gopherFallback(character) {
  const wizard = character === 'wizard';
  return `<svg class="fallback" viewBox="${wizard ? '0 -45 400 485' : '0 0 400 440'}" role="img" aria-label="${wizard ? 'Furry wizard gopher with a backward-bent hat and crooked staff' : 'Turquoise keyholder gopher with silver keys'}">
    <ellipse cx="200" cy="394" rx="112" ry="13" fill="#2f99a4" opacity=".12"/>
    ${wizard ? '<path d="M105 362Q70 226 120 204H275Q321 279 290 365Z" fill="#392d2a"/><ellipse cx="200" cy="294" rx="79" ry="92" fill="#526c70"/><path d="M62 388L72 271L68 210L78 103M78 112Q47 87 62 47M78 112Q105 84 94 44" fill="none" stroke="#67422a" stroke-width="9" stroke-linecap="round"/><path d="M79 31L92 73L78 101L65 73Z" fill="#829c91" stroke="#302735" stroke-width="2"/><path d="M79 31L78 101L65 73Z" fill="#b9d4c9"/><path d="M70 243L78 245M69 252L77 254M68 261L76 263" stroke="#34241d" stroke-width="4"/>' : ''}
    ${wizard ? '<ellipse cx="200" cy="175" rx="121" ry="113" fill="#dfc297"/>' : '<path d="M200 95C295 95 314 123 322 211C332 326 334 381 258 384H142C66 381 68 326 78 211C86 123 105 95 200 95Z" fill="#73c4d0" stroke="#302735" stroke-width="3"/>'}
    ${wizard ? '<path d="M83 209L64 258L104 246L95 284L149 269L176 297L213 278L245 292L269 262L304 267L290 228" fill="#c5a078"/>' : '<path d="M200 174C282 174 302 229 306 296C310 361 282 379 244 379H156C118 379 90 361 94 296C98 229 118 174 200 174Z" fill="#b2e8f1"/><circle cx="117" cy="109" r="26" fill="#69b6c4"/><circle cx="283" cy="109" r="26" fill="#69b6c4"/><path d="M99 166H302" stroke="#372334" stroke-width="30"/>'}
    ${wizard ? '<g fill="#dfc297" stroke="#76513d" stroke-width="2"><path d="M106 199L66 212L98 225L58 238L108 245M294 199L334 212L302 225L342 238L292 245"/><path d="M92 94L116 127L131 105L147 133L168 106L190 137L209 105L231 132L248 104L269 127L285 99Z"/></g><path d="M109 93Q120 2 191 -22Q258 -38 282 25Q252 5 247 37L280 98Z" fill="#536c7b" stroke="#302735" stroke-width="3"/><path d="M240 -10Q255 3 247 37L280 98L246 95Q205 28 240 -10Z" fill="#3d5363"/><path d="M112 82L275 86L280 100L108 97Z" fill="#344957"/><path d="M68 94Q174 76 316 101Q340 117 287 116L107 108Q59 108 68 94Z" fill="#536c7b" stroke="#302735" stroke-width="3"/>' : ''}
    <g fill="white" stroke="#372334" stroke-width="3"><circle cx="145" cy="166" r="43"/><circle cx="255" cy="166" r="43"/></g>
    <g fill="#271c23"><ellipse cx="${wizard ? 145 : 138}" cy="170" rx="${wizard ? 30 : 13}" ry="${wizard ? 34 : 13}"/><ellipse cx="${wizard ? 255 : 262}" cy="170" rx="${wizard ? 30 : 13}" ry="${wizard ? 34 : 13}"/></g>
    ${wizard ? '' : '<g fill="white"><circle cx="134" cy="166" r="3"/><circle cx="258" cy="166" r="3"/></g><g fill="#efaea7"><ellipse cx="132" cy="219" rx="12" ry="5"/><ellipse cx="268" cy="219" rx="12" ry="5"/></g>'}
    ${wizard ? '<g fill="white"><circle cx="134" cy="153" r="10"/><circle cx="244" cy="153" r="10"/></g><path d="M185 209Q200 200 215 209L200 222Z" fill="#271c23"/><path d="M165 221L64 208M167 229L58 232M236 221L334 208M235 229L338 234" stroke="#493529" stroke-width="2"/>' : '<g fill="#73c4d0" stroke="#302735" stroke-width="2"><path d="M117 251Q106 251 109 267Q103 280 117 282Q134 281 129 267Q131 251 117 251Z"/><path d="M273 235Q261 235 262 250Q255 263 270 266Q288 264 283 250Q289 237 273 235Z"/></g><ellipse cx="200" cy="180" rx="10" ry="7" fill="#f3debc"/><g fill="white" stroke="#372334" stroke-width="2"><ellipse cx="194" cy="199.5" rx="6" ry="10.3" transform="rotate(-3 194 199.5)"/><ellipse cx="207" cy="199" rx="6.1" ry="10" transform="rotate(2 207 199)"/></g><g stroke="#a0bac6" fill="none" stroke-width="8"><circle cx="279" cy="275" r="25"/><path d="M274 300L249 375L264 380M279 300L284 389L301 389M288 295L318 370L332 364"/></g>'}
    <ellipse cx="141" cy="382" rx="32" ry="13" fill="${wizard ? '#76513d' : '#372334'}"/><ellipse cx="259" cy="382" rx="32" ry="13" fill="${wizard ? '#76513d' : '#372334'}"/>
  </svg>`;
}
