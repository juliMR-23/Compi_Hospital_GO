"use strict";

// Small REST client + renderer for the hospital viewer. The API on the Go side
// owns a mutex, so every call here is safe even while the page polls.

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
];

let state = null;
const cards = new Map(); // patient id -> { el, sprite, name, meta }
const prevKey = new Map(); // patient id -> last stateKey
const walkUntil = new Map(); // patient id -> timestamp until which Walk plays

function findPatient(id) {
  return state ? state.patients.find((p) => p.id === id) : null;
}

function pickLocation() {
  return HALLWAY_LOCATIONS[Math.floor(Math.random() * HALLWAY_LOCATIONS.length)];
}

// --- rendering ---------------------------------------------------------------

function render(data) {
  state = data;
  document.getElementById("hospital-name").textContent = data.name;
  renderRooms(data);
  renderPatients(data);
  renderQueries(data);
}

function spriteClass(p) {
  if ((walkUntil.get(p.id) || 0) > Date.now()) return "walk";
  if (p.stateKey === "Awake") return "idle";
  return "sleep";
}

function applySprite(card, cls) {
  if (card.sprite.dataset.anim === cls) return;
  card.sprite.dataset.anim = cls;
  card.sprite.className = "sprite " + cls;
}

function renderPatients(data) {
  const wrap = document.getElementById("patients");
  const seen = new Set();
  for (const p of data.patients) {
    seen.add(p.id);
    let card = cards.get(p.id);
    if (!card) {
      card = createCard(p);
      cards.set(p.id, card);
      wrap.appendChild(card.el);
    }
    updateCard(card, p);
  }
  for (const [id, card] of cards) {
    if (!seen.has(id)) {
      card.el.remove();
      cards.delete(id);
      prevKey.delete(id);
      walkUntil.delete(id);
    }
  }
}

function createCard(p) {
  const el = document.createElement("div");
  el.className = "patient";

  const sprite = document.createElement("div");
  sprite.className = "sprite";

  const info = document.createElement("div");
  const name = document.createElement("div");
  name.className = "name";
  const meta = document.createElement("div");
  meta.className = "meta";
  const actions = document.createElement("div");
  actions.className = "actions";

  const button = (label, fn) => {
    const b = document.createElement("button");
    b.className = "small";
    b.textContent = label;
    b.addEventListener("click", fn);
    return b;
  };

  actions.append(
    button("Sleep", () => run({ type: "sleep", patient: p.id, location: pickLocation() })),
    button("Wake", () => run({ type: "wake", patient: p.id })),
    button("Assign room", () => run({ type: "assign-room", patient: p.id })),
    button("Dispatch", () => {
      const current = findPatient(p.id);
      run({ type: "dispatch", patient: p.id, location: (current && current.location) || "hallway" });
    }),
  );

  info.append(name, meta, actions);
  el.append(sprite, info);
  return { el, sprite, name, meta };
}

function updateCard(card, p) {
  const previous = prevKey.get(p.id);
  if (previous !== undefined && previous !== p.stateKey) {
    // The patient just changed place: play Walk for a moment before settling.
    walkUntil.set(p.id, Date.now() + 900);
    window.setTimeout(() => {
      const latest = findPatient(p.id);
      if (latest) updateCard(card, latest);
    }, 950);
  }
  prevKey.set(p.id, p.stateKey);

  applySprite(card, spriteClass(p));

  card.name.textContent = p.id + " · " + p.name;

  const badge = document.createElement("span");
  badge.className = "badge " + p.level;
  badge.textContent = p.level;

  const where = p.room != null ? "room " + p.room : p.location || "—";
  card.meta.textContent = "";
  card.meta.append(
    badge,
    document.createTextNode(" " + p.state + " · " + where + " · age " + p.age),
  );
}

function renderRooms(data) {
  const wrap = document.getElementById("rooms");
  const byId = new Map(data.patients.map((p) => [p.id, p]));
  wrap.textContent = "";
  for (const r of data.rooms) {
    const el = document.createElement("div");
    el.className = "room" + (r.occupants.length ? " occupied" : "");

    const head = document.createElement("div");
    head.className = "room-head";
    head.textContent = "Room " + r.number;

    const meta = document.createElement("div");
    meta.className = "room-meta";
    meta.textContent = r.state + " · " + r.occupants.length + "/" + r.capacity;

    el.append(head, meta);

    for (const id of r.occupants) {
      const sprite = document.createElement("div");
      sprite.className = "sprite sleep";
      sprite.style.margin = "0 auto";
      el.appendChild(sprite);

      const label = document.createElement("div");
      label.className = "room-meta";
      const p = byId.get(id);
      label.textContent = p ? p.name : id;
      el.appendChild(label);
    }

    wrap.appendChild(el);
  }
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

  const history = document.getElementById("q-history");
  history.textContent = "";
  for (const d of data.doctors) {
    const title = document.createElement("div");
    title.className = "meta";
    title.textContent = d.name + " · " + d.specialty;
    history.appendChild(title);

    if (d.episodes.length === 0) {
      const empty = document.createElement("div");
      empty.className = "empty";
      empty.textContent = "No episodes yet";
      history.appendChild(empty);
      continue;
    }
    for (const e of d.episodes) {
      const row = document.createElement("div");
      row.className = "episode";
      const time = document.createElement("span");
      time.className = "time";
      time.textContent = e.time;
      row.append(time, document.createTextNode(e.summary));
      history.appendChild(row);
    }
  }

  fillList(
    "q-severe",
    data.severeReport.map((s) => s.name + " — " + s.count + " episode(s)"),
  );
}

function fillList(id, items) {
  const list = document.getElementById(id);
  list.textContent = "";
  if (items.length === 0) {
    const li = document.createElement("li");
    li.className = "empty";
    li.textContent = "None";
    list.appendChild(li);
    return;
  }
  for (const text of items) {
    const li = document.createElement("li");
    li.textContent = text;
    list.appendChild(li);
  }
}

// --- actions -----------------------------------------------------------------

async function run(action) {
  try {
    render(await API.action(action));
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

function toast(message) {
  const el = document.getElementById("toast");
  el.textContent = message;
  el.classList.add("show");
  window.clearTimeout(toast.timer);
  toast.timer = window.setTimeout(() => el.classList.remove("show"), 2600);
}

// --- wiring ------------------------------------------------------------------

document.getElementById("admit-form").addEventListener("submit", (event) => {
  event.preventDefault();
  const form = event.target;
  run({
    type: "admit",
    name: form.name.value,
    age: Number(form.age.value),
    level: form.level.value,
  });
  form.reset();
});

document.getElementById("attend-hallway").addEventListener("click", () => {
  run({ type: "attend-hallway" });
});

document.getElementById("reset").addEventListener("click", () => {
  cards.forEach((card) => card.el.remove());
  cards.clear();
  prevKey.clear();
  walkUntil.clear();
  run({ type: "reset" });
});

refresh();
window.setInterval(refresh, 2500);
