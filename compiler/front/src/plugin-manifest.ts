import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import type { PluginOption } from "vite";

export type PluginManifestMetadata = {
  name: string;
  compat?: string[];
  nodes?: string[];
  depends?: string[];
};

type PluginManifestOptions = {
  outputRoot: string;
  rawManifestFile: string;
  splitBundle: boolean;
  readMetadata: () => PluginManifestMetadata;
  readCompatSources: () => string[];
};

export function pluginManifestPlugin(
  options: PluginManifestOptions,
): PluginOption {
  return {
    name: "dever-front-plugin-manifest",
    apply: "build",
    closeBundle() {
      publishPluginManifest(options);
    },
  };
}

function publishPluginManifest(options: PluginManifestOptions) {
  const outputRoot = path.resolve(options.outputRoot);
  const rawManifest = path.resolve(outputRoot, options.rawManifestFile);
  assertInsideOutputRoot(outputRoot, rawManifest);
  const manifest = readManifest(rawManifest);

  attachManifestCSSAssets(manifest, !options.splitBundle);
  if (options.splitBundle) {
    markManifestEntryAsModule(manifest);
  }
  attachManifestAssetVersions(manifest, outputRoot);
  manifest.__plugin = {
    ...options.readMetadata(),
    compat: Array.from(new Set(options.readCompatSources())).sort(),
    ...(options.splitBundle ? { compatMode: "module" } : {}),
  };

  const finalManifest = path.join(outputRoot, "manifest.json");
  const temporaryManifest = path.join(
    outputRoot,
    `.manifest.json.tmp-${process.pid}-${Date.now()}`,
  );
  try {
    fs.writeFileSync(
      temporaryManifest,
      `${JSON.stringify(manifest, null, 2)}\n`,
      { flag: "wx" },
    );
    fs.renameSync(temporaryManifest, finalManifest);
    fs.rmSync(rawManifest);
    removeDirectoryIfEmpty(path.dirname(rawManifest));
  } finally {
    fs.rmSync(temporaryManifest, { force: true });
  }
}

function readManifest(file: string) {
  let value: unknown;
  try {
    value = JSON.parse(fs.readFileSync(file, "utf8"));
  } catch (error) {
    throw new Error(
      `[dever-front-plugin] 读取 Vite manifest 失败: ${file}: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`[dever-front-plugin] Vite manifest 必须是对象: ${file}`);
  }
  return value as Record<string, unknown>;
}

function attachManifestCSSAssets(
  manifest: Record<string, unknown>,
  includeDynamicEntries: boolean,
) {
  const entries = manifestEntries(manifest);
  const entry = findManifestEntry(entries);
  if (!entry) {
    throw new Error("[dever-front-plugin] Vite manifest 缺少插件入口");
  }
  const cssFiles = includeDynamicEntries
    ? manifestCSSAssets(entries)
    : normalizeStringList(entry.css);
  if (cssFiles.length > 0) {
    entry.css = cssFiles;
  }
}

function manifestCSSAssets(entries: Record<string, unknown>[]) {
  return uniqueStrings([
    ...entries.flatMap((item) => normalizeStringList(item.css)),
    ...entries
      .map((item) => String(item.file || "").trim())
      .filter((file) => file.endsWith(".css")),
  ]);
}

function markManifestEntryAsModule(manifest: Record<string, unknown>) {
  const entry = findManifestEntry(manifestEntries(manifest));
  if (!entry) {
    throw new Error("[dever-front-plugin] Vite manifest 缺少 module 入口");
  }
  entry.module = true;
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
    if (!isInside(outputRoot, assetFile)) {
      throw new Error(
        `[dever-front-plugin] manifest asset 越过输出目录: ${asset}`,
      );
    }
    if (!fs.statSync(assetFile, { throwIfNoEntry: false })?.isFile()) {
      throw new Error(
        `[dever-front-plugin] manifest asset 不存在: ${relativeAsset}`,
      );
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

function assertInsideOutputRoot(outputRoot: string, file: string) {
  if (!isInside(outputRoot, file)) {
    throw new Error(`[dever-front-plugin] manifest 路径越过输出目录: ${file}`);
  }
}

function isInside(root: string, target: string) {
  const relative = path.relative(root, target);
  return (
    relative !== "" &&
    !relative.startsWith(`..${path.sep}`) &&
    !path.isAbsolute(relative)
  );
}

function removeDirectoryIfEmpty(directory: string) {
  try {
    fs.rmdirSync(directory);
  } catch (error) {
    const code = (error as NodeJS.ErrnoException).code;
    if (code !== "ENOTEMPTY" && code !== "ENOENT") {
      throw error;
    }
  }
}

function plainObject(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return {};
  }
  return value as Record<string, unknown>;
}

function normalizeStringList(value: unknown) {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.map((item) => String(item || "").trim()).filter(Boolean);
}

function uniqueStrings(values: string[]) {
  return Array.from(new Set(values)).sort();
}
