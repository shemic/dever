import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react-swc";
import { defineConfig, normalizePath, type PluginOption } from "vite";
import { rewriteCompatImports } from "./src/compat-import";
import { readFrontSDKExports } from "./src/sdk-exports";

const compilerRoot = path.dirname(fileURLToPath(import.meta.url));
const pluginRoot = process.env.DEVER_FRONT_PLUGIN_ROOT || "";
const pluginSourceRoots = resolvePluginSourceRoots();
const pluginName = process.env.DEVER_FRONT_PLUGIN_NAME || "plugin";
const projectRoot =
  process.env.DEVER_FRONT_PLUGIN_PROJECT_ROOT ||
  path.resolve(compilerRoot, "..", "..", "..", "..");
const frontPackageRoot = resolveFrontPackageRoot();
const sdkEntry = path.resolve(frontPackageRoot, "sdk", "src", "index.ts");
const sdkCoreEntry = path.resolve(frontPackageRoot, "sdk", "src", "core.ts");
const {
  coreExports: sdkCoreExports,
  compatExports: sdkCompatExports,
} = readFrontSDKExports(sdkCoreEntry, sdkEntry);
const compatSourceRoots = Array.from(
  new Set([...pluginSourceRoots, normalizePath(path.dirname(sdkEntry))]),
);
const shimRoot = path.resolve(compilerRoot, "src", "shims");
const runtimeEntryFile = path.resolve(compilerRoot, "src", "runtime-entry.ts");

const runtimeEntryID = "virtual:dever-front-plugin-runtime";
const resolvedRuntimeEntryID = "\0" + runtimeEntryID;
const pluginEntry = pluginRoot ? path.join(pluginRoot, "src", "plugin.ts") : "";
const splitPluginBundle = readPluginBundleMode() === "split";
const splitPluginMinChunkSize = 24 * 1024;
const devServerAllowedRoots = Array.from(
  new Set(
    [projectRoot, frontPackageRoot, compilerRoot, ...pluginSourceRoots]
      .filter(Boolean)
      .map((root) => path.resolve(root)),
  ),
);

const compatModulePrefix = "virtual:dever-front-compat:";
const resolvedCompatModulePrefix = "\0" + compatModulePrefix;
const compatModuleSources = new Set<string>();
const moduleCompatMode = "module";
const sdkModuleSource = "@dever/front-plugin";

type PluginBundleMode = "single" | "split";

const canvasDependencies = new Set([
  "@xyflow/react",
  "@xyflow/system",
]);
const assistantDependencies = new Set([
  "assistant-cloud",
  "assistant-stream",
  "react-markdown",
]);
const assistantDependencyPrefixes = [
  "@assistant-ui/",
  "mdast-",
  "micromark-",
  "rehype-",
  "remark-",
  "unist-",
];

const shimModuleFiles: Record<string, string> = {
  react: "react.ts",
  "react-jsx-runtime": "react-jsx-runtime.ts",
  "react-dom": "react-dom.ts",
  "react-dom-client": "react-dom-client.ts",
};

function resolveFrontPackageRoot() {
  const configured = process.env.DEVER_FRONT_PACKAGE_ROOT || "";
  if (hasFrontSDK(configured)) {
    return path.resolve(configured);
  }

  for (const candidate of [
    path.resolve(projectRoot, "package", "front"),
    path.resolve(projectRoot, "backend", "package", "front"),
  ]) {
    if (hasFrontSDK(candidate)) {
      return candidate;
    }
  }

  return path.resolve(projectRoot, "package", "front");
}

function hasFrontSDK(root: string) {
  if (!root) {
    return false;
  }
  return fs.existsSync(path.resolve(root, "sdk", "src", "index.ts"));
}

function resolvePluginSourceRoots() {
  const configuredRoots = (process.env.DEVER_FRONT_PLUGIN_ROOTS || "")
    .split(path.delimiter)
    .map((root) => root.trim())
    .filter(Boolean);
  return Array.from(
    new Set(
      [pluginRoot, ...configuredRoots]
        .map((root) => root.trim())
        .filter(Boolean)
        .map((root) => normalizePath(path.resolve(root))),
    ),
  );
}

const pluginOptimizedDeps = [
  "@xyflow/react",
  "lucide-react",
  "sonner",
  "zustand",
  "zustand/react",
  "zustand/vanilla",
];
const runtimeOwnedDependencies = new Set([
  "@dever/front-plugin",
  "@vitejs/plugin-react-swc",
  ...pluginOptimizedDeps,
  "react",
  "react-dom",
  "react-dom/client",
  "react/jsx-dev-runtime",
  "react/jsx-runtime",
  "typescript",
  "vite",
]);
const frontPluginDependencyNames = readCompilerDependencyNames();
const optimizedPluginDependencyNames = uniqueDependencyNames([
  ...pluginOptimizedDeps,
  ...frontPluginDependencyNames,
]);

function uniqueDependencyNames(names: string[]) {
  return Array.from(
    new Set(names.map((name) => name.trim()).filter(Boolean)),
  ).sort();
}

function readCompilerDependencyNames() {
  const names = new Set<string>();
  const manifest = readJSONFile(path.join(compilerRoot, "package.json"));
  const dependencies = plainObject(manifest?.dependencies);
  for (const name of Object.keys(dependencies)) {
    const normalized = name.trim();
    if (normalized && !runtimeOwnedDependencies.has(normalized)) {
      names.add(normalized);
    }
  }
  return Array.from(names).sort();
}

function readJSONFile(file: string) {
  try {
    return JSON.parse(fs.readFileSync(file, "utf8"));
  } catch {
    return null;
  }
}

function plainObject(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {};
  }
  return value as Record<string, unknown>;
}

function readPluginBundleMode(): PluginBundleMode {
  if (!pluginRoot) {
    return "single";
  }
  const manifest = plainObject(
    readJSONFile(path.join(pluginRoot, "package.json")),
  );
  const dever = plainObject(manifest.dever);
  return dever.bundle === "split" ? "split" : "single";
}

function packageNameFromModuleID(id: string) {
  const normalizedID = normalizePath(id);
  const marker = "/node_modules/";
  const markerIndex = normalizedID.lastIndexOf(marker);
  if (markerIndex === -1) {
    return "";
  }

  const packagePath = normalizedID.slice(markerIndex + marker.length);
  const segments = packagePath.split("/");
  return segments[0]?.startsWith("@")
    ? `${segments[0]}/${segments[1] || ""}`
    : segments[0] || "";
}

function pluginManualChunk(id: string) {
  const dependencyName = packageNameFromModuleID(id);
  if (!dependencyName) {
    return undefined;
  }
  if (canvasDependencies.has(dependencyName)) {
    return "vendor-canvas";
  }
  if (
    assistantDependencies.has(dependencyName) ||
    assistantDependencyPrefixes.some((prefix) =>
      dependencyName.startsWith(prefix),
    )
  ) {
    return "vendor-assistant";
  }
  return undefined;
}

function splitImportSpecifier(source: string) {
  const suffixIndex = source.search(/[?#]/);
  if (suffixIndex === -1) {
    return { id: source, suffix: "" };
  }
  return {
    id: source.slice(0, suffixIndex),
    suffix: source.slice(suffixIndex),
  };
}

function resolveFrontPluginDependencySubpath(source: string) {
  const { id, suffix } = splitImportSpecifier(source);
  for (const name of frontPluginDependencyNames) {
    if (!id.startsWith(`${name}/`)) {
      continue;
    }
    const subpath = id.slice(name.length + 1);
    return normalizePath(path.join(dependency(name), subpath)) + suffix;
  }
  return "";
}

function rewriteDependencySubpathImports(code: string) {
  let changed = false;
  const rewrite = (
    match: string,
    prefix: string,
    quote: string,
    source: string,
  ) => {
    const resolved = resolveFrontPluginDependencySubpath(source);
    if (!resolved) {
      return match;
    }
    changed = true;
    return `${prefix}${quote}${resolved}${quote}`;
  };
  const rewritten = code
    .replace(
      /(\b(?:import|export)\s+[^'"]*\bfrom\s*)(["'])([^"']+)\2/g,
      rewrite,
    )
    .replace(/(\bimport\s*)(["'])([^"']+)\2/g, rewrite)
    .replace(/(\bimport\s*\(\s*)(["'])([^"']+)\2/g, rewrite);
  return changed ? rewritten : null;
}

type PluginMetadata = {
  name: string;
  compat?: string[];
  nodes?: string[];
  depends?: string[];
};

function readPluginMetadata(): PluginMetadata {
  const fallback: PluginMetadata = { name: pluginName };
  if (!pluginEntry || !fs.existsSync(pluginEntry)) {
    return fallback;
  }

  const content = fs.readFileSync(pluginEntry, "utf8");
  const metadata: PluginMetadata = {
    name: extractStringProperty(content, "name") || pluginName,
  };
  const nodesBlock = extractPropertyBlock(content, "nodes", "{", "}");
  if (nodesBlock) {
    metadata.nodes = extractObjectStringKeys(nodesBlock);
  }
  const dependsBlock = extractPropertyBlock(content, "depends", "[", "]");
  if (dependsBlock) {
    metadata.depends = extractStringLiterals(dependsBlock);
  }
  return metadata;
}

function extractStringProperty(content: string, key: string) {
  const match = new RegExp(
    `\\b${escapeRegExp(key)}\\s*:\\s*${stringLiteralPattern()}`,
    "m",
  ).exec(content);
  return match?.[1]?.trim() || "";
}

function extractPropertyBlock(
  content: string,
  key: string,
  open: "{" | "[",
  close: "}" | "]",
) {
  const pattern = new RegExp(`\\b${escapeRegExp(key)}\\s*:`, "gm");
  let match: RegExpExecArray | null;
  while ((match = pattern.exec(content))) {
    let index = match.index + match[0].length;
    while (index < content.length && /\s/.test(content[index])) {
      index++;
    }
    if (content[index] !== open) {
      continue;
    }
    return matchDelimitedBlock(content, index, open, close);
  }
  return "";
}

function matchDelimitedBlock(
  content: string,
  start: number,
  open: string,
  close: string,
) {
  let depth = 0;
  let quote = "";
  let escaped = false;
  for (let index = start; index < content.length; index++) {
    const current = content[index];
    if (quote) {
      if (escaped) {
        escaped = false;
        continue;
      }
      if (current === "\\") {
        escaped = true;
        continue;
      }
      if (current === quote) {
        quote = "";
      }
      continue;
    }
    if (current === '"' || current === "'" || current === "`") {
      quote = current;
      continue;
    }
    if (current === open) {
      depth++;
    }
    if (current === close) {
      depth--;
      if (depth === 0) {
        return content.slice(start, index + 1);
      }
    }
  }
  return "";
}

function extractObjectStringKeys(block: string) {
  const result: string[] = [];
  const pattern = new RegExp(`${stringLiteralPattern()}\\s*:`, "g");
  let match: RegExpExecArray | null;
  while ((match = pattern.exec(block))) {
    result.push(match[1]);
  }
  return uniqueDependencyNames(result);
}

function extractStringLiterals(block: string) {
  const result: string[] = [];
  const pattern = new RegExp(stringLiteralPattern(), "g");
  let match: RegExpExecArray | null;
  while ((match = pattern.exec(block))) {
    result.push(match[1]);
  }
  return uniqueDependencyNames(result);
}

function stringLiteralPattern() {
  return "[\"'`]([^\"'`]+)[\"'`]";
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function runtimeEntryPlugin(): PluginOption {
  return {
    name: "dever-front-plugin-runtime-entry",
    resolveId(id) {
      return id === runtimeEntryID ? resolvedRuntimeEntryID : null;
    },
    load(id) {
      if (id !== resolvedRuntimeEntryID) {
        return null;
      }
      return [
        `import plugin from ${JSON.stringify(normalizePath(pluginEntry))}`,
        "window.DeverFront?.registerPlugin(plugin)",
      ].join("\n");
    },
  };
}

function compatModulePlugin(moduleScopedCompat: boolean): PluginOption {
  return {
    name: "dever-front-plugin-compat-modules",
    enforce: "pre",
    buildStart() {
      compatModuleSources.clear();
    },
    resolveId(id, _importer, options) {
      if (id.startsWith(compatModulePrefix)) {
        return {
          id: resolvedCompatModulePrefix + id.slice(compatModulePrefix.length),
          moduleSideEffects: false,
        };
      }
      if (id.startsWith("@/")) {
        if ((options as { scan?: boolean } | undefined)?.scan) {
          return id;
        }
        throw new Error(
          `[dever-front-plugin] 宿主模块 ${JSON.stringify(id)} 未经过兼容导入转换`,
        );
      }
      return null;
    },
    transform(code, id) {
      if (!isCompatSourceFile(id)) {
        return null;
      }
      const rewritten = rewriteCompatImports(code, id, {
        virtualModulePrefix: compatModulePrefix,
        rewriteCompatCalls: moduleScopedCompat,
        sdkModuleSource,
        sdkCoreSource: normalizePath(sdkCoreEntry),
        sdkCoreExports,
        sdkCompatExports,
        onCompatSource(source) {
          compatModuleSources.add(source);
        },
        onImplicitCompatSource(source) {
          throw new Error(
            `[dever-front-plugin] ${normalizePath(id.split("?", 1)[0])} 通过普通函数引用了宿主模块 ${JSON.stringify(source)}；请直接使用 getCompatModule。`,
          );
        },
      });
      if (!rewritten) {
        return null;
      }
      return {
        code: rewritten,
        map: null,
      };
    },
    load(id) {
      if (!id.startsWith(resolvedCompatModulePrefix)) {
        return null;
      }

      const source = id.slice(resolvedCompatModulePrefix.length);
      const missingModuleMessage = `[dever-front-plugin] 宿主未注册兼容模块 ${source}`;
      return [
        // Split bundles load each host module at the point its plugin chunk is evaluated.
        moduleScopedCompat
          ? `await window.DeverFront?.ensureCompat?.([${JSON.stringify(source)}])`
          : "",
        `const mod = window.DeverFront?.sdk?.getCompatModule(${JSON.stringify(source)})`,
        `if (!mod || Object.keys(mod).length === 0) { throw new Error(${JSON.stringify(missingModuleMessage)}) }`,
        "export default mod",
      ].join("\n");
    },
  };
}

function isCompatSourceFile(id: string) {
  if (compatSourceRoots.length === 0) {
    return false;
  }
  const cleanID = normalizePath(id.split("?", 1)[0]);
  return (
    !cleanID.includes("/node_modules/") &&
    compatSourceRoots.some((root) => cleanID.startsWith(`${root}/`)) &&
    /\.[cm]?[jt]sx?$/.test(cleanID)
  );
}

function frontPluginDependencySubpathPlugin(): PluginOption {
  return {
    name: "dever-front-plugin-dependency-subpaths",
    enforce: "pre",
    resolveId(source) {
      return resolveFrontPluginDependencySubpath(source) || null;
    },
    transform(code, id) {
      if (id.includes("/node_modules/")) {
        return null;
      }
      const rewritten = rewriteDependencySubpathImports(code);
      if (!rewritten) {
        return null;
      }
      return {
        code: rewritten,
        map: null,
      };
    },
  };
}

function pluginManifestMetadataPlugin(): PluginOption {
  return {
    name: "dever-front-plugin-manifest-metadata",
    apply: "build",
    closeBundle() {
      if (!pluginRoot) {
        return;
      }
      const manifestFile = path.join(pluginRoot, "dist", "manifest.json");
      const manifest = plainObject(readJSONFile(manifestFile));
      attachManifestCSSAssets(manifest, !splitPluginBundle);
      if (splitPluginBundle) {
        markManifestEntryAsModule(manifest);
      }
      attachManifestAssetVersions(manifest, path.dirname(manifestFile));
      manifest.__plugin = {
        ...readPluginMetadata(),
        compat: Array.from(compatModuleSources).sort(),
        ...(splitPluginBundle ? { compatMode: moduleCompatMode } : {}),
      };
      fs.writeFileSync(manifestFile, `${JSON.stringify(manifest, null, 2)}\n`);
    },
  };
}

function pluginChunkCSSPlugin(): PluginOption {
  return {
    name: "dever-front-plugin-chunk-css",
    apply: "build",
    enforce: "post",
    renderChunk(code, chunk) {
      const cssFiles = Array.from(chunk.viteMetadata?.importedCss || []);
      if (chunk.isEntry || cssFiles.length === 0) {
        return null;
      }

      const chunkDirectory = path.posix.dirname(normalizePath(chunk.fileName));
      const styleUrls = cssFiles.map((file) => {
        const relativeFile = path.posix.relative(
          chunkDirectory,
          normalizePath(file),
        );
        const specifier = relativeFile.startsWith(".")
          ? relativeFile
          : `./${relativeFile}`;
        return `new URL(${JSON.stringify(specifier)}, import.meta.url).href`;
      });
      const styleLoader = [
        "if (!window.DeverFront?.ensureStyles) {",
        `  throw new Error(${JSON.stringify("Dever front runtime does not support chunk styles")});`,
        "}",
        `await window.DeverFront.ensureStyles([${styleUrls.join(",")}]);`,
      ].join("\n");
      return {
        code: `${styleLoader}\n${code}`,
        map: null,
      };
    },
  };
}

function attachManifestCSSAssets(
  manifest: Record<string, unknown>,
  includeDynamicEntries: boolean,
) {
  const entries = manifestEntries(manifest);
  const entry = findManifestEntry(entries);
  if (!entry) {
    return;
  }

  const cssFiles = includeDynamicEntries
    ? manifestCSSAssets(entries)
    : normalizeStringList(entry.css);
  if (cssFiles.length > 0) {
    entry.css = cssFiles;
  }
}

function manifestCSSAssets(entries: Record<string, unknown>[]) {
  return uniqueDependencyNames([
    ...entries.flatMap((item) => normalizeStringList(item.css)),
    ...entries
      .map((item) => String(item.file || "").trim())
      .filter((file) => file.endsWith(".css")),
  ]);
}

function markManifestEntryAsModule(manifest: Record<string, unknown>) {
  const entry = findManifestEntry(manifestEntries(manifest));
  if (entry) {
    entry.module = true;
  }
}

function manifestEntries(manifest: Record<string, unknown>) {
  return Object.values(manifest)
    .map(plainObject)
    .filter((item) => Object.keys(item).length > 0);
}

function findManifestEntry(entries: Record<string, unknown>[]) {
  return (
    entries.find((item) => item.isEntry) ||
    entries.find((item) => String(item.file || "").endsWith(".js"))
  );
}

function attachManifestAssetVersions(
  manifest: Record<string, unknown>,
  outputRoot: string,
) {
  const versions = new Map<string, string>();
  const versionAsset = (value: unknown) => {
    const asset = String(value || "").trim();
    if (!asset) {
      return asset;
    }

    const relativeAsset = asset.split(/[?#]/, 1)[0];
    const assetFile = path.resolve(outputRoot, relativeAsset);
    const relativeFile = path.relative(outputRoot, assetFile);
    if (
      !relativeFile ||
      relativeFile.startsWith(`..${path.sep}`) ||
      path.isAbsolute(relativeFile) ||
      !fs.statSync(assetFile, { throwIfNoEntry: false })?.isFile()
    ) {
      return asset;
    }

    let version = versions.get(assetFile);
    if (!version) {
      version = createHash("sha256")
        .update(fs.readFileSync(assetFile))
        .digest("hex")
        .slice(0, 16);
      versions.set(assetFile, version);
    }
    return `${relativeAsset}?v=${version}`;
  };

  Object.values(manifest).forEach((rawEntry) => {
    const entry = plainObject(rawEntry);
    if (typeof entry.file === "string") {
      entry.file = versionAsset(entry.file);
    }
    if (Array.isArray(entry.css)) {
      entry.css = entry.css.map(versionAsset);
    }
  });
}

function normalizeStringList(value: unknown) {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.map((item) => String(item || "").trim()).filter(Boolean);
}

function dependency(name: string) {
  return path.resolve(compilerRoot, "node_modules", name);
}

function dependencyEntry(name: string) {
  return path.resolve(compilerRoot, "node_modules", ...name.split("/"));
}

function shimFile(name: string) {
  const file = shimModuleFiles[name];
  if (!file) {
    throw new Error(`Unknown front plugin shim: ${name}`);
  }
  return path.join(shimRoot, file);
}

function frontPluginDependencyAliases() {
  return frontPluginDependencyNames.map((name) => ({
    find: name,
    replacement: dependency(name),
  }));
}

function runtimeAlias() {
  return [
    { find: "@dever/front-plugin", replacement: sdkEntry },
    {
      find: /^zustand$/,
      replacement: dependencyEntry("zustand/esm/index.mjs"),
    },
    {
      find: /^zustand\/vanilla$/,
      replacement: dependencyEntry("zustand/esm/vanilla.mjs"),
    },
    {
      find: /^zustand\/react$/,
      replacement: dependencyEntry("zustand/esm/react.mjs"),
    },
    {
      find: "react/jsx-dev-runtime",
      replacement: shimFile("react-jsx-runtime"),
    },
    {
      find: "react/jsx-runtime",
      replacement: shimFile("react-jsx-runtime"),
    },
    {
      find: "react-dom/client",
      replacement: shimFile("react-dom-client"),
    },
    {
      find: "react-dom",
      replacement: shimFile("react-dom"),
    },
    {
      find: "react",
      replacement: shimFile("react"),
    },
    { find: "@xyflow/react", replacement: dependency("@xyflow/react") },
    { find: "lucide-react", replacement: dependency("lucide-react") },
    { find: "sonner", replacement: dependency("sonner") },
    ...frontPluginDependencyAliases(),
  ];
}

export default defineConfig(({ command }) => {
  const nodeEnv = command === "serve" ? "development" : "production";
  return {
    // Avoid recursively watching the backend; /@fs imports are watched separately.
    root: compilerRoot,
    define: {
      "process.env.NODE_ENV": JSON.stringify(nodeEnv),
    },
    plugins: [
      frontPluginDependencySubpathPlugin(),
      runtimeEntryPlugin(),
      compatModulePlugin(command === "serve" || splitPluginBundle),
      pluginManifestMetadataPlugin(),
      react(),
      ...(splitPluginBundle ? [pluginChunkCSSPlugin()] : []),
    ],
    resolve: {
      dedupe: ["react", "react-dom"],
      alias: runtimeAlias(),
    },
    server: {
      host: "127.0.0.1",
      hmr: false,
      fs: {
        allow: devServerAllowedRoots,
      },
    },
    optimizeDeps: {
      include: optimizedPluginDependencyNames,
    },
    build: {
      outDir: pluginRoot ? path.join(pluginRoot, "dist") : "dist",
      emptyOutDir: true,
      manifest: "manifest.json",
      // Split plugins keep feature CSS with the dynamic feature chunk. The
      // legacy single bundle still exposes one stylesheet through the entry.
      cssCodeSplit: splitPluginBundle,
      lib: {
        entry: runtimeEntryFile,
        formats: splitPluginBundle ? ["es"] : ["iife"],
        name: `${pluginName.replace(/[^a-zA-Z0-9_$]/g, "_")}FrontPlugin`,
        fileName: () => `${pluginName}.js`,
        cssFileName: pluginName,
      },
      rollupOptions: {
        external: splitPluginBundle ? [] : ["react"],
        output: splitPluginBundle
          ? {
              inlineDynamicImports: false,
              // Preserve feature boundaries while coalescing tiny shared chunks.
              experimentalMinChunkSize: splitPluginMinChunkSize,
              manualChunks: pluginManualChunk,
              onlyExplicitManualChunks: true,
              chunkFileNames: "assets/[name]-[hash].js",
              assetFileNames: "assets/[name]-[hash][extname]",
            }
          : {
              globals: {
                react: "React",
              },
              inlineDynamicImports: true,
            },
      },
    },
  };
});
