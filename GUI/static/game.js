"use strict";

// "Coste on the ward": an HTML5 canvas arcade game driven by the Go ward API.
// The server (GUI/main.go) owns the hospital model and makes patients doze off
// again; this file only draws them and tells the server when Coste wakes one.

const W = 960;
const H = 600;
const SCALE = 3; // 14x14 frames -> 42x42 pixels

const ANIMS = {
  costeBase: { sheet: "Coste-base.png", frames: 1, fw: 14, fh: 14, fps: 1 },
  costeBlink: { sheet: "Coste-idle1-eyes.png", frames: 2, fw: 14, fh: 14, fps: 2 },
  costeVallenato: { sheet: "Coste-idle2-acordeon.png", frames: 23, fw: 14, fh: 14, fps: 15 },
  costeWalk: { sheet: "Coste-walk.png", frames: 4, fw: 14, fh: 14, fps: 9 },
  costeAsleep: { sheet: "Coste-asleep.png", frames: 10, fw: 14, fh: 14, fps: 5 },
  doctorIdle: { sheet: "Doctor-idle.png", frames: 2, fw: 14, fh: 14, fps: 2 },
  doctorWalk: { sheet: "Doctor-walk.png", frames: 4, fw: 14, fh: 14, fps: 9 },
  orderlyIdle: { sheet: "Orderly-idle.png", frames: 2, fw: 14, fh: 14, fps: 2 },
  orderlyWalk: { sheet: "Orderly-walk.png", frames: 4, fw: 14, fh: 14, fps: 9 },
  catIdle: { sheet: "Cat-idle.png", frames: 4, fw: 10, fh: 8, fps: 4 },
};

const POWER_COOLDOWN = 4200;
const POWER_DURATION = 1500;
const POWER_RADIUS = 190;
const TOUCH_RADIUS = 36;
const TOUCH_COOLDOWN = 900;

// Twelve beds: two rows of six.
const BEDS = [];
for (let c = 0; c < 6; c++) {
  BEDS.push({ x: 70 + c * 140, y: 70 });
  BEDS.push({ x: 70 + c * 140, y: H - 132 });
}

const canvas = document.getElementById("game");
const ctx = canvas.getContext("2d");
const overlay = document.getElementById("overlay");
const hudAwake = document.getElementById("hud-awake");
const hudTotal = document.getElementById("hud-total");
const hudScore = document.getElementById("hud-score");
const hudPower = document.getElementById("hud-power").firstElementChild;

const bannerEl = document.createElement("div");
bannerEl.className = "banner";
document.body.appendChild(bannerEl);

const toastEl = document.createElement("div");
toastEl.className = "toast";
document.body.appendChild(toastEl);

const IMG = {};
const keys = {};

let started = false;
let last = 0;
let score = 0;
let bannerShown = false;
let patients = [];
let staff = [];
let cat = null;
let effects = [];

// --- sprites -----------------------------------------------------------------

class Sprite {
  constructor(anim, scale) {
    this.t = 0;
    this.anim = null;
    this.scale = scale;
    this.set(anim, scale);
  }

  set(anim, scale) {
    if (this.anim === anim && this.scale === scale) return;
    this.anim = anim;
    this.scale = scale;
    this.t = 0;
  }

  update(dt) {
    this.t += dt;
  }

  draw(context, x, y, flip) {
    const a = this.anim;
    const img = IMG[a.sheet];
    if (!img || !img.complete || !img.naturalWidth) return;
    const frame = Math.floor(this.t * a.fps) % a.frames;
    const w = a.fw * this.scale;
    const h = a.fh * this.scale;
    context.save();
    if (flip) {
      context.translate(x + w, y);
      context.scale(-1, 1);
      context.drawImage(img, frame * a.fw, 0, a.fw, a.fh, 0, 0, w, h);
    } else {
      context.drawImage(img, frame * a.fw, 0, a.fw, a.fh, x, y, w, h);
    }
    context.restore();
  }
}

function loadImages() {
  const sheets = new Set(Object.values(ANIMS).map((a) => a.sheet));
  return Promise.all(
    [...sheets].map(
      (name) =>
        new Promise((resolve) => {
          const img = new Image();
          img.onload = () => {
            IMG[name] = img;
            resolve();
          };
          img.onerror = () => resolve();
          img.src = "/assets/" + name;
        }),
    ),
  );
}

// --- API ---------------------------------------------------------------------

function getJSON(url) {
  return fetch(url).then((r) => {
    if (!r.ok) throw new Error("HTTP " + r.status);
    return r.json();
  });
}

function postJSON(url, body) {
  return fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  }).then((r) => {
    if (!r.ok) throw new Error("HTTP " + r.status);
    return r.json();
  });
}

// --- world building ----------------------------------------------------------

const player = {
  x: W / 2 - 21,
  y: H / 2 - 21,
  speed: 220,
  dir: 1,
  sprite: new Sprite(ANIMS.costeVallenato, SCALE),
  vallenatoUntil: 0,
  cooldownUntil: 0,
};

function buildActors(ward) {
  patients = ward.patients.map((dto, i) => {
    const bed = BEDS[i % BEDS.length];
    return {
      id: dto.id,
      name: dto.name,
      level: dto.level,
      stateKey: dto.stateKey,
      x: bed.x,
      y: bed.y,
      sprite: new Sprite(ANIMS.costeAsleep, SCALE),
      wakeCooldown: 0,
      zt: Math.random() * 10,
    };
  });

  const staffAnims = {
    Doctor: { idle: ANIMS.doctorIdle, walk: ANIMS.doctorWalk },
    Orderly: { idle: ANIMS.orderlyIdle, walk: ANIMS.orderlyWalk },
  };

  staff = ward.staff.map((dto, i) => {
    const a = staffAnims[dto.role] || staffAnims.Orderly;
    return {
      id: dto.id,
      name: dto.name,
      role: dto.role,
      x: 180 + i * 260,
      y: H / 2 + 30,
      dir: 1,
      scale: SCALE,
      idleAnim: a.idle,
      walkAnim: a.walk,
      sprite: new Sprite(a.idle, SCALE),
      target: null,
      pauseUntil: 0,
    };
  });

  cat = {
    x: W / 2,
    y: H - 46,
    dir: 1,
    scale: SCALE,
    idleAnim: ANIMS.catIdle,
    walkAnim: ANIMS.catIdle,
    sprite: new Sprite(ANIMS.catIdle, SCALE),
    target: null,
    pauseUntil: 0,
  };
}

function applyWard(ward) {
  if (!ward) return;
  for (const dto of ward.patients) {
    const p = patients.find((x) => x.id === dto.id);
    if (p) p.stateKey = dto.stateKey;
  }
}

// --- actions -----------------------------------------------------------------

function doWake(p) {
  if (p.stateKey === "Awake") return;
  p.stateKey = "Awake";
  score++;
  postJSON("/api/wake", { patient: p.id })
    .then(applyWard)
    .catch(() => {
      p.stateKey = "AsleepInHallway";
      score = Math.max(0, score - 1);
      toast("Lost the ward server");
    });
}

function playVallenato() {
  const t = performance.now();
  if (t < player.cooldownUntil || !started) return;
  player.vallenatoUntil = t + POWER_DURATION;
  player.cooldownUntil = t + POWER_COOLDOWN;
  effects.push({ x: player.x + 21, y: player.y + 21, t: 0, life: 0.9, radius: POWER_RADIUS });

  window.setTimeout(() => {
    const pcx = player.x + 21;
    const pcy = player.y + 21;
    const near = patients.filter(
      (p) => p.stateKey !== "Awake" && Math.hypot(p.x + 21 - pcx, p.y + 21 - pcy) < POWER_RADIUS,
    );
    if (near.length === 0) return;
    for (const p of near) {
      p.stateKey = "Awake";
      score++;
    }
    postJSON("/api/vallenato", { patients: near.map((p) => p.id) })
      .then(applyWard)
      .catch(() => {});
  }, 280);
}

// --- update ------------------------------------------------------------------

function update(dt) {
  const now = performance.now();
  const playing = now < player.vallenatoUntil && now < player.cooldownUntil;

  let dx = 0;
  let dy = 0;
  if (started && !playing) {
    if (keys["a"] || keys["arrowleft"]) dx -= 1;
    if (keys["d"] || keys["arrowright"]) dx += 1;
    if (keys["w"] || keys["arrowup"]) dy -= 1;
    if (keys["s"] || keys["arrowdown"]) dy += 1;
  }
  const moving = dx !== 0 || dy !== 0;
  if (moving) {
    const len = Math.hypot(dx, dy) || 1;
    player.x = clamp(player.x + (dx / len) * player.speed * dt, 6, W - 48);
    player.y = clamp(player.y + (dy / len) * player.speed * dt, 6, H - 48);
    if (dx !== 0) player.dir = dx < 0 ? -1 : 1;
  }
  player.sprite.set(moving && !playing ? ANIMS.costeWalk : ANIMS.costeVallenato, SCALE);
  player.sprite.update(dt);

  for (const p of patients) {
    p.sprite.set(p.stateKey === "Awake" ? ANIMS.costeVallenato : ANIMS.costeAsleep, SCALE);
    p.sprite.update(dt);
    p.zt += dt;
  }

  for (const s of staff) wander(s, dt, 55);
  if (cat) wander(cat, dt, 26);

  if (started) {
    const pcx = player.x + 21;
    const pcy = player.y + 21;
    for (const p of patients) {
      if (p.stateKey === "Awake" || now - p.wakeCooldown < TOUCH_COOLDOWN) continue;
      if (Math.hypot(p.x + 21 - pcx, p.y + 21 - pcy) < TOUCH_RADIUS) {
        p.wakeCooldown = now;
        doWake(p);
      }
    }
  }

  for (const e of effects) e.t += dt;
  effects = effects.filter((e) => e.t < e.life);

  updateHud();
}

function wander(e, dt, speed) {
  const now = performance.now();
  if (now < e.pauseUntil) {
    e.sprite.set(e.idleAnim, e.scale);
    e.sprite.update(dt);
    return;
  }
  if (!e.target) {
    e.target = { x: 70 + Math.random() * (W - 140), y: 150 + Math.random() * (H - 260) };
  }
  const tx = e.target.x - e.x;
  const ty = e.target.y - e.y;
  const d = Math.hypot(tx, ty);
  if (d < 4) {
    e.target = null;
    e.pauseUntil = now + 700 + Math.random() * 2400;
    return;
  }
  e.x += (tx / d) * speed * dt;
  e.y += (ty / d) * speed * dt;
  if (Math.abs(tx) > 1) e.dir = tx < 0 ? -1 : 1;
  e.sprite.set(e.walkAnim, e.scale);
  e.sprite.update(dt);
}

// An awake Coste celebrates with the accordion, so the vallenato idle is the
// base pose everywhere on screen; the sleeping frame is the exception.

// --- draw --------------------------------------------------------------------

function draw() {
  drawFloor();
  drawBeds();
  for (const p of patients) drawPatient(p);
  for (const s of staff) s.sprite.draw(ctx, s.x, s.y, s.dir < 0);
  if (cat) cat.sprite.draw(ctx, cat.x, cat.y, cat.dir < 0);
  player.sprite.draw(ctx, player.x, player.y, player.dir < 0);
  drawEffects();
}

function drawFloor() {
  ctx.fillStyle = "#10132a";
  ctx.fillRect(0, 0, W, H);
  ctx.strokeStyle = "rgba(120,130,200,0.08)";
  ctx.lineWidth = 1;
  for (let x = 0; x <= W; x += 48) {
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, H);
    ctx.stroke();
  }
  for (let y = 0; y <= H; y += 48) {
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(W, y);
    ctx.stroke();
  }
  ctx.fillStyle = "rgba(255,209,102,0.05)";
  ctx.fillRect(0, H / 2 - 60, W, 120);
}

function drawBeds() {
  for (const b of BEDS) {
    ctx.fillStyle = "#1b2046";
    roundRect(b.x - 8, b.y + 4, 58, 46, 8);
    ctx.fill();
    ctx.fillStyle = "#2a3060";
    roundRect(b.x - 8, b.y + 4, 58, 12, 8);
    ctx.fill();
  }
}

function drawPatient(p) {
  p.sprite.draw(ctx, p.x, p.y, false);

  ctx.fillStyle = levelColor(p.level);
  ctx.beginPath();
  ctx.arc(p.x + 4, p.y + 4, 4, 0, Math.PI * 2);
  ctx.fill();

  if (p.stateKey !== "Awake") {
    ctx.fillStyle = "#9b5de5";
    ctx.font = "bold 14px Segoe UI";
    const bob = Math.sin(p.zt * 3) * 3;
    ctx.fillText("z", p.x + 34, p.y + 8 + bob);
    ctx.fillText("z", p.x + 42, p.y - 2 + bob);
  }

  ctx.fillStyle = "rgba(238,241,255,0.7)";
  ctx.font = "11px Segoe UI";
  ctx.textAlign = "center";
  ctx.fillText(shortName(p.name), p.x + 21, p.y + 58);
  ctx.textAlign = "left";
}

function drawEffects() {
  for (const e of effects) {
    const k = e.t / e.life;
    ctx.strokeStyle = `rgba(255,209,102,${(1 - k) * 0.85})`;
    ctx.lineWidth = 3;
    ctx.beginPath();
    ctx.arc(e.x, e.y, e.radius * k, 0, Math.PI * 2);
    ctx.stroke();
  }
}

// --- HUD ---------------------------------------------------------------------

function updateHud() {
  const awake = patients.filter((p) => p.stateKey === "Awake").length;
  hudAwake.textContent = awake;
  hudTotal.textContent = patients.length;
  hudScore.textContent = score;

  const remain = Math.max(0, player.cooldownUntil - performance.now());
  const pct = remain > 0 ? (1 - remain / POWER_COOLDOWN) * 100 : 100;
  hudPower.style.width = pct.toFixed(0) + "%";

  if (patients.length > 0 && awake === patients.length) {
    if (!bannerShown) {
      bannerShown = true;
      bannerEl.textContent = "Ward fully awake!";
      bannerEl.classList.add("show");
    }
  } else if (bannerShown) {
    bannerShown = false;
    bannerEl.classList.remove("show");
  }
}

function toast(message) {
  toastEl.textContent = message;
  toastEl.classList.add("show");
  window.clearTimeout(toast.timer);
  toast.timer = window.setTimeout(() => toastEl.classList.remove("show"), 2400);
}

// --- helpers -----------------------------------------------------------------

function clamp(v, min, max) {
  return Math.max(min, Math.min(max, v));
}

function shortName(name) {
  return name.split(" ")[0];
}

function levelColor(level) {
  if (level === "Severe") return "#ef476f";
  if (level === "Moderate") return "#ffb703";
  return "#06d6a0";
}

function roundRect(x, y, w, h, r) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}

// --- loop and input ----------------------------------------------------------

function loop(ts) {
  const dt = last ? Math.min(0.05, (ts - last) / 1000) : 0;
  last = ts;
  update(dt);
  draw();
  requestAnimationFrame(loop);
}

window.addEventListener("keydown", (event) => {
  const key = event.key.toLowerCase();
  keys[key] = true;
  if (key === " " || key === "spacebar") {
    if (started && !event.repeat) playVallenato();
    event.preventDefault();
  }
  if (["arrowup", "arrowdown", "arrowleft", "arrowright"].includes(key)) {
    event.preventDefault();
  }
});

window.addEventListener("keyup", (event) => {
  keys[event.key.toLowerCase()] = false;
});

document.getElementById("start").addEventListener("click", () => {
  started = true;
  overlay.classList.add("hidden");
});

async function boot() {
  await loadImages();
  try {
    const ward = await getJSON("/api/ward");
    buildActors(ward);
  } catch (err) {
    toast("Could not reach the ward server");
  }
  requestAnimationFrame(loop);
  window.setInterval(() => {
    getJSON("/api/ward").then(applyWard).catch(() => {});
  }, 1500);
}

boot();
