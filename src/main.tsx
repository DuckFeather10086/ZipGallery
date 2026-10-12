import React, {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { createRoot } from "react-dom/client";
import { useVirtualizer } from "@tanstack/react-virtual";
import { onFileDrop, runtime } from "mygo-runtime";
import {
  Desktop,
  type View,
  type Entry,
  type Recent,
  type RepairPreview,
} from "./mygo";
import "./style.css";
import { t, errorMessage, useLocale, setLocale } from "./i18n";
import { ArchiveTools, FileList } from "./archive-ui";
import { fullImage, keepFullImages, cachedFull } from "./full-cache";
import { pagingState, pagingPulse } from "./wheel";
const message = errorMessage;
function imageURL(e: Entry, thumb: boolean) {
  return `${runtime().platform === "win32" ? "http://gallery.localhost" : "gallery://localhost"}/${thumb ? "thumb" : "full"}/${e.id}`;
}
function Picture({
  entry,
  thumb = false,
  onSize,
}: {
  entry: Entry;
  thumb?: boolean;
  onSize?: (w: number, h: number) => void;
}) {
  const [url, setURL] = useState(thumb ? "" : cachedFull(entry.id)),
    [error, setError] = useState("");
  useEffect(() => {
    const c = new AbortController();
    let blobURL = "";
    setURL(thumb ? "" : cachedFull(entry.id));
    setError("");
    if (!thumb) {
      fullImage(entry.id, imageURL(entry, false))
        .then((u) => {
          if (!c.signal.aborted) setURL(u);
        })
        .catch((e) => {
          if (!c.signal.aborted) setError(String(e));
        });
      return () => c.abort();
    }
    fetch(imageURL(entry, thumb), { signal: c.signal })
      .then(async (r) => {
        if (!r.ok) throw Error(await r.text());
        return r.blob();
      })
      .then((b) => {
        if (c.signal.aborted) return;
        blobURL = URL.createObjectURL(b);
        setURL(blobURL);
      })
      .catch((e) => {
        if (!c.signal.aborted)
          setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      c.abort();
      if (blobURL) URL.revokeObjectURL(blobURL);
    };
  }, [entry.id, thumb]);
  return error ? (
    <span className="image-error" role="alert">
      ⚠<br />
      {message(error)}
    </span>
  ) : url ? (
    <img
      src={url}
      alt={entry.name}
      draggable={false}
      onLoad={(e) =>
        onSize?.(e.currentTarget.naturalWidth, e.currentTarget.naturalHeight)
      }
    />
  ) : (
    <span className="loading">{t("loading")}</span>
  );
}
const translateFolder = (count: number) => t("folderCount", { count });
const translateRoot = () => t("root");
type Tile = { folder: string; count: number } | { entry: Entry };
function Grid({
  tiles,
  selected,
  onFolder,
  onImage,
  folder,
  positions,
}: {
  tiles: Tile[];
  selected: string;
  onFolder: (s: string) => void;
  onImage: (e: Entry) => void;
  folder: string;
  positions: React.RefObject<Record<string, number>>;
}) {
  const scroll = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(900);
  const columns = Math.max(2, Math.floor(width / 210));
  useEffect(() => {
    const ro = new ResizeObserver((es) => setWidth(es[0].contentRect.width));
    if (scroll.current) ro.observe(scroll.current);
    return () => ro.disconnect();
  }, []);
  const virtual = useVirtualizer({
    count: Math.ceil(tiles.length / columns),
    getScrollElement: () => scroll.current,
    estimateSize: () => 222,
    overscan: 1,
  });
  useLayoutEffect(() => {
    if (scroll.current)
      scroll.current.scrollTop = positions.current[folder] || 0;
  }, [folder]);
  return (
    <div
      className="grid-scroll"
      tabIndex={0}
      aria-label={t("imageGrid")}
      ref={scroll}
      onScroll={(e) => {
        positions.current[folder] = e.currentTarget.scrollTop;
      }}
    >
      <div style={{ height: virtual.getTotalSize(), position: "relative" }}>
        {virtual.getVirtualItems().map((row) => (
          <div
            className="tile-row"
            key={row.index}
            style={{
              transform: `translateY(${row.start}px)`,
              gridTemplateColumns: `repeat(${columns},minmax(0,1fr))`,
            }}
          >
            {tiles
              .slice(row.index * columns, (row.index + 1) * columns)
              .map((t) =>
                "folder" in t ? (
                  <button
                    key={"folder:" + t.folder}
                    className="tile folder"
                    onClick={() => onFolder(t.folder)}
                  >
                    <span className="folder-icon">▱</span>
                    <strong>{t.folder.split("/").at(-1)}</strong>
                    <small>{translateFolder(t.count)}</small>
                  </button>
                ) : (
                  <button
                    key={t.entry.id}
                    title={t.entry.name}
                    className={
                      "tile " + (selected === t.entry.id ? "selected" : "")
                    }
                    onClick={() => onImage(t.entry)}
                  >
                    <div className="thumbnail">
                      <Picture entry={t.entry} thumb />
                    </div>
                    <strong>{t.entry.name.split("/").at(-1)}</strong>
                    <small>{t.entry.folder || translateRoot()}</small>
                  </button>
                ),
              )}
          </div>
        ))}
      </div>
      {tiles.length === 0 && <div className="empty">{t("emptyFolder")}</div>}
    </div>
  );
}
function Viewer({
  entries,
  index,
  onIndex,
  onClose,
}: {
  entries: Entry[];
  index: number;
  onIndex: (i: number) => void;
  onClose: () => void;
}) {
  const [zoom, setZoom] = useState<number | null>(null),
    [size, setSize] = useState({ w: 1000, h: 700 });
  const entry = entries[index];
  const stage = useRef<HTMLDivElement>(null);
  const paging = useRef(pagingState());
  // Advance the live cursor synchronously so multiple inputs before React's
  // next render cannot all target the same stale index.
  const cursor = useRef(index);
  const latest = useRef({ entries, onIndex });
  useLayoutEffect(() => {
    cursor.current = index;
    latest.current = { entries, onIndex };
  }, [index, entries, onIndex]);
  const step = useCallback((d: number) => {
    const next = Math.max(
      0,
      Math.min(latest.current.entries.length - 1, cursor.current + d),
    );
    if (next === cursor.current) return;
    cursor.current = next;
    latest.current.onIndex(next);
  }, []);
  const inputStep = useCallback(
    (direction: number, repeat = true) => {
      const d = pagingPulse(
        paging.current,
        direction,
        performance.now(),
        repeat,
      );
      if (d) step(d);
    },
    [step],
  );
  useEffect(() => {
    setZoom(null);
    const nearby = entries.slice(Math.max(0, index - 1), index + 3);
    keepFullImages(nearby.map((e) => e.id));
    for (const next of nearby)
      if (next.id !== entry.id)
        void fullImage(next.id, imageURL(next, false) + "?prefetch=1").catch(
          () => {},
        );
  }, [entry.id]);
  useEffect(() => {
    const f = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      } else if (
        e.key === "ArrowRight" ||
        e.key === "ArrowDown" ||
        e.key === "PageDown"
      ) {
        e.preventDefault();
        inputStep(1, e.repeat);
      } else if (
        e.key === "ArrowLeft" ||
        e.key === "ArrowUp" ||
        e.key === "PageUp"
      ) {
        e.preventDefault();
        inputStep(-1, e.repeat);
      } else if (e.key === "+" || e.key === "=") {
        setZoom((z) => Math.min(4, (z ?? 1) + 0.25));
      } else if (e.key === "-") {
        setZoom((z) => Math.max(0.25, (z ?? 1) - 0.25));
      } else if (e.key === "0") {
        setZoom(null);
      } else if (e.key === "1") {
        setZoom(1);
      }
    };
    window.addEventListener("keydown", f);
    return () => window.removeEventListener("keydown", f);
  }, [inputStep, onClose]);
  useEffect(() => {
    const el = stage.current;
    if (!el) return;
    paging.current = pagingState();
    const reset = () => {
      paging.current = pagingState();
    };
    const onWheel = (e: WheelEvent) => {
      if (zoom !== null) return;
      e.preventDefault();
      if (e.ctrlKey || Math.abs(e.deltaX) > Math.abs(e.deltaY)) return;
      inputStep(e.deltaY);
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    window.addEventListener("blur", reset);
    return () => {
      el.removeEventListener("wheel", onWheel);
      window.removeEventListener("blur", reset);
      reset();
    };
  }, [inputStep, zoom]);
  return (
    <div className="viewer" role="dialog" aria-label={t("viewer")}>
      <div className="viewer-bar">
        <button onClick={onClose}>
          {t("backGrid")} <kbd>Esc</kbd>
        </button>
        <span className="viewer-title">{entry.name}</span>
        <span>
          {index + 1} / {entries.length}
        </span>
        <button
          aria-label={t("zoomOut")}
          onClick={() => setZoom((z) => Math.max(0.25, (z ?? 1) - 0.25))}
        >
          −
        </button>
        <button
          className={zoom === null ? "pressed" : ""}
          onClick={() => setZoom(null)}
        >
          {t("fit")}
        </button>
        <button
          className={zoom === 1 ? "pressed" : ""}
          onClick={() => setZoom(1)}
        >
          {t("actual")}
        </button>
        <button
          aria-label={t("zoomIn")}
          onClick={() => setZoom((z) => Math.min(4, (z ?? 1) + 0.25))}
        >
          ＋
        </button>
      </div>
      <div className="image-stage" ref={stage}>
        <div
          className={"full-image " + (zoom === null ? "fit" : "")}
          style={
            zoom === null
              ? {}
              : {
                  width: Math.round(size.w * zoom),
                  height: Math.round(size.h * zoom),
                }
          }
        >
          <Picture
            key={entry.id}
            entry={entry}
            onSize={(w, h) => setSize({ w, h })}
          />
        </div>
      </div>
      <div className="pager">
        <button disabled={index === 0} onClick={() => step(-1)}>
          {t("previous")}
        </button>
        <span>
          {zoom === null
            ? t("wheelHint")
            : t("panHint", { percent: Math.round(zoom * 100) })}
        </span>
        <button disabled={index === entries.length - 1} onClick={() => step(1)}>
          {t("next")}
        </button>
      </div>
    </div>
  );
}
function Repair({
  path,
  onClose,
  onSaved,
}: {
  path: string;
  onClose: () => void;
  onSaved: (p: string) => void;
}) {
  const [codec, setCodec] = useState("gb18030"),
    [preview, setPreview] = useState<RepairPreview | null>(null),
    [error, setError] = useState(""),
    [working, setWorking] = useState(false);
  const inspect = async () => {
    setWorking(true);
    setError("");
    setPreview(null);
    try {
      setPreview(await Desktop.previewRepair(path, codec));
    } catch (e) {
      setError(message(e));
    } finally {
      setWorking(false);
    }
  };
  const save = async () => {
    if (!preview) return;
    setWorking(true);
    setError("");
    try {
      const p = await Desktop.repair(
        path,
        codec,
        preview.version,
        t("saveCopy"),
      );
      if (p) onSaved(p);
    } catch (e) {
      setError(message(e));
    } finally {
      setWorking(false);
    }
  };
  return (
    <div className="modal-shade">
      <section
        className="repair-dialog"
        role="dialog"
        aria-label={t("repairTitle")}
      >
        <h2>{t("repairTitle")}</h2>
        <p>{t("repairHint")}</p>
        <div className="repair-controls">
          <label>
            {t("encoding")}{" "}
            <select
              disabled={working}
              value={codec}
              onChange={(e) => {
                setCodec(e.target.value);
                setPreview(null);
              }}
            >
              <option value="gb18030">GB18030 / GBK</option>
              <option value="shift-jis">Shift-JIS</option>
              <option value="cp437">CP437</option>
            </select>
          </label>
          <button disabled={working} onClick={inspect}>
            {t("preview")}
          </button>
        </div>
        {error && <div className="banner error">{error}</div>}
        {preview && (
          <>
            <p>
              {t("repairSummary", {
                count: preview.changes.length,
                total: preview.total,
              })}
            </p>
            <div className="repair-list">
              <div className="repair-row">
                <strong>{t("before")}</strong>
                <strong>{t("after")}</strong>
              </div>
              {preview.changes.slice(0, 100).map((r, i) => (
                <div className="repair-row" key={i}>
                  <code>{r.before}</code>
                  <span>{r.after}</span>
                </div>
              ))}
            </div>
            {preview.changes.length > 100 && <small>{t("repairMore")}</small>}
          </>
        )}
        {working && <p>{t("repairWorking")}</p>}
        <div className="repair-actions">
          <button disabled={working} onClick={onClose}>
            {t("close")}
          </button>
          <button
            disabled={working || !preview || preview.changes.length === 0}
            onClick={save}
          >
            {t("saveCopy")}
          </button>
        </div>
      </section>
    </div>
  );
}
type Snapshot = {
  path: string;
  folder: string;
  all: boolean;
  selected: string;
  positions: Record<string, number>;
};
function App() {
  const locale = useLocale();
  const [repair, setRepair] = useState(false);
  const [filesMode, setFilesMode] = useState(false),
    [locked, setLocked] = useState(""),
    [password, setPassword] = useState("");
  const [view, setView] = useState<View | null>(null),
    [folder, setFolder] = useState(""),
    [all, setAll] = useState(false),
    [selected, setSelected] = useState(""),
    [reader, setReader] = useState<string | null>(null),
    [recents, setRecents] = useState<Recent[]>([]),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [notice, setNotice] = useState("");
  const positions = useRef<Record<string, number>>({});
  const generation = useRef(0);
  const back = useRef<Snapshot[]>([]),
    forward = useRef<Snapshot[]>([]);
  const snapshot = useRef<Snapshot | null>(null);
  snapshot.current = view
    ? {
        path: view.location.path,
        folder,
        all,
        selected: view.entries.find((e) => e.id === selected)?.identity || "",
        positions: { ...positions.current },
      }
    : null;
  const adopt = useCallback((v: View, restore?: Snapshot) => {
    keepFullImages([]);
    if (!v.id) return;
    setView(v);
    setFilesMode(!v.entries.length);
    setFolder(restore?.folder ?? (v.folder === "*" ? "" : v.folder || ""));
    setAll(restore?.all ?? v.folder === "*");
    setReader(null);
    setSelected(
      v.entries.find((e) => e.identity === (restore?.selected ?? v.progress))
        ?.id || "",
    );
    positions.current = restore?.positions || {};
    Desktop.recents()
      .then(setRecents)
      .catch((e) => setError(message(e)));
  }, []);
  const open = useCallback(
    async (path?: string) => {
      const g = ++generation.current,
        previous = snapshot.current
          ? { ...snapshot.current, positions: { ...positions.current } }
          : null;
      setBusy(true);
      setError("");
      try {
        path = path || (await Desktop.selectArchive(t("openTitle")));
        if (!path) return;
        const v = await Desktop.openArchive(path, "");
        if (g === generation.current && v.id) {
          if (previous && previous.path !== v.location.path) {
            back.current.push(previous);
            forward.current = [];
          }
          adopt(v);
        }
      } catch (e) {
        if (g === generation.current) {
          if (String(e).includes("E_PASSWORD") && path) setLocked(path);
          else setError(message(e));
        }
      } finally {
        if (g === generation.current) setBusy(false);
      }
    },
    [adopt],
  );
  const travel = async (direction: "back" | "forward") => {
    const source = direction === "back" ? back : forward,
      target = direction === "back" ? forward : back;
    const restore = source.current.at(-1);
    if (!restore || busy) return;
    setBusy(true);
    setError("");
    const g = ++generation.current;
    try {
      const v = await Desktop.openLocation({
        kind: "local",
        path: restore.path,
      });
      if (g === generation.current) {
        if (snapshot.current)
          target.current.push({
            ...snapshot.current,
            positions: { ...positions.current },
          });
        source.current.pop();
        adopt(v, restore);
      }
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => {
    Desktop.current()
      .then((v) => adopt(v))
      .catch((e) => setError(message(e)));
    Desktop.recents()
      .then(setRecents)
      .catch((e) => setError(message(e)));
    const off = onFileDrop((e) => {
      if (e.paths[0]) void open(e.paths[0]);
    });
    let polling = false;
    const poll = setInterval(async () => {
      if (polling) return;
      polling = true;
      try {
        const p = await Desktop.pending();
        if (p) await open(p);
      } catch (e) {
        setError(message(e));
      } finally {
        polling = false;
      }
    }, 300);
    return () => {
      off();
      clearInterval(poll);
    };
  }, [adopt, open]);
  useEffect(() => {
    const f = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "o") {
        e.preventDefault();
        void open();
      } else if (e.altKey && e.key === "ArrowLeft" && !reader) {
        e.preventDefault();
        void travel("back");
      } else if (e.altKey && e.key === "ArrowRight" && !reader) {
        e.preventDefault();
        void travel("forward");
      }
    };
    window.addEventListener("keydown", f);
    return () => window.removeEventListener("keydown", f);
  }, [open, reader, busy]);
  const entries = useMemo(
    () => view?.entries.filter((e) => all || e.folder === folder) || [],
    [view, folder, all],
  );
  const tiles = useMemo(() => {
    const folders = new Map<string, number>();
    if (!all)
      for (const e of view?.entries || []) {
        const prefix = folder ? folder + "/" : "";
        if (e.folder !== folder && e.name.startsWith(prefix)) {
          const rest = e.name.slice(prefix.length);
          if (rest.includes("/")) {
            const name = prefix + rest.split("/")[0];
            folders.set(name, (folders.get(name) || 0) + 1);
          }
        }
      }
    return [
      ...[...folders].map(([folder, count]) => ({ folder, count })),
      ...entries.map((entry) => ({ entry })),
    ];
  }, [view, folder, all, entries]);
  const save = useCallback(
    (id: string, f: string) => {
      if (view)
        Desktop.progress(view.id, id, f).catch((e) =>
          setError(t("progressError") + message(e)),
        );
    },
    [view],
  );
  const navigate = (f: string, a = false) => {
    setFilesMode(!view?.entries.length);
    setFolder(f);
    setAll(a);
    setReader(null);
    save("", a ? "*" : f);
  };
  const read = (e: Entry) => {
    setSelected(e.id);
    setReader(e.id);
    save(e.id, all ? "*" : folder);
  };
  const current = entries.findIndex((e) => e.id === reader);
  const resume = () => {
    const e = view?.entries.find((e) => e.id === selected);
    if (e) {
      setAll(false);
      setFolder(e.folder);
      setReader(e.id);
      save(e.id, e.folder);
    }
  };
  return (
    <div className="app">
      <aside>
        <div className="sidebar-title">ZipGallery</div>
        <button className="open-button" onClick={() => open()} disabled={busy}>
          {t("openArchive")} <kbd>⌘O</kbd>
        </button>
        <div className="aside-label">{t("currentArchive")}</div>
        <button
          className={!all ? "nav active" : "nav"}
          onClick={() => navigate("")}
        >
          {t("folders")}
        </button>
        <button
          className={all ? "nav active" : "nav"}
          onClick={() => navigate("", true)}
        >
          {t("allImages")} <span>{view?.entries.length || 0}</span>
        </button>
        <div className="aside-label">
          {t("recents")} <span>{recents.length}</span>
        </div>
        <div className="recents">
          {recents.length === 0 ? (
            <p className="muted">{t("noRecents")}</p>
          ) : (
            recents.map((r) => (
              <button
                key={r.location.path}
                className={
                  "recent " +
                  (view?.location.path === r.location.path ? "current" : "")
                }
                title={r.location.path}
                onClick={() => open(r.location.path)}
              >
                <strong>{r.location.path.split("/").at(-1)}</strong>
                <small>
                  {r.entry ? t("hasProgress") : ""}
                  {new Date(r.updated * 1000).toLocaleDateString(locale)}
                </small>
              </button>
            ))
          )}
        </div>
        <div className="aside-bottom">
          <select
            aria-label={t("language")}
            value={locale}
            onChange={(e) => setLocale(e.target.value as "zh-CN" | "en")}
          >
            <option value="zh-CN">简体中文</option>
            <option value="en">English</option>
          </select>
          <button
            onClick={async () => {
              try {
                await Desktop.clearCache();
                setNotice(t("cacheCleared"));
              } catch (e) {
                setError(message(e));
              }
            }}
          >
            {t("clearCache")}
          </button>
        </div>
      </aside>
      <main>
        <ArchiveTools
          view={view}
          open={open}
          mounted={async () => {
            try {
              const p = await Desktop.selectMounted(t("mountedShares"));
              if (p) void open(p);
            } catch (e) {
              setError(message(e));
            }
          }}
        />
        <header>
          <div className="history-controls">
            <button
              title={t("backArchive") + " (Alt+←)"}
              aria-label={t("backArchive")}
              disabled={!back.current.length || busy}
              onClick={() => travel("back")}
            >
              ←
            </button>
            <button
              title={t("forwardArchive") + " (Alt+→)"}
              aria-label={t("forwardArchive")}
              disabled={!forward.current.length || busy}
              onClick={() => travel("forward")}
            >
              →
            </button>
          </div>
          <h1 title={view?.location.path}>{view?.name || "ZipGallery"}</h1>
          {view && (
            <div className="header-actions">
              <button
                disabled={!view.entries.length}
                onClick={() => setFilesMode(!filesMode)}
              >
                {t(filesMode ? "images" : "fileList")}
              </button>
              <button
                disabled={!/\.(zip|cbz)$/i.test(view.location.path)}
                onClick={() => setRepair(true)}
              >
                {t("repair")}
              </button>
              {selected && <button onClick={resume}>{t("resume")}</button>}
              <button
                disabled={busy}
                onClick={async () => {
                  try {
                    await Desktop.clearCache();
                    await open(view.location.path);
                  } catch (e) {
                    setError(message(e));
                  }
                }}
              >
                {t("refresh")}
              </button>
            </div>
          )}
        </header>
        {error && !locked && (
          <div className="banner error" role="alert">
            {error}
            <button onClick={() => open()}>{t("chooseAgain")}</button>
            <button onClick={() => setError("")}>{t("close")}</button>
          </div>
        )}
        {notice && (
          <div className="banner">
            {notice}
            <button onClick={() => setNotice("")}>{t("close")}</button>
          </div>
        )}
        {busy && <div className="banner">{t("loadingIndex")}</div>}
        {view ? (
          <>
            <div className="breadcrumbs">
              <button
                disabled={!folder || all}
                onClick={() =>
                  navigate(folder.split("/").slice(0, -1).join("/"))
                }
              >
                ↑
              </button>
              <button onClick={() => navigate("")}>{t("root")}</button>
              {all ? (
                <span>/ {t("allImages")}</span>
              ) : (
                folder
                  .split("/")
                  .filter(Boolean)
                  .map((p, i) => (
                    <React.Fragment key={i}>
                      <span>/</span>
                      <button
                        onClick={() =>
                          navigate(
                            folder
                              .split("/")
                              .slice(0, i + 1)
                              .join("/"),
                          )
                        }
                      >
                        {p}
                      </button>
                    </React.Fragment>
                  ))
              )}
              <span className="count">
                {filesMode
                  ? t("fileCount", { count: view.files?.length || 0 })
                  : t("imageCount", { count: entries.length })}
                {!filesMode && tiles.length > entries.length
                  ? t("directoryCount", {
                      count: tiles.length - entries.length,
                    })
                  : ""}
              </span>
            </div>
            {filesMode ? (
              <FileList view={view} />
            ) : (
              <Grid
                key={view.id}
                tiles={tiles}
                selected={selected}
                onFolder={(f) => navigate(f)}
                onImage={read}
                folder={all ? "*" : folder}
                positions={positions}
              />
            )}
            <footer>
              {filesMode
                ? t("fileCount", { count: view.files?.length || 0 })
                : t("imageCount", { count: view.entries.length })}
              <span>{view.location.path}</span>
            </footer>
          </>
        ) : (
          <div className="welcome">
            <h2>{t("openTitle")}</h2>
            <p>{t("dropHint")}</p>
            <button onClick={() => open()}>{t("openFile")}</button>
          </div>
        )}
      </main>
      {locked && (
        <div className="modal-shade">
          <form
            className="repair-dialog"
            onSubmit={async (e) => {
              e.preventDefault();
              try {
                const v = await Desktop.openArchive(locked, password);
                if (snapshot.current) back.current.push(snapshot.current);
                adopt(v);
                setLocked("");
                setPassword("");
                setError("");
              } catch (e) {
                setError(message(e));
              }
            }}
          >
            <h2>{t("unlock")}</h2>
            <input
              type="password"
              aria-label={t("password")}
              autoFocus
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
            {error && <p>{error}</p>}
            <button type="submit">{t("openFile")}</button>
            <button
              type="button"
              onClick={() => {
                setLocked("");
                setPassword("");
              }}
            >
              {t("cancel")}
            </button>
          </form>
        </div>
      )}
      {repair && view && (
        <Repair
          path={view.location.path}
          onClose={() => setRepair(false)}
          onSaved={(p) => {
            setRepair(false);
            setNotice(t("repairSaved", { path: p }));
            void open(p);
          }}
        />
      )}
      {current >= 0 && (
        <Viewer
          entries={entries}
          index={current}
          onIndex={(i) => read(entries[i])}
          onClose={() => {
            setReader(null);
            Desktop.recents()
              .then(setRecents)
              .catch(() => {});
          }}
        />
      )}
    </div>
  );
}
createRoot(document.getElementById("root")!).render(<App />);
