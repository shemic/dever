import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react-swc";
import { defineConfig, normalizePath, type PluginOption } from "vite";
import { rewriteCompatImports, type CompatLoadMode } from "./src/compat-import";
import { bundleAuditPlugin } from "./src/bundle-audit";
import {
  readPluginBundlePolicy,
  type PluginBundlePolicy,
} from "./src/plugin-bundle-policy";
import {
  pluginManifestPlugin,
  type PluginManifestMetadata,
} from "./src/plugin-manifest";
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
const { coreExports: sdkCoreExports, compatExports: sdkCompatExports } =
  readFrontSDKExports(sdkCoreEntry, sdkEntry);
const compatSourceRoots = Array.from(
  new Set([...pluginSourceRoots, normalizePath(path.dirname(sdkEntry))]),
);
const shimRoot = path.resolve(compilerRoot, "src", "shims");
const runtimeEntryFile = path.resolve(compilerRoot, "src", "runtime-entry.ts");

const runtimeEntryID = "virtual:dever-front-plugin-runtime";
const resolvedRuntimeEntryID = "\0" + runtimeEntryID;
const pluginEntry = pluginRoot ? path.join(pluginRoot, "src", "plugin.ts") : "";
const rawManifestFile = ".vite/manifest.raw.json";
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
const sdkModuleSource = "@dever/front-plugin";

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

function resolvePluginOutputRoot(command: string) {
  if (command !== "build") {
    return pluginRoot ? path.join(pluginRoot, "dist") : "dist";
  }
  if (!pluginRoot) {
    return path.join(compilerRoot, "dist");
  }

  const configured = (process.env.DEVER_FRONT_PLUGIN_OUT_DIR || "").trim();
  if (!configured) {
    throw new Error(
      "[dever-front-plugin] 生产构建必须通过 dever front build 使用 staging 输出目录",
    );
  }
  const resolvedPluginRoot = path.resolve(pluginRoot);
  const outputRoot = path.resolve(configured);
  if (
    path.dirname(outputRoot) !== resolvedPluginRoot ||
    !/^\.dist-next-\d+$/.test(path.basename(outputRoot))
  ) {
    throw new Error(
      `[dever-front-plugin] 非法 staging 输出目录: ${outputRoot}`,
    );
  }
  return outputRoot;
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

function packageNameFromModuleID(id: string) {
  const normalizedID = normalizePath(id);
  const nodeModulesMarker = "/node_modules/";
  const markerIndex = normalizedID.lastIndexOf(nodeModulesMarker);
  if (markerIndex === -1) {
    return "";
  }

  const packagePath = normalizedID.slice(
    markerIndex + nodeModulesMarker.length,
  );
  const segments = packagePath.split("/").filter(Boolean);
  if (segments[0]?.startsWith("@")) {
    return segments.length >= 2 ? `${segments[0]}/${segments[1]}` : "";
  }
  return segments[0] || "";
}

type ProtectedPluginChunk = {
  name: string;
  token: string;
};

function protectedPluginChunks(policy: PluginBundlePolicy) {
  const tokens = Array.from(
    new Set(
      Object.values(policy.budget?.roots || {}).flatMap(
        (root) => root.forbid || [],
      ).map((token) => normalizePath(token)),
    ),
  )
    .filter(isProtectableSourceSuffix)
    .sort(
      (left, right) => right.length - left.length || left.localeCompare(right),
    );
  return tokens.map((token, index) => ({
    token,
    name: `protected-${index + 1}-${safeChunkName(token)}`,
  }));
}

function isProtectableSourceSuffix(value: string) {
  return value.startsWith("/") && /\.[cm]?[jt]sx?$/.test(value);
}

function pluginManualChunk(
  id: string,
  protectedChunks: readonly ProtectedPluginChunk[],
) {
  if (packageNameFromModuleID(id) === "lucide-react") {
    return "vendor-icons";
  }

  const normalizedID = normalizePath(id.split("?", 1)[0]);
  const isPluginSource = pluginSourceRoots.some((root) =>
    normalizedID.startsWith(`${root}/`),
  );
  if (!isPluginSource || !/\.[cm]?[jt]sx?$/.test(normalizedID)) {
    return undefined;
  }
  return protectedChunks.find(({ token }) => normalizedID.endsWith(token))
    ?.name;
}

function safeChunkName(value: string) {
  const normalized = value
    .replace(/[^a-zA-Z0-9_-]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return normalized || "feature";
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

function readPluginMetadata(): PluginManifestMetadata {
  const fallback: PluginManifestMetadata = { name: pluginName };
  if (!pluginEntry || !fs.existsSync(pluginEntry)) {
    return fallback;
  }

  const content = fs.readFileSync(pluginEntry, "utf8");
  const metadata: PluginManifestMetadata = {
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

function compatImportPlugin(loadMode: CompatLoadMode): PluginOption {
  return {
    name: "dever-front-plugin-compat-imports",
    enforce: "pre",
    buildStart() {
      compatModuleSources.clear();
    },
    resolveId(id, _importer, options) {
      if (loadMode === "virtual" && id.startsWith(compatModulePrefix)) {
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
        loadMode,
        ...(loadMode === "virtual"
          ? { virtualModulePrefix: compatModulePrefix }
          : {}),
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
      if (
        loadMode !== "virtual" ||
        !id.startsWith(resolvedCompatModulePrefix)
      ) {
        return null;
      }

      const source = id.slice(resolvedCompatModulePrefix.length);
      const missingModuleMessage = `[dever-front-plugin] 宿主未注册兼容模块 ${source}`;
      return [
        `await window.DeverFront?.ensureCompat?.([${JSON.stringify(source)}])`,
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
  const pluginBundlePolicy = readPluginBundlePolicy(pluginRoot, {
    includeBudget: command === "build",
  });
  const splitPluginBundle = pluginBundlePolicy.mode === "split";
  const protectedChunks = protectedPluginChunks(pluginBundlePolicy);
  const compatLoadMode: CompatLoadMode =
    command === "serve"
      ? "virtual"
      : splitPluginBundle
        ? "module"
        : "preloaded";
  const outputRoot = resolvePluginOutputRoot(command);
  return {
    // Avoid recursively watching the backend; /@fs imports are watched separately.
    root: compilerRoot,
    define: {
      "process.env.NODE_ENV": JSON.stringify(nodeEnv),
    },
    plugins: [
      frontPluginDependencySubpathPlugin(),
      runtimeEntryPlugin(),
      compatImportPlugin(compatLoadMode),
      react(),
      ...(command === "build" && splitPluginBundle
        ? [pluginChunkCSSPlugin()]
        : []),
      ...(command === "build"
        ? [
            bundleAuditPlugin({
              label: pluginName,
              budget: pluginBundlePolicy.budget,
            }),
            pluginManifestPlugin({
              outputRoot,
              rawManifestFile,
              splitBundle: splitPluginBundle,
              readMetadata: readPluginMetadata,
              readCompatSources: () => Array.from(compatModuleSources),
            }),
          ]
        : []),
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
      outDir: outputRoot,
      emptyOutDir: true,
      manifest: rawManifestFile,
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
              // Keep pure shared icons together and protect configured lazy
              // feature boundaries from experimental small-chunk merging.
              // A real static import still creates an audited eager edge.
              manualChunks: (id) => pluginManualChunk(id, protectedChunks),
              onlyExplicitManualChunks: true,
              experimentalMinChunkSize: splitPluginMinChunkSize,
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
