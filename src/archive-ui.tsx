import React, { useEffect, useState, useRef } from "react";
import { Desktop, type View, type Job } from "./mygo";
import { t, errorMessage } from "./i18n";
export function ArchiveTools({
  view,
  open,
  mounted,
}: {
  view: View | null;
  open: (p: string) => void;
  mounted: () => void;
}) {
  const dismissed = useRef("");
  const [mode, setMode] = useState(""),
    [inputs, setInputs] = useState<string[]>([]),
    [format, setFormat] = useState("zip"),
    [password, setPassword] = useState(""),
    [error, setError] = useState(""),
    [job, setJob] = useState<Job | null>(null),
    [working, setWorking] = useState(false);
  useEffect(() => {
    let active = true;
    const id = setInterval(
      () =>
        Desktop.jobStatus()
          .then((j) => {
            if (active && j.id && j.id !== dismissed.current) setJob(j);
          })
          .catch(() => {}),
      250,
    );
    return () => {
      active = false;
      clearInterval(id);
    };
  }, []);
  const start = async () => {
    setWorking(true);
    setError("");
    try {
      const j = await Desktop.startArchive(
        mode === "extract" && view ? [view.location.path] : inputs,
        format,
        password,
        t("outputTitle"),
        mode === "extract",
      );
      if (j.id) {
        setJob(j);
        setMode("");
        setPassword("");
      }
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setWorking(false);
    }
  };
  return (
    <>
      <div className="archive-tools">
        <button
          onClick={() => {
            setMode("create");
            setError("");
          }}
        >
          {t("createArchive")}
        </button>
        <button
          disabled={!view || job?.state === "running"}
          onClick={() => {
            setMode("extract");
            setError("");
          }}
        >
          {t("extractArchive")}
        </button>
        <button onClick={mounted}>{t("mountedShares")}</button>
      </div>
      {job && (
        <div className="job-panel" role="status">
          <span>{t(("job_" + job.state) as "job_running")}</span>
          <progress max={100} value={job.percent} />
          <span>{job.percent}%</span>
          {job.state === "running" ? (
            <button onClick={() => Desktop.cancelJob()}>{t("cancel")}</button>
          ) : (
            <button
              onClick={() => {
                dismissed.current = job.id;
                setJob(null);
              }}
            >
              {t("close")}
            </button>
          )}
          {job.error && <span>{errorMessage(job.error)}</span>}
          {job.state === "succeeded" && <small>{job.output}</small>}
        </div>
      )}
      {mode && (
        <div className="modal-shade">
          <section
            className="repair-dialog"
            role="dialog"
            aria-label={t(
              mode === "create" ? "createArchive" : "extractArchive",
            )}
          >
            <h2>{t(mode === "create" ? "createArchive" : "extractArchive")}</h2>
            {mode === "create" && (
              <>
                <button
                  disabled={working}
                  onClick={async () => {
                    try {
                      setInputs(
                        await Desktop.selectInputs(t("selectInputs"), false),
                      );
                    } catch (e) {
                      setError(errorMessage(e));
                    }
                  }}
                >
                  {t("selectFiles")}
                </button>{" "}
                <button
                  disabled={working}
                  onClick={async () => {
                    try {
                      setInputs(
                        await Desktop.selectInputs(t("selectInputs"), true),
                      );
                    } catch (e) {
                      setError(errorMessage(e));
                    }
                  }}
                >
                  {t("selectFolder")}
                </button>
                <p>{inputs.map((p) => p.split("/").at(-1)).join(", ")}</p>
                <label>
                  {t("format")}{" "}
                  <select
                    value={format}
                    onChange={(e) => setFormat(e.target.value)}
                  >
                    <option>zip</option>
                    <option>7z</option>
                    <option>tar.gz</option>
                  </select>
                </label>
              </>
            )}
            <p>{t(mode === "extract" ? "extractHint" : "createHint")}</p>
            {(mode === "extract" || format !== "tar.gz") && (
              <label>
                {t("password")}{" "}
                <input
                  type="password"
                  autoComplete="off"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </label>
            )}
            {error && <p role="alert">{error}</p>}
            <div className="repair-actions">
              <button
                disabled={working}
                onClick={() => {
                  setMode("");
                  setPassword("");
                }}
              >
                {t("cancel")}
              </button>
              <button
                disabled={working || (mode === "create" && !inputs.length)}
                onClick={start}
              >
                {t("start")}
              </button>
            </div>
          </section>
        </div>
      )}
    </>
  );
}
export function FileList({ view }: { view: View }) {
  const [query, setQuery] = useState("");
  return (
    <div className="file-list">
      <input
        aria-label={t("filterFiles")}
        placeholder={t("filterFiles")}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
      />
      <table>
        <thead>
          <tr>
            <th>{t("filename")}</th>
            <th>{t("size")}</th>
            <th>{t("password")}</th>
          </tr>
        </thead>
        <tbody>
          {(view.files || [])
            .filter((f) =>
              f.path.toLocaleLowerCase().includes(query.toLocaleLowerCase()),
            )
            .map((f, i) => (
              <tr key={i}>
                <td>
                  {f.folder ? "▱ " : ""}
                  {f.path}
                </td>
                <td>
                  {f.folder
                    ? "—"
                    : new Intl.NumberFormat().format(f.size) + " B"}
                </td>
                <td>{f.encrypted ? "●" : ""}</td>
              </tr>
            ))}
        </tbody>
      </table>
    </div>
  );
}
