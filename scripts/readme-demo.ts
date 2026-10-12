// Synthetic, reproducible artwork for public documentation screenshots.
import { resolve } from "node:path";
import { Resvg } from "@resvg/resvg-js";
import { mkdir, writeFile, rm } from "node:fs/promises";
import { execFileSync } from "node:child_process";
const dir = "fixtures/readme-demo";
await mkdir(dir, { recursive: true });
const palettes = [
  ["#c5dce4", "#f4dfae", "#789599", "#355c65"],
  ["#efc9aa", "#fff1c7", "#b78371", "#684f68"],
  ["#c3d0d9", "#edf1d7", "#859bab", "#405a76"],
  ["#dcc4ac", "#fff1d0", "#a09672", "#526957"],
  ["#b8d8d1", "#ffe0a1", "#6eaba3", "#376a75"],
  ["#d4cce1", "#ffe3b5", "#a19bb7", "#655e87"],
];
for (let i = 0; i < palettes.length; i++) {
  const [sky, sun, hill, front] = palettes[i];
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1400" height="1000" viewBox="0 0 1400 1000"><rect width="1400" height="1000" fill="${sky}"/><circle cx="${980 - i * 100}" cy="260" r="100" fill="${sun}"/><path d="M0 650 300 260 650 650 960 380 1400 730V1000H0Z" fill="${hill}"/><path d="m0 760 300-190 330 200 370-250 400 150v330H0Z" fill="${front}"/><path d="M0 840q350-130 700 0t700 0v160H0Z" fill="${sky}" opacity=".7"/><text x="80" y="115" font-family="sans-serif" font-size="25" letter-spacing="7" fill="${front}">ZIPGALLERY / STUDIES</text><text x="80" y="930" font-family="sans-serif" font-size="36" fill="${front}">Landscape ${String(i + 1).padStart(2, "0")}</text></svg>`;
  await writeFile(
    `${dir}/${String(i + 1).padStart(2, "0")}-landscape.png`,
    new Resvg(svg).render().asPng(),
  );
}
await rm("fixtures/ZipGallery-Demo.zip", { force: true });
execFileSync(
  resolve("third_party/7zip/7zz"),
  ["a", "-tzip", resolve("fixtures/ZipGallery-Demo.zip"), "*.png"],
  { stdio: "ignore", cwd: dir },
);
