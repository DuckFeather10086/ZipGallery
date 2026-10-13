import { defineConfig } from "mygo-cli";
const archiveTypes = [
  { ext: "zip", uti: "public.zip-archive", name: "ZIP archive" },
  { ext: "cbz", uti: "cx.c3.cbz-archive", name: "Comic Book ZIP" },
  { ext: "7z", uti: "org.7-zip.7-zip-archive", name: "7-Zip archive" },
  { ext: "rar", uti: "com.rarlab.rar-archive", name: "RAR archive" },
  { ext: "tar", uti: "public.tar-archive", name: "TAR archive" },
  { ext: "gz", uti: "org.gnu.gnu-zip-archive", name: "Gzip archive" },
  { ext: "bz2", uti: "public.bzip2-archive", name: "Bzip2 archive" },
  { ext: "xz", uti: "org.tukaani.xz-archive", name: "XZ archive" },
  {
    ext: "tgz",
    uti: "org.gnu.gnu-zip-tar-archive",
    name: "Compressed TAR archive",
  },
];
export default defineConfig({
  name: process.env.ZIPGALLERY_QA ? "ZipGallery QA" : "ZipGallery",
  identifier: process.env.ZIPGALLERY_QA
    ? "dev.zipgallery.qa"
    : "dev.zipgallery.app",
  icon: "resources/icon.png",
  resources: [
    "third_party/7zip",
    "third_party/licenses",
    "LICENSE",
    "THIRD_PARTY_NOTICES.md",
  ],
  version: "0.1.1",
  devUrl: "http://localhost:5173",
  devCommand: "bun run dev:web",
  buildCommand: "bun run build:web",
  frontendDist: "dist",
  bindings: "src/mygo.ts",
  out: process.env.ZIPGALLERY_QA ? "build/qa" : "build",
  macos: {
    infoPlist: {
      CFBundleDocumentTypes: archiveTypes.map(({ ext, uti, name }) => ({
        CFBundleTypeName: name,
        CFBundleTypeRole: "Viewer",
        LSHandlerRank: process.env.ZIPGALLERY_QA ? "Alternate" : "Default",
        CFBundleTypeExtensions: [ext],
        LSItemContentTypes: [uti],
        CFBundleTypeIconFile: "ArchiveDocument.icns",
      })),
      UTImportedTypeDeclarations: archiveTypes.map(({ ext, uti, name }) => ({
        UTTypeIdentifier: uti,
        UTTypeDescription: name,
        UTTypeConformsTo: ["public.archive"],
        UTTypeIconFile: "ArchiveDocument.icns",
        UTTypeTagSpecification: { "public.filename-extension": [ext] },
      })),
    },
  },
  fileAssociations: [
    {
      ext: ["cbz"],
      name: "Comic Book ZIP",
      mimeType: "application/vnd.comicbook+zip",
    },
    {
      ext: ["zip", "7z", "rar", "tar", "gz", "bz2", "xz", "tgz"],
      name: "Archives",
      mimeType: "application/octet-stream",
    },
  ],
});
