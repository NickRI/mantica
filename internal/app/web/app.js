import * as maplibregl from "/vendor/maplibre-gl/maplibre-gl.mjs";
import { layers as protomapsLayers, namedFlavor } from "/vendor/protomaps-basemaps/basemaps.mjs";

// MapLibre sprite loader requires an absolute URL (new URL without base throws).
const PROTOMAPS_ASSETS = new URL("/vendor/protomaps-basemaps-assets", window.location.origin).href;
const PROTOMAPS_LAYER_HINTS = ["earth", "roads", "buildings", "water", "landuse", "places", "pois"];

function isProtomaps(tj) {
  const ids = new Set((tj.vector_layers || []).map((l) => l.id));
  let hits = 0;
  for (const id of PROTOMAPS_LAYER_HINTS) {
    if (ids.has(id)) hits++;
  }
  return hits >= 4;
}
const I18N = {
  ru: {
    brand: "Mantica",
    title: "Mantica",
    tab_maps: "Карты",
    tab_download: "Загрузка",
    tab_catalog: "Каталог",
    tab_settings: "Настройки",
    search_placeholder: "Поиск по имени, формату, id…",
    catalog_search_placeholder: "Поиск по имени…",
    url_label: "URL файла .mbtiles / .pmtiles",
    name_label: "Имя файла (необязательно)",
    download_btn: "Скачать",
    redownload_btn: "Перекачать",
    downloading_btn: "Качается…",
    resume_btn: "Продолжить",
    clear_completed: "Очистить завершённые",
    disk_usage: "Диск: свободно {free} из {total}",
    maps_usage: "Карты · {used}",
    maps_other: "Остальное",
    jobs: "Задачи",
    catalog: "Каталог",
    servers: "Удалённые серверы",
    server_url: "URL mbtileserver",
    server_name: "Название",
    add_server: "Добавить сервер",
    language: "Язык интерфейса",
    settings_hint: "Язык и лимиты сохраняются на сервере. Карты лежат в каталоге tilesets.",
    empty: "Выберите карту в меню слева",
    no_maps: "Пока нет локальных карт",
    map_broken: "Битый файл",
    map_open_broken: "Карта повреждена и не может быть открыта",
    no_jobs: "Нет загрузок",
    no_catalog: "Ничего не найдено",
    no_servers: "Серверы не добавлены",
    delete: "Удалить",
    cancel: "Отменить",
    checksum_label: "Checksum (необязательно)",
    rate_limit: "Лимит скорости (байт/с, 0 = без лимита)",
    save_settings: "Сохранить",
    zoom: "зум",
    remote: "удалённая",
    open: "Открыть",
    tool_theme: "Светлая / тёмная тема",
    palette_label: "Цветовая тема",
    appearance_label: "Оформление",
    mode_dark: "Тёмная",
    mode_light: "Светлая",
    theme_hint: "Тема хранится только в браузере.",
    palette_atlas: "Атлас",
    palette_slate: "Сланец",
    palette_forest: "Лес",
    palette_ink: "Чернила",
    palette_copper: "Медь",
    palette_mist: "Туман",
    geo_empty: "Ничего не найдено",
    geo_error: "Поиск недоступен",
    geo_locate_error: "Не удалось определить местоположение",
    geo_locating: "Определяется местоположение…",
    geo_placeholder: "Адрес, место…",
    geocoders_label: "Геокодеры",
    geocoders_hint: "Порядок сверху вниз — приоритет. При ошибке пробуется следующий. Ключи API — в sops (mantica/geocoder-keys).",
    geocoder_enabled: "Вкл",
    geocoder_url: "Base URL",
    geocoder_test: "Проверить",
    geocoder_has_key: "ключ",
    geocoder_no_key: "без ключа",
    geocoder_ok: "ok",
    geocoder_fail: "fail",
    geocoder_idle: "—",
    reverse_here: "Адрес",
    verify_btn: "Проверить",
    verifying_btn: "Проверка…",
    hash_ok: "Хеш ок",
    hash_bad: "Хеш сломан",
    hash_error: "Ошибка хеша",
    hash_mismatch_hint: "Ожидался {expected}, получено {actual}",
  },
  en: {
    brand: "Mantica",
    title: "Mantica",
    tab_maps: "Maps",
    tab_download: "Download",
    tab_catalog: "Catalog",
    tab_settings: "Settings",
    search_placeholder: "Search by name, format, id…",
    catalog_search_placeholder: "Search by name…",
    url_label: "URL of .mbtiles / .pmtiles file",
    name_label: "Filename (optional)",
    download_btn: "Download",
    redownload_btn: "Re-download",
    downloading_btn: "Downloading…",
    resume_btn: "Continue",
    clear_completed: "Clear completed",
    disk_usage: "Disk: {free} free of {total}",
    maps_usage: "Maps · {used}",
    maps_other: "Other",
    jobs: "Jobs",
    catalog: "Catalog",
    servers: "Remote servers",
    server_url: "mbtileserver URL",
    server_name: "Name",
    add_server: "Add server",
    language: "Interface language",
    settings_hint: "Language and limits are stored on the server. Maps live in the tilesets directory.",
    empty: "Pick a map from the left menu",
    no_maps: "No local maps yet",
    map_broken: "Broken file",
    map_open_broken: "This map is damaged and cannot be opened",
    no_jobs: "No downloads",
    no_catalog: "Nothing found",
    no_servers: "No servers added",
    delete: "Delete",
    cancel: "Cancel",
    checksum_label: "Checksum (optional)",
    rate_limit: "Rate limit (bytes/s, 0 = unlimited)",
    save_settings: "Save",
    zoom: "zoom",
    remote: "remote",
    open: "Open",
    tool_theme: "Light / dark theme",
    palette_label: "Color theme",
    appearance_label: "Appearance",
    mode_dark: "Dark",
    mode_light: "Light",
    theme_hint: "Theme is stored only in this browser.",
    palette_atlas: "Atlas",
    palette_slate: "Slate",
    palette_forest: "Forest",
    palette_ink: "Ink",
    palette_copper: "Copper",
    palette_mist: "Mist",
    geo_placeholder: "Address, place…",
    geo_empty: "Nothing found",
    geo_error: "Search unavailable",
    geo_locate_error: "Could not determine your location",
    geo_locating: "Determining your location…",
    geocoders_label: "Geocoders",
    geocoders_hint: "Top to bottom is priority. On error the next one is tried. API keys live in sops (mantica/geocoder-keys).",
    geocoder_enabled: "On",
    geocoder_url: "Base URL",
    geocoder_test: "Test",
    geocoder_has_key: "key",
    geocoder_no_key: "no key",
    geocoder_ok: "ok",
    geocoder_fail: "fail",
    geocoder_idle: "—",
    reverse_here: "Address",
    verify_btn: "Verify",
    verifying_btn: "Verifying…",
    hash_ok: "Hash ok",
    hash_bad: "Hash broken",
    hash_error: "Hash error",
    hash_mismatch_hint: "Expected {expected}, got {actual}",
  },
};

const blankStyle = {
  version: 8,
  sources: {},
  layers: [{ id: "bg", type: "background", paint: { "background-color": "#16130f" } }],
};

const THEME_KEY = "mantica-ui-theme";
const PALETTE_KEY = "mantica-ui-palette";
const PALETTES = [
  { id: "atlas", dark: "#e8b848", light: "#16130f" },
  { id: "slate", dark: "#5ab4f0", light: "#12151a" },
  { id: "forest", dark: "#78d25a", light: "#121612" },
  { id: "ink", dark: "#64a0f0", light: "#0f141c" },
  { id: "copper", dark: "#f0963c", light: "#1a1410" },
  { id: "mist", dark: "#46c8be", light: "#121616" },
];

function readPalette() {
  const saved = localStorage.getItem(PALETTE_KEY);
  return PALETTES.some((p) => p.id === saved) ? saved : "atlas";
}

const state = {
  lang: "ru",
  theme: localStorage.getItem(THEME_KEY) === "light" ? "light" : "dark",
  palette: readPalette(),
  maps: [],
  catalog: [],
  jobs: [],
  storage: null,
  geocoders: [],
  active: null,
  activeTileJSON: null,
  geoMarker: null,
  hashCamera: null,
};

const t = (key) => I18N[state.lang][key];

function mapBg() {
  return getComputedStyle(document.documentElement).getPropertyValue("--map-bg").trim() || "#16130f";
}

function syncThemeControls() {
  document.querySelectorAll(".palette-btn").forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.palette === state.palette);
  });
  $("mode-dark")?.classList.toggle("active", state.theme === "dark");
  $("mode-light")?.classList.toggle("active", state.theme === "light");
}

function applyTheme(opts = {}) {
  const reloadMap = opts.reloadMap === true;
  document.documentElement.dataset.theme = state.theme;
  document.documentElement.dataset.palette = state.palette;
  localStorage.setItem(THEME_KEY, state.theme);
  localStorage.setItem(PALETTE_KEY, state.palette);
  blankStyle.layers[0].paint["background-color"] = mapBg();
  if (typeof map !== "undefined" && map.getLayer?.("bg")) {
    map.setPaintProperty("bg", "background-color", mapBg());
  }
  syncThemeControls();
  if (reloadMap) refreshActiveStyle();
}

function snapshotCamera() {
  const c = map.getCenter();
  return {
    center: [c.lng, c.lat],
    zoom: map.getZoom(),
    bearing: map.getBearing(),
    pitch: map.getPitch(),
  };
}

function refreshActiveStyle() {
  if (!state.activeTileJSON || typeof map === "undefined") return;
  const tj = state.activeTileJSON;
  if (!isProtomaps(tj)) return;
  const cam = snapshotCamera();
  const onLoad = () => {
    map.jumpTo(cam);
    requestAnimationFrame(() => map.jumpTo(cam));
    if (state.geoMarker) {
      const ll = state.geoMarker.getLngLat();
      const text = state.geoMarker.getPopup()?.getElement()?.textContent || $("geo-q")?.value || "";
      state.geoMarker.remove();
      state.geoMarker = new maplibregl.Marker({
        color: getComputedStyle(document.documentElement).getPropertyValue("--gold").trim() || "#e8b848",
      })
        .setLngLat(ll)
        .setPopup(text ? new maplibregl.Popup().setText(text) : undefined)
        .addTo(map);
    }
  };
  map.once("style.load", onLoad);
  map.setStyle(styleFromTileJSON(tj));
}

function renderPalettes() {
  const root = $("palette-list");
  if (!root) return;
  root.innerHTML = PALETTES.map(
    (p) => `<button class="palette-btn${p.id === state.palette ? " active" : ""}" type="button" data-palette="${p.id}">
      <span class="palette-swatch" style="--swatch-a:${p.dark};--swatch-b:${p.light}"></span>
      <span data-i18n="palette_${p.id}">${t("palette_" + p.id)}</span>
    </button>`,
  ).join("");
  root.querySelectorAll(".palette-btn").forEach((btn) => {
    btn.onclick = () => {
      state.palette = btn.dataset.palette;
      applyTheme();
    };
  });
}

const map = new maplibregl.Map({
  container: "map",
  style: blankStyle,
  center: [37.6, 55.75],
  zoom: 3,
  attributionControl: true,
  pitchWithRotate: true,
});
map.addControl(new maplibregl.NavigationControl({ visualizePitch: true }), "top-right");
const geolocate = new maplibregl.GeolocateControl({
  positionOptions: {
    enableHighAccuracy: true,
    maximumAge: 30000,
    timeout: 15000,
  },
  trackUserLocation: false,
  showAccuracyCircle: true,
  showUserLocation: true,
  fitBoundsOptions: {
    maxZoom: 17,
  },
});
map.addControl(geolocate, "top-right");
map.addControl(new maplibregl.FullscreenControl(), "top-right");

const $ = (id) => document.getElementById(id);
const drawer = $("drawer");
const scrim = $("scrim");
renderPalettes();
applyTheme();

geolocate.on("geolocate", () => {
  dismissLocateToast();
  $("empty").hidden = true;
});
geolocate.on("error", (err) => {
  dismissLocateToast();
  const msg = err?.message || t("geo_locate_error");
  toast(msg);
});
document.querySelector(".maplibregl-ctrl-geolocate")?.addEventListener("click", () => {
  dismissLocateToast();
  locateToastEl = toast(t("geo_locating"), "info", { sticky: true });
});

function applyI18n() {
  document.documentElement.lang = state.lang;
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    el.textContent = t(el.dataset.i18n);
  });
  document.querySelectorAll("[data-i18n-placeholder]").forEach((el) => {
    el.placeholder = t(el.dataset.i18nPlaceholder);
  });
  document.querySelectorAll("[data-i18n-title]").forEach((el) => {
    const title = t(el.dataset.i18nTitle);
    el.title = title;
    el.setAttribute("aria-label", title);
  });
}

function openDrawer() {
  drawer.classList.add("open");
  scrim.hidden = false;
}
function closeDrawer() {
  drawer.classList.remove("open");
  scrim.hidden = true;
}

$("menu-toggle").onclick = () => (drawer.classList.contains("open") ? closeDrawer() : openDrawer());
scrim.onclick = closeDrawer;

$("tool-theme").onclick = () => {
  state.theme = state.theme === "dark" ? "light" : "dark";
  applyTheme({ reloadMap: true });
};

$("mode-dark").onclick = () => {
  state.theme = "dark";
  applyTheme({ reloadMap: true });
};
$("mode-light").onclick = () => {
  state.theme = "light";
  applyTheme({ reloadMap: true });
};

function setTab(name) {
  document.querySelectorAll(".tab").forEach((b) => b.classList.toggle("active", b.dataset.tab === name));
  document.querySelectorAll(".panel").forEach((p) => p.classList.toggle("active", p.id === `panel-${name}`));
}

document.querySelectorAll(".tab").forEach((btn) => {
  btn.onclick = () => setTab(btn.dataset.tab);
});

async function api(path, opts = {}) {
  const { silent = false, ...fetchOpts } = opts;
  let res;
  try {
    res = await fetch(path, {
      headers: { "Content-Type": "application/json" },
      ...fetchOpts,
    });
  } catch (err) {
    const msg = err?.message || String(err);
    if (!silent) toast(msg);
    throw err;
  }
  if (res.status === 204) return null;
  if (!res.ok) {
    const text = await res.text();
    let msg = text || `${res.status}`;
    try {
      const j = JSON.parse(text);
      if (j?.error) msg = j.error;
    } catch {
      /* plain text */
    }
    if (!silent) toast(msg);
    throw new Error(msg);
  }
  const ct = res.headers.get("content-type") || "";
  return ct.includes("json") ? res.json() : res.text();
}

let locateToastEl = null;

function dismissLocateToast() {
  if (!locateToastEl) return;
  locateToastEl.classList.remove("show");
  const el = locateToastEl;
  locateToastEl = null;
  setTimeout(() => el.remove(), 240);
}

function toast(message, kind = "error", { sticky = false, ttl = 4800 } = {}) {
  const root = $("toasts");
  const el = document.createElement("div");
  el.className = `toast toast-${kind}`;
  el.textContent = message;
  el.onclick = () => {
    if (el === locateToastEl) locateToastEl = null;
    el.classList.remove("show");
    setTimeout(() => el.remove(), 240);
  };
  root.appendChild(el);
  requestAnimationFrame(() => el.classList.add("show"));
  if (!sticky) {
    setTimeout(() => {
      if (!el.isConnected) return;
      el.classList.remove("show");
      setTimeout(() => el.remove(), 240);
    }, ttl);
  }
  return el;
}

function fmtSize(n) {
  if (!n) return "";
  const units = ["B", "KB", "MB", "GB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
}

function styleFromTileJSON(tj) {
  const format = String(tj.format || "").toLowerCase();
  const vector = format === "pbf" || format === "mvt" || Array.isArray(tj.vector_layers);
  const bg = mapBg();
  if (!vector) {
    return {
      version: 8,
      sources: {
        map: {
          type: "raster",
          tiles: tj.tiles,
          tileSize: tj.tilesize || 256,
          minzoom: tj.minzoom,
          maxzoom: tj.maxzoom,
        },
      },
      layers: [
        { id: "bg", type: "background", paint: { "background-color": bg } },
        { id: "raster", type: "raster", source: "map" },
      ],
    };
  }
  if (isProtomaps(tj)) {
    const flavor = state.theme === "light" ? "light" : "dark";
    return {
      version: 8,
      glyphs: `${PROTOMAPS_ASSETS}/fonts/{fontstack}/{range}.pbf`,
      sprite: `${PROTOMAPS_ASSETS}/sprites/v4/${flavor}`,
      sources: {
        map: {
          type: "vector",
          tiles: tj.tiles,
          minzoom: tj.minzoom,
          maxzoom: tj.maxzoom,
          attribution:
            tj.attribution ||
            '<a href="https://protomaps.com">Protomaps</a> © <a href="https://openstreetmap.org">OpenStreetMap</a>',
        },
      },
      layers: protomapsLayers("map", namedFlavor(flavor), { lang: state.lang || "en" }),
    };
  }
  const vls = tj.vector_layers?.length ? tj.vector_layers : [{ id: "layer" }];
  const styleLayers = [{ id: "bg", type: "background", paint: { "background-color": bg } }];
  vls.forEach((vl, i) => {
    const hue = (i * 47) % 360;
    const color = `hsl(${hue} 42% 64%)`;
    styleLayers.push({
      id: `${vl.id}-fill`,
      type: "fill",
      source: "map",
      "source-layer": vl.id,
      filter: ["==", "$type", "Polygon"],
      paint: { "fill-color": color, "fill-opacity": 0.28, "fill-outline-color": color },
    });
    styleLayers.push({
      id: `${vl.id}-line`,
      type: "line",
      source: "map",
      "source-layer": vl.id,
      filter: ["==", "$type", "LineString"],
      paint: { "line-color": color, "line-width": 1.15 },
    });
    styleLayers.push({
      id: `${vl.id}-pt`,
      type: "circle",
      source: "map",
      "source-layer": vl.id,
      filter: ["==", "$type", "Point"],
      paint: { "circle-color": color, "circle-radius": 3 },
    });
  });
  return {
    version: 8,
    sources: {
      map: {
        type: "vector",
        tiles: tj.tiles,
        minzoom: tj.minzoom,
        maxzoom: tj.maxzoom,
      },
    },
    layers: styleLayers,
  };
}

async function showLocal(item) {
  const tj = await api(item.tilejson || `/services/${item.id}`);
  state.active = { type: "local", id: item.id, kind: item.kind || "mbtiles" };
  state.activeTileJSON = tj;
  $("empty").hidden = true;
  $("current-name").textContent = item.name;
  $("current-meta").textContent = [item.kind, item.format, item.minzoom != null ? `${t("zoom")} ${item.minzoom}–${item.maxzoom}` : "", fmtSize(item.size)]
    .filter(Boolean)
    .join(" · ");
  map.once("style.load", () => {
    fitTileJSON(tj);
    writeViewHash();
  });
  map.setStyle(styleFromTileJSON(tj));
  renderMaps();
}

function fitTileJSON(tj) {
  if (state.hashCamera) {
    const cam = state.hashCamera;
    state.hashCamera = null;
    const view = { center: [cam.lon, cam.lat], zoom: cam.z };
    map.jumpTo(view);
    requestAnimationFrame(() => map.jumpTo(view));
    return;
  }
  if (Array.isArray(tj.bounds) && tj.bounds.length === 4) {
    map.fitBounds(
      [
        [tj.bounds[0], tj.bounds[1]],
        [tj.bounds[2], tj.bounds[3]],
      ],
      { padding: 48, duration: 700 },
    );
    return;
  }
  if (Array.isArray(tj.center) && tj.center.length >= 2) {
    map.easeTo({ center: [tj.center[0], tj.center[1]], zoom: tj.center[2] ?? 6, duration: 700 });
  }
}

function writeViewHash() {
  if (!state.active || state.active.type !== "local") return;
  const c = map.getCenter();
  const z = map.getZoom();
  const params = new URLSearchParams({
    map: state.active.id,
    kind: state.active.kind || "mbtiles",
    lat: c.lat.toFixed(5),
    lon: c.lng.toFixed(5),
    z: z.toFixed(2),
  });
  history.replaceState(null, "", `#${params.toString()}`);
}

function parseViewHash() {
  const raw = location.hash.replace(/^#/, "");
  if (!raw) return null;
  const p = new URLSearchParams(raw);
  const id = p.get("map");
  if (!id) return null;
  return {
    id,
    kind: p.get("kind") || "mbtiles",
    lat: Number(p.get("lat")),
    lon: Number(p.get("lon")),
    z: Number(p.get("z")),
  };
}

function renderMaps() {
  const q = $("search").value.trim().toLowerCase();
  const root = $("map-list");
  const items = state.maps.filter((m) => {
    const blob = `${m.name} ${m.id} ${m.format} ${m.description || ""} ${m.error || ""}`.toLowerCase();
    return !q || blob.includes(q);
  });
  if (!items.length) {
    root.innerHTML = `<p class="hint">${t("no_maps")}</p>`;
    renderMapsBreakdown();
    return;
  }
  root.innerHTML = items
    .map((m) => {
      const broken = Boolean(m.error);
      const hashBad = m.hash_status === "mismatch" || m.hash_status === "error";
      const verifying = m.hash_status === "running";
      const active =
        !broken && state.active?.type === "local" && state.active.id === m.id && state.active.kind === (m.kind || "mbtiles")
          ? " active"
          : "";
      const kindPill = broken
        ? `<span class="pill pill-danger">${t("map_broken")}</span>`
        : `<span class="pill">${escapeHtml(m.kind || "mbtiles")} · ${escapeHtml(m.format || "—")}</span>`;
      let hashPill = "";
      if (m.hash_status === "ok") {
        hashPill = `<span class="pill pill-ok">${t("hash_ok")}</span>`;
      } else if (m.hash_status === "mismatch") {
        hashPill = `<span class="pill pill-danger">${t("hash_bad")}</span>`;
      } else if (m.hash_status === "error") {
        hashPill = `<span class="pill pill-danger">${t("hash_error")}</span>`;
      }
      let hashMeta = "";
      if (m.hash_status === "mismatch") {
        hashMeta = `<p class="meta map-error">${escapeHtml(fmtTpl("hash_mismatch_hint", { expected: m.checksum || "—", actual: m.hash_actual || "—" }))}</p>`;
      } else if (m.hash_error && m.hash_status === "error") {
        hashMeta = `<p class="meta map-error">${escapeHtml(m.hash_error)}</p>`;
      }
      const body = broken
        ? `<p class="meta map-error">${escapeHtml(m.error)}</p>
           <p class="meta">${fmtSize(m.size)}</p>`
        : `<p>${escapeHtml(m.description || m.id)}</p>
           <p class="meta">${m.minzoom}–${m.maxzoom} · ${fmtSize(m.size)}</p>
           ${hashMeta}`;
      const hashPct =
        m.hash_total > 0 ? Math.min(100, Math.round((m.hash_written / m.hash_total) * 100)) : 0;
      const progress = verifying
        ? `<div class="progress"><i style="width:${hashPct}%"></i></div>
           <p class="meta">${hashPct}%</p>`
        : "";
      const verified = m.hash_status === "ok" || m.hash_status === "mismatch" || m.hash_status === "error";
      const verifyBtn =
        m.checksum && !broken && !verified && !verifying
          ? `<button class="btn secondary" data-verify="${escapeHtml(m.id)}" data-kind="${escapeHtml(m.kind || "mbtiles")}" type="button">${t("verify_btn")}</button>`
          : "";
      return `<article class="card${active}${broken ? " card-broken" : ""}${hashBad && !broken ? " card-hash-bad" : ""}" data-id="${escapeHtml(m.id)}" data-kind="${escapeHtml(m.kind || "mbtiles")}" data-broken="${broken ? "1" : "0"}">
        <div class="card-row">
          <h3><span>${escapeHtml(m.name)}</span></h3>
          <span class="pills">${kindPill}${hashPill}</span>
        </div>
        ${body}
        ${progress}
        <div class="card-row">
          <span class="meta">${escapeHtml(m.id)}</span>
          ${verifyBtn}
          <button class="btn danger" data-del="${escapeHtml(m.id)}" data-kind="${escapeHtml(m.kind || "mbtiles")}" type="button">${t("delete")}</button>
        </div>
      </article>`;
    })
    .join("");
  root.querySelectorAll(".card").forEach((el) => {
    el.onclick = (e) => {
      if (e.target.dataset.del || e.target.dataset.verify) return;
      const item = state.maps.find((m) => m.id === el.dataset.id && (m.kind || "mbtiles") === el.dataset.kind);
      if (!item) return;
      if (item.error) {
        toast(item.error || t("map_open_broken"));
        return;
      }
      showLocal(item);
    };
  });
  root.querySelectorAll("[data-verify]").forEach((btn) => {
    btn.onclick = async (e) => {
      e.stopPropagation();
      await api(`/api/maps/${encodeURIComponent(btn.dataset.verify)}/verify?kind=${encodeURIComponent(btn.dataset.kind)}`, {
        method: "POST",
        body: "{}",
      });
      await loadMaps();
      pollJobs();
    };
  });
  root.querySelectorAll("[data-del]").forEach((btn) => {
    btn.onclick = async (e) => {
      e.stopPropagation();
      await api(`/api/maps/${btn.dataset.del}?kind=${encodeURIComponent(btn.dataset.kind)}`, { method: "DELETE" });
      if (state.active?.id === btn.dataset.del && state.active?.kind === btn.dataset.kind) {
        map.setStyle(blankStyle);
        $("empty").hidden = false;
        $("current-name").textContent = "Maps";
        $("current-meta").textContent = "";
        state.active = null;
        state.activeTileJSON = null;
      }
      await loadMaps();
    };
  });
  renderMapsBreakdown();
}


function fmtTpl(key, vars) {
  return t(key).replace(/\{(\w+)\}/g, (_, k) => vars[k] ?? "");
}

const MAP_SEGMENT_COLORS = [
  "#3dd68c",
  "#e8b848",
  "#f0963c",
  "#5ab4f0",
  "#c084fc",
  "#46c8be",
  "#f07272",
  "#94a3b8",
];

function renderMapsBreakdown() {
  const root = $("maps-usage");
  const bar = $("maps-usage-bar");
  const legend = $("maps-usage-legend");
  const title = $("maps-usage-title");
  if (!root || !bar || !legend || !title) return;

  const maps = [...state.maps]
    .filter((m) => Number(m.size) > 0)
    .sort((a, b) => Number(b.size) - Number(a.size));
  const total = maps.reduce((s, m) => s + Number(m.size), 0);
  if (!maps.length || !total) {
    root.hidden = true;
    return;
  }
  root.hidden = false;
  title.textContent = fmtTpl("maps_usage", { used: formatBytes(total) });

  const topN = 5;
  const top = maps.slice(0, topN);
  const rest = maps.slice(topN);
  const restSize = rest.reduce((s, m) => s + Number(m.size), 0);
  const segments = top.map((m, i) => ({
    name: m.name,
    size: Number(m.size),
    color: MAP_SEGMENT_COLORS[i % MAP_SEGMENT_COLORS.length],
    tip: `${m.name} · ${formatBytes(m.size)}`,
  }));
  if (restSize > 0) {
    const tip = `${t("maps_other")} · ${formatBytes(restSize)}\n${rest
      .map((m) => `${m.name} (${formatBytes(m.size)})`)
      .join(", ")}`;
    segments.push({
      name: t("maps_other"),
      size: restSize,
      color: MAP_SEGMENT_COLORS[topN % MAP_SEGMENT_COLORS.length],
      tip,
    });
  }

  bar.innerHTML = segments
    .map((seg) => {
      const pct = (seg.size / total) * 100;
      return `<i style="width:${pct}%;background:${seg.color}" title="${escapeHtml(seg.tip)}"></i>`;
    })
    .join("");

  legend.innerHTML = segments
    .map(
      (seg) => `<span class="storage-legend-item" title="${escapeHtml(seg.tip)}">
        <span class="storage-legend-swatch" style="background:${seg.color}"></span>
        <span>${escapeHtml(seg.name)} <strong>${formatBytes(seg.size)}</strong></span>
      </span>`,
    )
    .join("");
}

function renderStorage() {
  const st = state.storage;
  const disk = $("disk-usage");
  if (!st) {
    if (disk) disk.hidden = true;
    return;
  }
  const total = Number(st.total) || 0;
  const free = Number(st.free) || 0;
  const usedPct = total > 0 ? Math.min(100, ((total - free) / total) * 100) : 0;
  if (disk) {
    disk.hidden = false;
    $("disk-usage-fill").style.width = `${usedPct}%`;
    $("disk-usage-text").textContent = fmtTpl("disk_usage", {
      free: formatBytes(free),
      total: formatBytes(total),
    });
  }
}

async function loadStorage() {
  state.storage = await api("/api/storage");
  renderStorage();
}

function formatBytes(n) {
  n = Math.max(0, Number(n) || 0);
  if (n < 1024) return `${Math.round(n)} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = n;
  let i = -1;
  do {
    v /= 1024;
    i++;
  } while (v >= 1024 && i < units.length - 1);
  const digits = v < 10 ? 1 : 0;
  return `${v.toFixed(digits)} ${units[i]}`;
}

function jobProgressLine(j) {
  const parts = [];
  if (j.total > 0) {
    const pct = Math.min(100, Math.round((j.written / j.total) * 100));
    parts.push(`${formatBytes(j.written)} / ${formatBytes(j.total)}`);
    parts.push(`${pct}%`);
  } else {
    parts.push(formatBytes(j.written || 0));
  }
  if (j.status === "running" && j.speed > 0) {
    parts.push(`${formatBytes(j.speed)}/s`);
  }
  return parts.join(" · ");
}

function renderJobs() {
  const root = $("job-list");
  if (!state.jobs.length) {
    root.innerHTML = `<p class="hint">${t("no_jobs")}</p>`;
    return;
  }
  root.innerHTML = state.jobs
    .map((j) => {
      const pct =
        j.total > 0
          ? Math.min(100, Math.round((j.written / j.total) * 100))
          : j.status === "done"
            ? 100
            : 0;
      let actions = `<button class="btn danger" data-job-del="${escapeHtml(j.id)}" type="button">${t("delete")}</button>`;
      if (j.status === "running") {
        actions = `<button class="btn secondary" data-job-cancel="${escapeHtml(j.id)}" type="button">${t("cancel")}</button>
             ${actions}`;
      } else if (j.status === "error" || j.status === "paused") {
        actions = `<button class="btn" data-job-resume="${escapeHtml(j.id)}" type="button">${t("resume_btn")}</button>
             ${actions}`;
      }
      return `<article class="card">
        <div class="card-row"><h3>${escapeHtml(j.name)}</h3><span class="pill">${escapeHtml(j.status)}</span></div>
        <p class="meta">${escapeHtml(j.error || j.url)}</p>
        <p class="meta">${escapeHtml(jobProgressLine(j))}</p>
        <div class="progress"><i style="width:${pct}%"></i></div>
        <div class="card-row">${actions}</div>
      </article>`;
    })
    .join("");
  root.querySelectorAll("[data-job-cancel]").forEach((btn) => {
    btn.onclick = async () => {
      await api(`/api/downloads/${btn.dataset.jobCancel}/cancel`, { method: "POST", body: "{}" });
      pollJobs();
    };
  });
  root.querySelectorAll("[data-job-resume]").forEach((btn) => {
    btn.onclick = async () => {
      await api(`/api/downloads/${btn.dataset.jobResume}/resume`, { method: "POST", body: "{}" });
      pollJobs();
    };
  });
  root.querySelectorAll("[data-job-del]").forEach((btn) => {
    btn.onclick = async () => {
      await api(`/api/downloads/${btn.dataset.jobDel}`, { method: "DELETE" });
      state.jobs = state.jobs.filter((j) => j.id !== btn.dataset.jobDel);
      renderJobs();
    };
  });
}

function catalogFilename(item) {
  return item.filename || "";
}

function mapHasFilename(filename) {
  if (!filename) return false;
  const stem = filename.replace(/\.(pm|mb)tiles$/i, "");
  return state.maps.some((m) => {
    if (m.id === stem || m.id === filename) return true;
    const path = m.path || "";
    return path === filename || path.endsWith("/" + filename);
  });
}

function activeJobForFilename(filename) {
  if (!filename) return null;
  return state.jobs.find((j) => j.name === filename && (j.status === "running" || j.status === "paused"));
}

function catalogAction(item) {
  const filename = catalogFilename(item);
  if (activeJobForFilename(filename)) {
    return { label: t("downloading_btn"), replace: false, disabled: true };
  }
  if (mapHasFilename(filename)) {
    return { label: t("redownload_btn"), replace: true, disabled: false };
  }
  return { label: t("download_btn"), replace: false, disabled: false };
}

function renderCatalog() {
  const q = $("catalog-search").value.trim().toLowerCase();
  const root = $("catalog-list");
  const sections = (state.catalog || [])
    .map((sec) => {
      const items = (sec.items || []).filter((c) => {
        if (!q) return true;
        return `${c.name} ${c.region || ""} ${sec.category || ""} ${c.id || ""}`.toLowerCase().includes(q);
      });
      return { category: sec.category, items };
    })
    .filter((sec) => !q || sec.items.length);

  if (!sections.length) {
    root.innerHTML = `<p class="hint">${t("no_catalog")}</p>`;
    return;
  }

  root.innerHTML = sections
    .map((sec) => {
      const cards = sec.items.length
        ? sec.items
            .map((c) => {
              const action = catalogAction(c);
              const flag = c.flag
                ? `<img class="catalog-flag" src="https://flagcdn.com/${escapeHtml(c.flag)}.svg" alt="" loading="lazy" />`
                : "";
              return `<article class="card">
        <div class="card-row"><h3>${flag}<span>${escapeHtml(c.name)}</span></h3><span class="pill">${escapeHtml(c.format)}</span></div>
        <p>${escapeHtml(c.description)}</p>
        <p class="meta">${escapeHtml(c.size_hint || "")}</p>
        <button class="btn secondary" data-cat="${c.id}" data-replace="${action.replace ? "1" : "0"}" type="button"${action.disabled ? " disabled" : ""}>${escapeHtml(action.label)}</button>
      </article>`;
            })
            .join("")
        : `<p class="hint">${t("no_catalog")}</p>`;
      return `<section class="catalog-section">
        <h2>${escapeHtml(sec.category)}</h2>
        <div class="list catalog-section-list">${cards}</div>
      </section>`;
    })
    .join("");

  root.querySelectorAll("[data-cat]").forEach((btn) => {
    btn.onclick = async () => {
      await api(`/api/catalog/${btn.dataset.cat}/download`, {
        method: "POST",
        body: JSON.stringify({ replace: btn.dataset.replace === "1" }),
      });
      setTab("download");
      pollJobs();
    };
  });
}


function escapeHtml(s) {
  return String(s)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

async function loadMaps() {
  state.maps = await api("/api/maps");
  renderMaps();
  renderCatalog();
  await loadStorage();
}
async function loadCatalog() {
  state.catalog = await api("/api/catalog");
  renderCatalog();
}
async function pollJobs() {
  state.jobs = await api("/api/downloads");
  renderJobs();
  renderCatalog();
  const downloading = state.jobs.some((j) => j.status === "running");
  const verifying = state.maps.some((m) => m.hash_status === "running");
  if (downloading || verifying) {
    if (verifying) await loadMaps();
    setTimeout(pollJobs, 700);
  } else {
    loadMaps();
  }
}

$("search").oninput = renderMaps;
$("catalog-search").oninput = renderCatalog;
$("clear-completed").onclick = async () => {
  await api("/api/downloads/clear-completed", { method: "POST", body: "{}" });
  pollJobs();
};


$("url-form").onsubmit = async (e) => {
  e.preventDefault();
  await api("/api/downloads", {
    method: "POST",
    body: JSON.stringify({
      url: $("url-input").value,
      name: $("name-input").value,
      checksum: $("checksum-input").value,
    }),
  });
  $("url-input").value = "";
  $("name-input").value = "";
  $("checksum-input").value = "";
  pollJobs();
};

$("download-settings").onsubmit = async (e) => {
  e.preventDefault();
  const settings = await api("/api/settings", {
    method: "PUT",
    body: JSON.stringify({
      rate_limit_bps: Number($("rate-limit").value) || 0,
    }),
  });
  $("rate-limit").value = settings.rate_limit_bps || 0;
};

$("lang-select").onchange = async () => {
  state.lang = $("lang-select").value;
  applyI18n();
  renderPalettes();
  await api("/api/settings", { method: "PUT", body: JSON.stringify({ language: state.lang }) });
  renderMaps();
  renderJobs();
  renderCatalog();
  renderGeocoders();
  renderMapsBreakdown();
  renderStorage();
  refreshActiveStyle();
};

function viewboxParam() {
  const b = map.getBounds();
  return [b.getWest(), b.getSouth(), b.getEast(), b.getNorth()].join(",");
}

function focusParams() {
  const c = map.getCenter();
  return `lat=${c.lat}&lon=${c.lng}&viewbox=${encodeURIComponent(viewboxParam())}`;
}

function flyToResult(r) {
  if (state.geoMarker) state.geoMarker.remove();
  state.geoMarker = new maplibregl.Marker({ color: getComputedStyle(document.documentElement).getPropertyValue("--gold").trim() || "#e8b848" })
    .setLngLat([r.lon, r.lat])
    .setPopup(new maplibregl.Popup().setText(r.label))
    .addTo(map);
  if (Array.isArray(r.bbox) && r.bbox.length === 4) {
    map.fitBounds(
      [
        [r.bbox[0], r.bbox[1]],
        [r.bbox[2], r.bbox[3]],
      ],
      { padding: 64, duration: 900, maxZoom: 16 },
    );
  } else {
    map.easeTo({ center: [r.lon, r.lat], zoom: Math.max(map.getZoom(), 14), duration: 900 });
  }
  $("empty").hidden = true;
}

function clearGeoMarker() {
  if (!state.geoMarker) return;
  state.geoMarker.remove();
  state.geoMarker = null;
}

function hideGeoResults() {
  const box = $("geo-results");
  box.hidden = true;
  box.innerHTML = "";
}

function showGeoResults(items, err) {
  const box = $("geo-results");
  if (err) {
    box.innerHTML = `<div class="geo-empty">${escapeHtml(t("geo_error"))}: ${escapeHtml(err)}</div>`;
    box.hidden = false;
    return;
  }
  if (!items.length) {
    box.innerHTML = `<div class="geo-empty">${escapeHtml(t("geo_empty"))}</div>`;
    box.hidden = false;
    return;
  }
  box.innerHTML = items
    .map((r, i) => `<button type="button" data-i="${i}">${escapeHtml(r.label)}</button>`)
    .join("");
  box.hidden = false;
  box.querySelectorAll("button").forEach((btn) => {
    btn.onclick = () => {
      flyToResult(items[Number(btn.dataset.i)]);
      hideGeoResults();
      $("geo-q").value = items[Number(btn.dataset.i)].label;
    };
  });
}

let geoTimer = null;
$("geo-q").oninput = () => {
  clearTimeout(geoTimer);
  const q = $("geo-q").value.trim();
  if (!q) {
    hideGeoResults();
    clearGeoMarker();
    return;
  }
  if (q.length < 2) {
    hideGeoResults();
    return;
  }
  geoTimer = setTimeout(async () => {
    try {
      const data = await api(`/api/geocode/autocomplete?q=${encodeURIComponent(q)}&${focusParams()}`, { silent: true });
      showGeoResults(data.results || [], data.error);
    } catch (e) {
      showGeoResults([], String(e.message || e));
    }
  }, 280);
};

$("geo-q").onkeydown = async (e) => {
  if (e.key !== "Enter") return;
  e.preventDefault();
  clearTimeout(geoTimer);
  const q = $("geo-q").value.trim();
  if (!q) return;
  try {
    const data = await api(`/api/geocode?q=${encodeURIComponent(q)}&${focusParams()}`);
    const items = data.results || [];
    if (items[0]) {
      flyToResult(items[0]);
      hideGeoResults();
    } else {
      showGeoResults(items, data.error);
    }
  } catch (err) {
    showGeoResults([], String(err.message || err));
  }
};

document.addEventListener("click", (e) => {
  if (!e.target.closest(".geo-search")) hideGeoResults();
});

map.on("click", async (e) => {
  if (e.originalEvent?.target?.closest?.(".maplibregl-marker, .maplibregl-popup, .hud, .drawer, .scrim")) return;
  try {
    const data = await api(`/api/geocode/reverse?lat=${e.lngLat.lat}&lon=${e.lngLat.lng}&limit=1`, { silent: true });
    const r = (data.results || [])[0];
    if (!r) return;
    flyToResult(r);
    $("geo-q").value = r.label;
  } catch {
    /* ignore */
  }
});

function renderGeocoders() {
  const root = $("geocoder-list");
  if (!root) return;
  root.innerHTML = state.geocoders
    .map((g, idx) => {
      const st = g.status === "fail" ? "fail" : g.status === "ok" ? "ok" : "idle";
      const stText =
        st === "fail" ? `${t("geocoder_fail")}: ${g.error || ""}` : st === "ok" ? t("geocoder_ok") : t("geocoder_idle");
      return `<article class="geocoder-card" draggable="true" data-id="${escapeHtml(g.id)}" data-idx="${idx}">
        <div class="card-row">
          <span class="drag-handle" aria-hidden="true">⠿</span>
          <h3>${escapeHtml(g.id)}</h3>
          <span class="pill">${g.has_key ? t("geocoder_has_key") : t("geocoder_no_key")}</span>
        </div>
        <label class="inline"><input type="checkbox" data-en="${escapeHtml(g.id)}" ${g.enabled ? "checked" : ""} /> ${t("geocoder_enabled")}</label>
        <label>
          <span>${t("geocoder_url")}</span>
          <input type="url" data-url="${escapeHtml(g.id)}" value="${escapeHtml(g.base_url)}" />
        </label>
        <div class="geocoder-foot">
          <span class="geocoder-status ${st}" title="${escapeHtml(stText)}">${escapeHtml(stText)}</span>
          <button class="btn secondary" type="button" data-test="${escapeHtml(g.id)}">${t("geocoder_test")}</button>
        </div>
      </article>`;
    })
    .join("");

  let dragId = null;
  root.querySelectorAll(".geocoder-card").forEach((el) => {
    el.ondragstart = () => {
      dragId = el.dataset.id;
      el.classList.add("dragging");
    };
    el.ondragend = () => el.classList.remove("dragging");
    el.ondragover = (e) => e.preventDefault();
    el.ondrop = async (e) => {
      e.preventDefault();
      const toId = el.dataset.id;
      if (!dragId || dragId === toId) return;
      const from = state.geocoders.findIndex((g) => g.id === dragId);
      const to = state.geocoders.findIndex((g) => g.id === toId);
      if (from < 0 || to < 0) return;
      const [item] = state.geocoders.splice(from, 1);
      state.geocoders.splice(to, 0, item);
      await saveGeocoders();
    };
  });
  root.querySelectorAll("[data-en]").forEach((el) => {
    el.onchange = async () => {
      const g = state.geocoders.find((x) => x.id === el.dataset.en);
      g.enabled = el.checked;
      await saveGeocoders();
    };
  });
  root.querySelectorAll("[data-url]").forEach((el) => {
    el.onchange = async () => {
      const g = state.geocoders.find((x) => x.id === el.dataset.url);
      g.base_url = el.value.trim();
      await saveGeocoders();
    };
  });
  root.querySelectorAll("[data-test]").forEach((btn) => {
    btn.onclick = async () => {
      try {
        await api(`/api/geocoders/${btn.dataset.test}/test`, { method: "POST", body: "{}" });
      } catch {
        /* status comes from list */
      }
      state.geocoders = await api("/api/geocoders");
      renderGeocoders();
    };
  });
}

async function saveGeocoders() {
  state.geocoders = await api("/api/geocoders", {
    method: "PUT",
    body: JSON.stringify(
      state.geocoders.map((g) => ({
        id: g.id,
        enabled: g.enabled,
        base_url: g.base_url,
      })),
    ),
  });
  renderGeocoders();
}

const settings = await api("/api/settings");
state.lang = settings.language || "ru";
state.geocoders = settings.geocoders || [];
$("lang-select").value = state.lang;
$("rate-limit").value = settings.rate_limit_bps || 0;
{
  const el = $("build-info");
  const ver = settings.version || "dev";
  const rev = settings.commit || "unknown";
  el.textContent = `mantica ${ver} · ${rev}`;
  el.hidden = false;
}
applyTheme();
applyI18n();
renderGeocoders();
await Promise.all([loadMaps(), loadCatalog(), pollJobs(), loadStorage()]);

map.on("moveend", () => writeViewHash());

const bootHash = parseViewHash();
if (bootHash) {
  const item = state.maps.find((m) => m.id === bootHash.id && (m.kind || "mbtiles") === bootHash.kind);
  if (item && !item.error && Number.isFinite(bootHash.lat) && Number.isFinite(bootHash.lon)) {
    state.hashCamera = { lat: bootHash.lat, lon: bootHash.lon, z: Number.isFinite(bootHash.z) ? bootHash.z : 3 };
    await showLocal(item);
  }
}

openDrawer();
