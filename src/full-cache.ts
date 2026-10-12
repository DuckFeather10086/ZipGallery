// Small window of decoded images: prefetch work is reused by the visible page.
const cache = new Map<string, Promise<string>>();
const urls = new Map<string, string>();
const controllers = new Map<string, AbortController>();
export function fullImage(key: string, url: string) {
  const existing = cache.get(key);
  if (existing) return existing;
  const controller = new AbortController();
  controllers.set(key, controller);
  const task = fetch(url, { signal: controller.signal })
    .then(async (r) => {
      if (!r.ok) throw Error(await r.text());
      return r.blob();
    })
    .then(async (b) => {
      const u = URL.createObjectURL(b);
      const img = new Image();
      img.src = u;
      try {
        await img.decode();
      } catch (e) {
        URL.revokeObjectURL(u);
        throw e;
      }
      if (cache.get(key) !== task) {
        URL.revokeObjectURL(u);
        throw Error("E_SESSION");
      }
      urls.set(key, u);
      return u;
    })
    .catch((e) => {
      if (cache.get(key) === task) {
        cache.delete(key);
        controllers.delete(key);
      }
      throw e;
    });
  cache.set(key, task);
  return task;
}
export function keepFullImages(keys: string[]) {
  for (const key of cache.keys()) {
    if (!keys.includes(key)) {
      cache.delete(key);
      controllers.get(key)?.abort();
      controllers.delete(key);
      const u = urls.get(key);
      if (u) URL.revokeObjectURL(u);
      urls.delete(key);
    }
  }
}
export function cachedFull(key: string) {
  return urls.get(key) || "";
}
