import { Resvg } from "@resvg/resvg-js";
import { readFile, writeFile, mkdir, rm } from "node:fs/promises";
import { execFileSync } from "node:child_process";
const app = await readFile("resources/icon.svg");
await writeFile("resources/icon.png", new Resvg(app).render().asPng());
const document = await readFile("resources/archive.svg");
await writeFile("resources/archive.png", new Resvg(document).render().asPng());
const dir = "resources/ArchiveDocument.iconset";
await mkdir(dir, { recursive: true });
try {
  for (const size of [16, 32, 128, 256, 512])
    for (const scale of [1, 2]) {
      const png = new Resvg(document, {
        fitTo: { mode: "width", value: size * scale },
      })
        .render()
        .asPng();
      await writeFile(
        `${dir}/icon_${size}x${size}${scale === 2 ? "@2x" : ""}.png`,
        png,
      );
    }
  execFileSync("/usr/bin/iconutil", [
    "-c",
    "icns",
    dir,
    "-o",
    "resources/ArchiveDocument.icns",
  ]);
} finally {
  await rm(dir, { recursive: true, force: true });
}
