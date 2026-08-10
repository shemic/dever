import fs from "node:fs";
import path from "node:path";
import { validateBundleBudget, type BundleBudget } from "./bundle-audit";

export type PluginBundleMode = "single" | "split";

export type PluginBundlePolicy = {
  mode: PluginBundleMode;
  budget?: BundleBudget;
};

type ReadPluginBundlePolicyOptions = {
  includeBudget?: boolean;
};

export function readPluginBundlePolicy(
  pluginRoot: string,
  options: ReadPluginBundlePolicyOptions = {},
): PluginBundlePolicy {
  if (!pluginRoot) {
    return { mode: "single" };
  }
  const packageFile = path.join(pluginRoot, "package.json");
  const packageStat = fs.statSync(packageFile, { throwIfNoEntry: false });
  if (!packageStat) {
    return { mode: "single" };
  }
  if (!packageStat.isFile()) {
    throw new Error(`[dever bundle] ${packageFile} 必须是文件`);
  }
  const manifest = readJSONRecord(packageFile);
  const dever = readOptionalRecord(manifest.dever, `${packageFile}#dever`);
  const mode = dever.bundle === "split" ? "split" : "single";
  if (options.includeBudget === false || dever.bundleBudget === undefined) {
    return { mode };
  }
  return {
    mode,
    budget: validateBundleBudget(
      dever.bundleBudget,
      `${packageFile}#dever.bundleBudget`,
    ),
  };
}

function readJSONRecord(file: string) {
  let value: unknown;
  try {
    value = JSON.parse(fs.readFileSync(file, "utf8"));
  } catch (error) {
    throw new Error(
      `[dever bundle] 无法读取 ${file}: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
  return readOptionalRecord(value, file);
}

function readOptionalRecord(value: unknown, configPath: string) {
  if (value === undefined) {
    return {} as Record<string, unknown>;
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`[dever bundle] ${configPath} 必须是对象`);
  }
  return value as Record<string, unknown>;
}
