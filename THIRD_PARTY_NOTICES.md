# Third-party components

ZipGallery's original code, documentation and original SVG artwork are licensed under the MIT License in `LICENSE`. Third-party software retains its own copyright and license; the MIT license does not relicense those components.

## Bundled 7-Zip

The app executes the unmodified, replaceable official 7zz 26.04 program by Igor Pavlov. It is distributed under LGPL 2.1 or later, additional unRAR restrictions for relevant code, and BSD licenses for relevant code. See `third_party/7zip/License.txt`, `LGPL-2.1.txt`, and `PROVENANCE.txt`. The exact corresponding source archive is included as `third_party/7zip/7z2604-src.tar.xz`. These files are also included in the application Resources directory. ZipGallery does not include or derive code from MacPacker.

## Application dependencies

Full upstream notices are preserved in `third_party/licenses/` and copied into the application's Resources directory. They cover Go, MyGo, purego, imaging, golang.org/x packages, React, React DOM, Scheduler, mygo-runtime, TanStack React Virtual and Virtual Core. Exact dependency versions are recorded in `go.mod`, `go.sum`, `package.json` and `bun.lock`.

## Build tools

Build tools are installed separately and retain their upstream licenses. In particular, `@resvg/resvg-js` 2.6.2 is MPL-2.0 licensed and is used to render the project's original SVG icons during development; its code and native binary are not included in the shipped application. Vite, TypeScript, Bun and other build dependencies are not covered by ZipGallery's MIT grant.
