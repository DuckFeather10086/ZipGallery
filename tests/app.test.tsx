import { test, expect } from "bun:test";
import { Window } from "happy-dom";
import type { View } from "../src/mygo";

const dom = new Window({ url: "http://localhost" });
for (const key of [
  "window",
  "document",
  "navigator",
  "HTMLElement",
  "HTMLInputElement",
  "Event",
  "MouseEvent",
  "KeyboardEvent",
  "localStorage",
  "ResizeObserver",
] as const) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: key === "window" ? dom : dom[key],
  });
}
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });
const { act } = await import("react");
const { createRoot } = await import("react-dom/client");
const { App } = await import("../src/main");

function view(path: string, id = path): View {
  return {
    id,
    name: path.slice(1),
    location: { kind: "local", path },
    folder: "chapter/nested",
    progress: "chapter/nested/1.jpg#1",
    entries: [
      {
        id: `${id}-1`,
        name: "chapter/nested/1.jpg",
        folder: "chapter/nested",
        identity: "chapter/nested/1.jpg#1",
      },
    ],
    files: [
      {
        path: "chapter/nested/1.jpg",
        size: 32,
        folder: false,
        encrypted: path === "/secret.zip",
      },
    ],
  };
}

test("folder and archive history, failed navigation, unlocking, and extraction dialogs", async () => {
  let current = view("/a.zip");
  let chosen = "/b.zip";
  let blocked = "";
  let unlocked = false;
  const opened: string[] = [];
  const extracted: unknown[][] = [];
  Object.assign(globalThis, {
    mygo: {
      platform: "darwin",
      on: () => () => {},
      call: async (method: string, ...args: unknown[]) => {
        switch (method) {
          case "Desktop.Current":
            return current;
          case "Desktop.Recents":
            return [];
          case "Desktop.Pending":
            return "";
          case "Desktop.JobStatus":
            return {};
          case "Desktop.Progress":
            return;
          case "Desktop.SelectArchive":
            return chosen;
          case "Desktop.StartArchive":
            extracted.push(args);
            return {
              id: "job",
              state: "succeeded",
              percent: 100,
              output: "/out",
              error: "",
            };
          case "Desktop.OpenArchive": {
            const path = args[0] as string;
            opened.push(path);
            if (path === blocked) throw Error("E_UNREACHABLE");
            if (path === "/secret.zip" && !unlocked) {
              if (args[1] !== "correct") throw Error("E_PASSWORD");
              unlocked = true;
            }
            return (current = view(path, String(opened.length)));
          }
          default:
            throw Error(`Unexpected bridge call: ${method}`);
        }
      },
    },
  });
  const host = document.createElement("div");
  document.body.append(host);
  const root = createRoot(host);
  const button = (text: string, selector = "button") => {
    const result = [...host.querySelectorAll<HTMLButtonElement>(selector)].find(
      (b) => b.textContent?.trim() === text,
    );
    if (!result) throw Error(`Missing button: ${text}`);
    return result;
  };
  const history = (direction: "后退" | "前进") =>
    host.querySelector<HTMLButtonElement>(`[aria-label="${direction}"]`)!;
  const click = async (b: HTMLButtonElement) => {
    await act(async () => b.click());
  };
  const title = () => host.querySelector("h1")?.textContent;
  const crumbs = () => host.querySelector(".breadcrumbs")?.textContent || "";
  const submitPassword = async (password: string) => {
    const input = host.querySelector<HTMLInputElement>(
      'input[type="password"]',
    )!;
    await act(async () => {
      Object.getOwnPropertyDescriptor(
        HTMLInputElement.prototype,
        "value",
      )!.set!.call(input, password);
      input.dispatchEvent(new dom.Event("input", { bubbles: true }));
    });
    await act(async () =>
      host
        .querySelector("form")!
        .dispatchEvent(
          new dom.Event("submit", { bubbles: true, cancelable: true }),
        ),
    );
  };
  try {
    await act(async () => root.render(<App />));
    expect(crumbs()).toContain("nested");
    expect(history("后退").disabled).toBe(true);
    const grid = host.querySelector<HTMLElement>(".grid-scroll")!;
    grid.scrollTop = 450;
    await act(async () =>
      grid.dispatchEvent(new dom.Event("scroll", { bubbles: true })),
    );
    await click(button("↑", ".breadcrumbs button"));
    expect(crumbs()).not.toContain("nested");
    await click(history("后退"));
    expect(crumbs()).toContain("nested");
    expect(host.querySelector<HTMLElement>(".grid-scroll")!.scrollTop).toBe(
      450,
    );
    await click(history("前进"));
    expect(crumbs()).not.toContain("nested");
    expect(opened).toEqual([]); // Folder navigation preserves the live backend session.
    await act(async () =>
      dom.dispatchEvent(
        new dom.KeyboardEvent("keydown", {
          key: "ArrowLeft",
          altKey: true,
          bubbles: true,
        }),
      ),
    );
    expect(crumbs()).toContain("nested");
    await click(button("根目录", ".breadcrumbs button"));
    expect(history("前进").disabled).toBe(true); // A new branch discards forward history.
    await click(host.querySelector<HTMLButtonElement>(".open-button")!);
    expect(title()).toBe("b.zip");
    blocked = "/a.zip";
    await click(history("后退"));
    expect(title()).toBe("b.zip");
    expect(history("后退").disabled).toBe(false);
    expect(history("前进").disabled).toBe(true);
    blocked = "";
    await click(history("后退"));
    expect(title()).toBe("a.zip");
    expect(crumbs()).not.toContain("chapter");
    await click(history("前进"));
    expect(title()).toBe("b.zip");
    expect(crumbs()).toContain("nested");
    await click(button("解压", ".archive-tools button"));
    expect(host.querySelector('input[type="password"]')).toBeNull();
    await click(button("开始", ".repair-dialog button"));
    expect(extracted[0]?.slice(0, 3)).toEqual([["/b.zip"], "zip", ""]);
    chosen = "/secret.zip";
    await click(host.querySelector<HTMLButtonElement>(".open-button")!);
    expect(host.querySelector('input[type="password"]')).not.toBeNull();
    await submitPassword("wrong");
    expect(title()).toBe("b.zip");
    expect(host.querySelector("form")?.textContent).toContain("密码");
    await submitPassword("correct");
    expect(title()).toBe("secret.zip");
    expect(host.querySelector("form")).toBeNull();
    await click(history("后退"));
    expect(title()).toBe("b.zip");
    unlocked = false;
    await click(history("前进"));
    expect(title()).toBe("b.zip");
    await click(button("取消", "form button"));
    expect(history("前进").disabled).toBe(false);
    await click(history("前进"));
    await submitPassword("correct");
    expect(title()).toBe("secret.zip");
    expect(history("前进").disabled).toBe(true);
    await click(button("解压", ".archive-tools button"));
    expect(host.querySelector('input[type="password"]')).toBeNull();
    await click(button("取消", ".repair-dialog button"));
    await click(button("新建压缩包", ".archive-tools button"));
    expect(host.querySelector('input[type="password"]')).not.toBeNull();
  } finally {
    await act(async () => root.unmount());
    host.remove();
    await dom.happyDOM.abort();
  }
});
