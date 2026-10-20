"use strict";

// Live moderation panel for the hospital. The Go API owns the model and the
// mutex, so this file only: (1) reads /api/state, (2) posts actions, (3) renders
// the result. It performs no validation of its own — whatever error the model
// raises is displayed as-is, so the business rules stay in the hospital package.

const API = {
  state() {
    return fetch("/api/state").then((r) => r.json());
  },
  action(body) {
    return fetch("/api/action", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }).then(async (r) => {
      const data = await r.json();
      if (!r.ok) throw new Error(data.error || "HTTP " + r.status);
      return data;
    });
  },
};

const HALLWAY_LOCATIONS = [
  "pasillo 2, segundo piso",
  "sala de espera",
  "cafetería",
  "pasillo de urgencias",
  "fila de radiología",
];

// One PNG strip per animation; frames sit side by side with transparent gaps.
// Coste/doctor/orderly are 14x14 frames, the cat is 10x8. Durations are slow on
// purpose: idle animations read better when they breathe.
const SPRITES = {
  coste: {
    blink: { sheet: "Coste-idle1-eyes.png", frames: 2, dur: "1.6s" },
    vallenato: { sheet: "Coste-idle2-acordeon.png", frames: 23, dur: "2.6s" },
    walk: { sheet: "Coste-walk.png", frames: 4, dur: "0.6s" },
    asleep: { sheet: "Coste-asleep.png", frames: 10, dur: "2.2s" },
  },
  doctor: {
    idle: { sheet: "Doctor-idle.png", frames: 2, dur: "1.8s" },
    walk: { sheet: "Doctor-walk.png", frames: 4, dur: "0.6s" },
  },
  orderly: {
    idle: { sheet: "Orderly-idle.png", frames: 2, dur: "1.8s" },
    walk: { sheet: "Orderly-walk.png", frames: 4, dur: "0.6s" },
  },
  cat: {
    idle: { sheet: "Cat-idle.png", frames: 4, dur: "2s" },
  },
};

const WALK_MS = 1000;
const MAX_CARE = 4; // a Doctor holds at most 4 patients under care (hospital model)

let state = null;
let lastHistoryLength = -1;

const cards = new Map();
const prevKey = new Map();
const walkUntil = new Map();
const staffCards = new Map();
const staffWalkUntil = new Map();

// --- helpers -----------------------------------------------------------------

function findPatient(id) {
  return state ? state.patients.find((p) => p.id === id) : null;
}

function staffById(id) {
  return state ? state.staff.find((s) => s.id === id) : null;
}

function pickLocation() {
  return HALLWAY_LOCATIONS[Math.floor(Math.random() * HALLWAY_LOCATIONS.length)];
}

function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text != null) node.textContent = text;
  return node;
}

function button(label, fn, className) {
  const b = el("button", "small" + (className ? " " + className : ""), label);
  b.addEventListener("click", fn);
  return b;
}

// --- sprite helpers ----------------------------------------------------------

function setAnim(node, anim) {
  if (node.dataset.sheet === anim.sheet && node.dataset.frames === String(anim.frames)) return;
  node.dataset.sheet = anim.sheet;
  node.dataset.frames = String(anim.frames);
  node.style.setProperty("--sheet", `url("/assets/${anim.sheet}")`);
  node.style.setProperty("--frames", String(anim.frames));
  node.style.setProperty("--dur", anim.dur);
  if (!node.style.getPropertyValue("--delay")) {
    // A negative delay starts the loop mid-way, so idles never run in sync.
    const seconds = Number.parseFloat(anim.dur);
    node.style.setProperty("--delay", `-${(Math.random() * seconds).toFixed(2)}s`);
  }
}

function newSprite(extraClass) {
  return el("div", extraClass ? "sprite " + extraClass : "sprite");
}

// --- rendering ---------------------------------------------------------------

function render(data) {
  state = data;
  document.getElementById("hospital-name").textContent = data.name;
  renderStats(data);
  renderRooms(data);
  renderPatients(data);
  renderStaff(data);
  renderHistory(data);
  renderQueries(data);
  renderDecor();
  detectWalks(data);
}

function renderStats(data) {
  const awakeCount = data.patients.filter((p) => p.stateKey === "Awake").length;
  const stats = [
    { value: data.patients.length, label: "Patients" },
    { value: awakeCount, label: "Awake" },
    { value: data.staff.length, label: "Staff" },
    { value: data.doctors.length, label: "Doctors" },
    { value: data.rooms.length, label: "Rooms" },
    { value: data.hallway.length, label: "In hallway" },
    { value: data.history.length, label: "Episodes" },
  ];
  const wrap = document.getElementById("stats");
  wrap.textContent = "";
  for (const s of stats) {
    const card = el("div", "stat");
    card.append(el("div", "value", String(s.value)), el("div", "label", s.label));
    wrap.appendChild(card);
  }
}

function renderPatients(data) {
  const wrap = document.getElementById("patients");
  const seen = new Set();
  for (const p of data.patients) {
    seen.add(p.id);
    let card = cards.get(p.id);
    if (!card) {
      card = createPatientCard(p);
      cards.set(p.id, card);
    }
    updatePatientCard(card, p);
  }
  for (const [id, card] of cards) {
    if (!seen.has(id)) {
      stopIdle(card);
      card.el.remove();
      cards.delete(id);
      prevKey.delete(id);
      walkUntil.delete(id);
    }
  }
}

function createPatientCard() {
  const el2 = el("div", "patient");
  const sprite = newSprite();
  const info = el("div");
  const name = el("div", "name");
  const meta = el("div", "meta");
  const actions = el("div", "actions");
  info.append(name, meta, actions);
  el2.append(sprite, info);
  return { el: el2, sprite, name, meta, actions, idleTimer: null };
}

function updatePatientCard(card, p) {
  const previous = prevKey.get(p.id);
  if (previous !== undefined && previous !== p.stateKey) {
    walkUntil.set(p.id, Date.now() + WALK_MS);
  }
  prevKey.set(p.id, p.stateKey);

  const walking = (walkUntil.get(p.id) || 0) > Date.now();
  if (walking) {
    stopIdle(card);
    card.sprite.classList.remove("sleep");
    setAnim(card.sprite, SPRITES.coste.walk);
  } else if (p.stateKey === "Awake") {
    card.sprite.classList.remove("sleep");
    startIdle(card);
  } else {
    stopIdle(card);
    card.sprite.classList.add("sleep");
    setAnim(card.sprite, SPRITES.coste.asleep);
  }

  card.name.textContent = p.id + " · " + p.name;

  const badge = el("span", "badge " + p.level, p.level);
  const pill = el("span", "state-pill " + p.stateKey, p.state);
  const where = p.room != null ? "room " + p.room : p.location || "—";
  const doctor = p.doctor ? " · Dr. " + p.doctor : " · no doctor";
  card.meta.textContent = "";
  card.meta.append(
    badge,
    document.createTextNode(" "),
    pill,
    document.createTextNode(` · ${where} · age ${p.age} · ${p.episodes} episode(s)${doctor}`),
  );

  renderPatientActions(card.actions, p);
}

function renderPatientActions(actions, p) {
  actions.textContent = "";
  actions.append(
    button("Sleep", () => run({ type: "sleep", patient: p.id, location: pickLocation() })),
    button("Wake", () => run({ type: "wake", patient: p.id })),
    button("Assign room", () => run({ type: "assign-room", patient: p.id })),
    button("Dispatch", () => {
      const current = findPatient(p.id);
      run({ type: "dispatch", patient: p.id, location: (current && current.location) || "hallway" });
    }),
    button("Diagnose", () => run({ type: "diagnose", patient: p.id })),
    button("Attend sleep", () => run({ type: "attend-sleep", patient: p.id })),
  );
}

// An awake Coste mostly plays the vallenato (accordion) idle and blinks now and
// then, so both authored idle loops are visible.
function startIdle(card) {
  if (card.idleTimer) return;
  const playVallenato = () => {
    setAnim(card.sprite, SPRITES.coste.vallenato);
    card.idleTimer = window.setTimeout(() => {
      setAnim(card.sprite, SPRITES.coste.blink);
      card.idleTimer = window.setTimeout(playVallenato, 2000 + Math.random() * 2200);
    }, 3200 + Math.random() * 2600);
  };
  playVallenato();
}

function stopIdle(card) {
  if (card.idleTimer) {
    window.clearTimeout(card.idleTimer);
    card.idleTimer = null;
  }
}

function renderStaff(data) {
  const wrap = document.getElementById("staff");
  const seen = new Set();
  for (const s of data.staff) {
    seen.add(s.id);
    let card = staffCards.get(s.id);
    if (!card) {
      card = createStaffCard();
      staffCards.set(s.id, card);
    }
    updateStaffCard(card, s, data);
  }
  for (const [id, card] of staffCards) {
    if (!seen.has(id)) {
      card.el.remove();
      staffCards.delete(id);
      staffWalkUntil.delete(id);
    }
  }
}

function createStaffCard() {
  const el2 = el("div", "member");
  const sprite = newSprite();
  const info = el("div");
  const name = el("div", "name");
  const meta = el("div", "meta");
  const actions = el("div", "actions");
  const episodes = el("div", "episodes");
  info.append(name, meta, actions, episodes);
  el2.append(sprite, info);
  return { el: el2, sprite, name, meta, actions, episodes, picker: null };
}

function updateStaffCard(card, s, data) {
  const set = s.role === "Doctor" ? SPRITES.doctor : SPRITES.orderly;
  const walking = (staffWalkUntil.get(s.id) || 0) > Date.now();
  setAnim(card.sprite, walking ? set.walk : set.idle);
  card.name.textContent = s.id + " · " + s.name;
  const extra = s.specialty ? " · " + s.specialty : "";
  const load = s.role === "Doctor" && s.underCare ? s.underCare.length + "/" + MAX_CARE + " under care · " : "";
  card.meta.textContent = s.role + " · age " + s.age + extra + " · " + load + s.episodes.length + " episode(s)";

  renderStaffActions(card, s, data);

  card.episodes.textContent = "";
  for (const e of s.episodes) {
    const row = el("div", "episode");
    row.append(el("span", "time", e.time), document.createTextNode(e.summary));
    card.episodes.appendChild(row);
  }
}

// The attender is an actor: the panel picks a patient and makes this specific
// staff member act. "Attend patient" goes through Attender.Attend (doctor and
// orderly keep their own rule: only a sleeping patient is an emergency), while
// the doctor-only buttons expose DiagnosePatient and AttendSleepEmergency. All of
// them return the model's verdict unchanged; the panel only renders it.
function renderStaffActions(card, s, data) {
  const actions = card.actions;
  actions.textContent = "";

  const picker = el("select", "patient-picker");
  const previous = card.picker ? card.picker.value : "";
  for (const p of data.patients) {
    const option = el("option", null, p.id + " · " + p.name);
    option.value = p.id;
    picker.appendChild(option);
  }
  if (previous) picker.value = previous;
  card.picker = picker;

  const selected = () => picker.value;
  const location = () => {
    const current = findPatient(selected());
    return (current && current.location) || "hallway";
  };

  const buttons = [
    button("Attend patient", () =>
      run({ type: "attend", patient: selected(), staff: s.id, location: location() }),
    ),
  ];
  if (s.role === "Doctor") {
    buttons.push(
      button("Diagnose", () => run({ type: "diagnose", patient: selected(), doctor: s.id })),
      button("Sleep emergency", () => run({ type: "attend-sleep", patient: selected(), doctor: s.id })),
    );
  }

  actions.append(picker, ...buttons);
}

// When a new episode appears, the attender that just responded walks to it.
function detectWalks(data) {
  if (lastHistoryLength >= 0 && data.history.length > lastHistoryLength) {
    const latest = data.history[data.history.length - 1];
    staffWalkUntil.set(latest.attender, Date.now() + WALK_MS + 400);
  }
  lastHistoryLength = data.history.length;
}

function renderDecor() {
  const wrap = document.getElementById("decor");
  if (wrap.childElementCount) return;
  for (let i = 0; i < 2; i++) {
    const cat = newSprite("cat");
    cat.style.setProperty("--scale", i === 1 ? "3" : "2");
    setAnim(cat, SPRITES.cat.idle);
    wrap.appendChild(cat);
  }
}

function renderRooms(data) {
  const wrap = document.getElementById("rooms");
  const byId = new Map(data.patients.map((p) => [p.id, p]));
  wrap.textContent = "";
  for (const r of data.rooms) {
    const room = el("div", "room" + (r.occupants.length ? " occupied" : ""));
    room.append(el("div", "room-head", "Room " + r.number));
    room.append(el("div", "room-meta", r.state + " · " + r.occupants.length + "/" + r.capacity));

    for (const id of r.occupants) {
      const row = el("div", "occupant");
      const sprite = newSprite("sleep");
      setAnim(sprite, SPRITES.coste.asleep);
      const p = byId.get(id);
      row.append(
        sprite,
        el("span", "room-meta", p ? p.name : id),
        button("Wake", () => run({ type: "wake", patient: id })),
      );
      room.appendChild(row);
    }
    wrap.appendChild(room);
  }
}

function renderHistory(data) {
  const wrap = document.getElementById("history");
  wrap.textContent = "";
  if (data.history.length === 0) {
    wrap.appendChild(el("div", "empty", "No episodes registered yet."));
    return;
  }
  const table = el("table");
  const head = el("tr");
  for (const h of ["ID", "Time", "Patient", "Attender", "Location", "Room", "Summary"]) {
    head.appendChild(el("th", null, h));
  }
  table.appendChild(head);

  const body = el("tbody");
  const byId = new Map(data.patients.map((p) => [p.id, p]));
  for (const e of data.history) {
    const tr = el("tr");
    const p = byId.get(e.patient);
    tr.append(
      el("td", "mono", e.id),
      el("td", "mono", e.time),
      el("td", null, p ? p.name : e.patient),
      el("td", null, e.attender),
      el("td", null, e.location),
      el("td", null, e.room != null ? "room " + e.room : "—"),
      el("td", "wrap", e.summary),
    );
    body.appendChild(tr);
  }
  table.appendChild(body);
  wrap.appendChild(table);
}

function renderQueries(data) {
  const byId = new Map(data.patients.map((p) => [p.id, p]));

  fillList(
    "q-hallway",
    data.hallway.map((id) => {
      const p = byId.get(id);
      return p ? p.name + " (" + p.id + ")" : id;
    }),
  );

  // 5.2 — episodes grouped per doctor.
  const history = document.getElementById("q-history");
  history.textContent = "";
  for (const d of data.doctors) {
    history.appendChild(el("div", "meta", d.name + " · " + d.specialty));
    if (d.episodes.length === 0) {
      history.appendChild(el("div", "empty", "No episodes yet"));
      continue;
    }
    for (const e of d.episodes) {
      const row = el("div", "episode");
      row.append(el("span", "time", e.time), document.createTextNode(e.summary));
      history.appendChild(row);
    }
  }

  // 5.3 — bed availability, room by room.
  const rooms = document.getElementById("q-rooms");
  rooms.textContent = "";
  const available = data.rooms.filter((r) => r.occupants.length < r.capacity).length;
  rooms.appendChild(el("div", "meta", available + " of " + data.rooms.length + " room(s) free"));
  for (const r of data.rooms) {
    const free = r.capacity - r.occupants.length;
    rooms.appendChild(el("div", "episode", "Room " + r.number + ": " + (free > 0 ? free + " free" : "full")));
  }

  // 5.4 — severity report.
  fillList(
    "q-severe",
    data.severeReport.map((s) => s.name + " — " + s.count + " episode(s)"),
  );
}

function fillList(id, items) {
  const list = document.getElementById(id);
  list.textContent = "";
  if (items.length === 0) {
    list.appendChild(el("li", "empty", "None"));
    return;
  }
  for (const text of items) {
    list.appendChild(el("li", null, text));
  }
}

// --- actions -----------------------------------------------------------------

async function run(action) {
  try {
    render(await API.action(action));
    toast("OK", true);
  } catch (err) {
    toast(err.message);
  }
}

async function refresh() {
  try {
    render(await API.state());
  } catch (err) {
    toast("Lost connection to the server: " + err.message);
  }
}

function toast(message, ok) {
  const node = document.getElementById("toast");
  node.textContent = message;
  node.classList.toggle("ok", !!ok);
  node.classList.add("show");
  window.clearTimeout(toast.timer);
  toast.timer = window.setTimeout(() => node.classList.remove("show"), 2400);
}

// --- tabs --------------------------------------------------------------------

function initTabs() {
  const tabs = document.querySelectorAll("#tabs .tab");
  const panels = document.querySelectorAll(".tab-panel");
  for (const tab of tabs) {
    tab.addEventListener("click", () => {
      for (const t of tabs) t.classList.toggle("active", t === tab);
      for (const panel of panels) {
        panel.classList.toggle("active", panel.dataset.panel === tab.dataset.tab);
      }
    });
  }
}

// --- wiring ------------------------------------------------------------------

function wireForms() {
  document.getElementById("admit-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const form = event.target;
    run({ type: "admit", name: form.name.value, age: Number(form.age.value), level: form.level.value });
    form.reset();
  });

  document.getElementById("doctor-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const form = event.target;
    run({ type: "hire-doctor", name: form.name.value, age: Number(form.age.value), specialty: form.specialty.value });
    form.reset();
  });

  document.getElementById("orderly-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const form = event.target;
    run({ type: "hire-orderly", name: form.name.value, age: Number(form.age.value) });
    form.reset();
  });

  document.getElementById("room-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const form = event.target;
    run({ type: "register-room", number: Number(form.number.value), capacity: Number(form.capacity.value) });
    form.reset();
  });

  document.getElementById("attend-hallway").addEventListener("click", () => run({ type: "attend-hallway" }));

  document.getElementById("reset").addEventListener("click", () => {
    cards.forEach(stopIdle);
    cards.forEach((card) => card.el.remove());
    cards.clear();
    prevKey.clear();
    walkUntil.clear();
    staffCards.forEach((card) => card.el.remove());
    staffCards.clear();
    staffWalkUntil.clear();
    lastHistoryLength = -1;
    run({ type: "reset" });
  });
}

initTabs();
wireForms();
refresh();
window.setInterval(refresh, 2500);
