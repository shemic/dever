export type BundleRootBudget = {
  facadeSuffix: string;
  maxJs: number;
  maxCss: number;
  maxBytes?: number;
  forbid?: string[];
};

export type BundleBudget = {
  maxJs: number;
  maxCss: number;
  maxDynamicEntries?: number;
  tinyJsBytes: number;
  maxTinyJs: number;
  allowTinyEntryNames?: string[];
  roots?: Record<string, BundleRootBudget>;
};

export type BundleAuditOptions = {
  label: string;
  budget?: BundleBudget;
};

type BundleOutputAsset = {
  type: "asset";
  fileName: string;
  name?: string;
  source: string | Uint8Array;
};

type BundleOutputChunk = {
  type: "chunk";
  code: string;
  facadeModuleId: string | null;
  fileName: string;
  imports: string[];
  isDynamicEntry: boolean;
  isEntry: boolean;
  moduleIds: string[];
  name: string;
};

type BundleOutput = BundleOutputAsset | BundleOutputChunk;
type BundleOutputMap = Record<string, BundleOutput>;

type BundleAuditPluginContext = {
  error: (message: string) => never;
  info: (message: string) => void;
};

export type BundleAuditPlugin = {
  name: string;
  apply: "build";
  generateBundle: (
    this: BundleAuditPluginContext,
    outputOptions: unknown,
    bundle: BundleOutputMap,
  ) => void;
};

type AuditedChunk = BundleOutputChunk & {
  viteMetadata?: {
    importedCss?: Set<string>;
  };
};

type StaticClosure = {
  chunks: AuditedChunk[];
  css: Set<string>;
  paths: Map<string, AuditedChunk[]>;
};

export function validateBundleBudget(
  value: unknown,
  configPath = "bundleBudget",
): BundleBudget {
  const budget = requireRecord(value, configPath);
  const maxJs = requireInteger(budget.maxJs, `${configPath}.maxJs`);
  const maxCss = requireInteger(budget.maxCss, `${configPath}.maxCss`);
  const tinyJsBytes = requireInteger(
    budget.tinyJsBytes,
    `${configPath}.tinyJsBytes`,
  );
  const maxTinyJs = requireInteger(budget.maxTinyJs, `${configPath}.maxTinyJs`);
  if (maxTinyJs > maxJs) {
    throw new Error(
      `[dever bundle] ${configPath}.maxTinyJs 不能大于 ${configPath}.maxJs`,
    );
  }

  const roots = readRootBudgets(budget.roots, `${configPath}.roots`);
  const result: BundleBudget = {
    maxJs,
    maxCss,
    tinyJsBytes,
    maxTinyJs,
  };
  if (budget.maxDynamicEntries !== undefined) {
    result.maxDynamicEntries = requireInteger(
      budget.maxDynamicEntries,
      `${configPath}.maxDynamicEntries`,
    );
  }
  if (budget.allowTinyEntryNames !== undefined) {
    result.allowTinyEntryNames = requireStringList(
      budget.allowTinyEntryNames,
      `${configPath}.allowTinyEntryNames`,
    );
  }
  if (Object.keys(roots).length > 0) {
    result.roots = roots;
  }
  return result;
}

export function bundleAuditPlugin(
  options: BundleAuditOptions,
): BundleAuditPlugin {
  const label = options.label.trim() || "bundle";
  return {
    name: `dever-bundle-audit-${safePluginName(label)}`,
    apply: "build",
    generateBundle(_outputOptions, bundle) {
      const report = auditBundle(bundle, label, options.budget);
      this.info(report.summary);
      if (process.env.DEVER_FRONT_BUNDLE_REPORT === "verbose") {
        report.details.forEach((line) => this.info(line));
      }
      if (report.errors.length > 0) {
        this.error(report.errors.join("\n"));
      }
    },
  };
}

function auditBundle(
  bundle: BundleOutputMap,
  label: string,
  budget: BundleBudget | undefined,
) {
  const outputs = Object.values(bundle);
  const chunks = outputs.filter(isOutputChunk);
  const chunksByFile = new Map(chunks.map((chunk) => [chunk.fileName, chunk]));
  const jsOutputs = outputs.filter((output) =>
    isAssetType(output.fileName, "js"),
  );
  const cssOutputs = outputs.filter((output) =>
    isAssetType(output.fileName, "css"),
  );
  const dynamicEntries = chunks.filter((chunk) => chunk.isDynamicEntry);
  const tinyLimit = budget?.tinyJsBytes ?? 2048;
  const allowedTinyNames = new Set(budget?.allowTinyEntryNames || []);
  const tinyJs = jsOutputs.filter(
    (output) =>
      !isEntryChunk(output) &&
      !isAllowedTinyOutput(output, allowedTinyNames) &&
      outputBytes(output) < tinyLimit,
  );
  const errors: string[] = [];
  const details: string[] = [];

  for (const chunk of chunks) {
    // Rollup may emit a valid facade that only imports another chunk and owns
    // no modules. The emitted code, rather than module ownership, determines
    // whether the output is actually empty.
    if (chunk.code.trim() === "") {
      errors.push(
        `[dever bundle] ${label}: empty chunk ${describeChunk(chunk)}`,
      );
    }
    const compatModule = chunk.moduleIds.find((moduleID) =>
      normalizeModuleID(moduleID).includes("virtual:dever-front-compat:"),
    );
    if (compatModule) {
      errors.push(
        `[dever bundle] ${label}: production compat virtual chunk ${describeChunk(chunk)} (${normalizeModuleID(compatModule)})`,
      );
    }
  }

  findStaticCycles(chunks, chunksByFile).forEach((cycle) => {
    errors.push(
      `[dever bundle] ${label}: static chunk cycle: ${cycle.map(describeChunk).join(" -> ")}`,
    );
  });

  if (budget) {
    enforceMaximum(errors, label, "JS files", jsOutputs.length, budget.maxJs);
    enforceMaximum(
      errors,
      label,
      "CSS files",
      cssOutputs.length,
      budget.maxCss,
    );
    if (budget.maxDynamicEntries !== undefined) {
      enforceMaximum(
        errors,
        label,
        "dynamic entries",
        dynamicEntries.length,
        budget.maxDynamicEntries,
      );
    }
    if (tinyJs.length > budget.maxTinyJs) {
      errors.push(
        `[dever bundle] ${label}: ${tinyJs.length} non-entry JS files are below ${tinyLimit} bytes (max ${budget.maxTinyJs})\n${tinyJs.map(describeTinyOutput).join("\n")}`,
      );
    }
  }

  const rootSummaries: string[] = [];
  for (const [rootName, rootBudget] of Object.entries(budget?.roots || {})) {
    const matches = findRootChunks(chunks, rootBudget.facadeSuffix);
    if (matches.length !== 1) {
      errors.push(
        `[dever bundle] ${label}/${rootName}: expected exactly one facade or owned module ending with ${JSON.stringify(rootBudget.facadeSuffix)}, found ${matches.length}`,
      );
      continue;
    }

    const root = matches[0];
    const closure = collectStaticClosure(root, chunksByFile);
    const bytes = staticClosureBytes(closure, bundle);
    rootSummaries.push(
      `${rootName}=${closure.chunks.length}JS/${closure.css.size}CSS/${formatBytes(bytes)}`,
    );
    details.push(
      `[dever bundle] ${label}/${rootName}: static closure ${closure.chunks.map((chunk) => chunk.fileName).join(", ")}`,
    );
    enforceMaximum(
      errors,
      `${label}/${rootName}`,
      "static JS files",
      closure.chunks.length,
      rootBudget.maxJs,
    );
    enforceMaximum(
      errors,
      `${label}/${rootName}`,
      "static CSS files",
      closure.css.size,
      rootBudget.maxCss,
    );
    if (rootBudget.maxBytes !== undefined) {
      enforceMaximum(
        errors,
        `${label}/${rootName}`,
        "static bytes",
        bytes,
        rootBudget.maxBytes,
      );
    }
    for (const token of rootBudget.forbid || []) {
      for (const chunk of closure.chunks) {
        if (!chunkMatchesToken(chunk, token)) {
          continue;
        }
        const chain = closure.paths.get(chunk.fileName) || [root, chunk];
        errors.push(
          `[dever bundle] ${label}/${rootName}: forbidden chunk ${JSON.stringify(token)} is eager: ${chain.map(describeChunk).join(" -> ")}`,
        );
      }
    }
  }

  details.push(
    ...outputs
      .filter((output) =>
        ["js", "css"].some((extension) =>
          isAssetType(output.fileName, extension),
        ),
      )
      .sort((left, right) => left.fileName.localeCompare(right.fileName))
      .map(
        (output) => describeOutput(output, label),
      ),
  );

  const rootSummary = rootSummaries.length
    ? `; roots ${rootSummaries.join(", ")}`
    : "";
  return {
    summary: `[dever bundle] ${label}: ${jsOutputs.length} JS, ${cssOutputs.length} CSS, ${dynamicEntries.length} dynamic, ${tinyJs.length} tiny${rootSummary}`,
    details,
    errors,
  };
}

function readRootBudgets(value: unknown, configPath: string) {
  if (value === undefined) {
    return {};
  }
  const roots = requireRecord(value, configPath);
  const result: Record<string, BundleRootBudget> = {};
  const facadePaths = new Map<string, string>();
  for (const [name, rawRoot] of Object.entries(roots)) {
    const rootPath = `${configPath}.${name}`;
    const root = requireRecord(rawRoot, rootPath);
    const facadeSuffix = requireString(
      root.facadeSuffix,
      `${rootPath}.facadeSuffix`,
    ).replaceAll("\\", "/");
    if (!facadeSuffix.startsWith("/")) {
      throw new Error(`[dever bundle] ${rootPath}.facadeSuffix 必须以 / 开头`);
    }
    const existingPath = facadePaths.get(facadeSuffix);
    if (existingPath) {
      throw new Error(
        `[dever bundle] ${rootPath}.facadeSuffix 与 ${existingPath}.facadeSuffix 重复`,
      );
    }
    facadePaths.set(facadeSuffix, rootPath);

    const rootBudget: BundleRootBudget = {
      facadeSuffix,
      maxJs: requireInteger(root.maxJs, `${rootPath}.maxJs`),
      maxCss: requireInteger(root.maxCss, `${rootPath}.maxCss`),
    };
    if (root.maxBytes !== undefined) {
      rootBudget.maxBytes = requireInteger(
        root.maxBytes,
        `${rootPath}.maxBytes`,
      );
    }
    if (root.forbid !== undefined) {
      rootBudget.forbid = requireStringList(root.forbid, `${rootPath}.forbid`);
    }
    result[name] = rootBudget;
  }
  return result;
}

function requireRecord(value: unknown, configPath: string) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`[dever bundle] ${configPath} 必须是对象`);
  }
  return value as Record<string, unknown>;
}

function requireInteger(value: unknown, configPath: string) {
  if (
    typeof value !== "number" ||
    !Number.isFinite(value) ||
    !Number.isInteger(value) ||
    value < 0
  ) {
    throw new Error(`[dever bundle] ${configPath} 必须是非负整数`);
  }
  return value;
}

function requireString(value: unknown, configPath: string) {
  if (typeof value !== "string" || value.trim() === "") {
    throw new Error(`[dever bundle] ${configPath} 必须是非空字符串`);
  }
  return value.trim();
}

function requireStringList(value: unknown, configPath: string) {
  if (!Array.isArray(value)) {
    throw new Error(`[dever bundle] ${configPath} 必须是字符串数组`);
  }
  return Array.from(
    new Set(
      value.map((item, index) =>
        requireString(item, `${configPath}[${index}]`),
      ),
    ),
  );
}

function isOutputChunk(output: BundleOutput): output is AuditedChunk {
  return output.type === "chunk";
}

function isEntryChunk(output: BundleOutput) {
  return output.type === "chunk" && (output.isEntry || output.isDynamicEntry);
}

function isAssetType(fileName: string, extension: string) {
  return fileName.toLowerCase().endsWith(`.${extension}`);
}

function outputBytes(output: BundleOutput) {
  if (output.type === "chunk") {
    return Buffer.byteLength(output.code);
  }
  return typeof output.source === "string"
    ? Buffer.byteLength(output.source)
    : output.source.byteLength;
}

function isAllowedTinyOutput(output: BundleOutput, allowedNames: Set<string>) {
  const fileStem = output.fileName
    .split("/")
    .at(-1)
    ?.replace(/-[A-Za-z0-9_-]+\.js$/, "");
  return (
    allowedNames.has(output.name || "") ||
    allowedNames.has(fileStem || "") ||
    (output.type === "chunk" && allowedNames.has(output.name))
  );
}

function describeTinyOutput(output: BundleOutput) {
  const modules =
    output.type === "chunk"
      ? output.moduleIds.map(normalizeModuleID).join(", ") || "<empty>"
      : "<emitted asset>";
  return `  - ${output.fileName} (${outputBytes(output)} bytes): ${modules}`;
}

function describeOutput(output: BundleOutput, label: string) {
  const base = `[dever bundle] ${label}: ${output.fileName} ${formatBytes(outputBytes(output))}`;
  if (output.type !== "chunk") {
    return base;
  }
  const kind = output.isEntry
    ? "entry"
    : output.isDynamicEntry
      ? "dynamic"
      : "shared";
  const facade = normalizedFacade(output) || "<none>";
  return `${base} ${kind} facade=${facade} modules=${output.moduleIds.length}`;
}

function normalizedFacade(chunk: AuditedChunk) {
  return normalizeModuleID(chunk.facadeModuleId || "");
}

function findRootChunks(chunks: AuditedChunk[], moduleSuffix: string) {
  const facadeMatches = chunks.filter((chunk) =>
    normalizedFacade(chunk).endsWith(moduleSuffix),
  );
  if (facadeMatches.length > 0) {
    return facadeMatches;
  }
  return chunks.filter((chunk) =>
    chunk.moduleIds.some((moduleID) =>
      normalizeModuleID(moduleID).endsWith(moduleSuffix),
    ),
  );
}

function collectStaticClosure(
  root: AuditedChunk,
  chunksByFile: Map<string, AuditedChunk>,
): StaticClosure {
  const chunks: AuditedChunk[] = [];
  const css = new Set<string>();
  const paths = new Map<string, AuditedChunk[]>([[root.fileName, [root]]]);
  const queue = [root];
  const seen = new Set<string>();
  while (queue.length > 0) {
    const chunk = queue.shift()!;
    if (seen.has(chunk.fileName)) {
      continue;
    }
    seen.add(chunk.fileName);
    chunks.push(chunk);
    for (const cssFile of chunk.viteMetadata?.importedCss || []) {
      css.add(cssFile);
    }
    for (const importFile of chunk.imports) {
      const imported = chunksByFile.get(importFile);
      if (!imported) {
        continue;
      }
      if (!paths.has(imported.fileName)) {
        paths.set(imported.fileName, [
          ...(paths.get(chunk.fileName) || [root]),
          imported,
        ]);
      }
      queue.push(imported);
    }
  }
  return { chunks, css, paths };
}

function staticClosureBytes(closure: StaticClosure, bundle: BundleOutputMap) {
  const chunkBytes = closure.chunks.reduce(
    (total, chunk) => total + outputBytes(chunk),
    0,
  );
  const cssBytes = Array.from(closure.css).reduce((total, fileName) => {
    const output = bundle[fileName];
    return total + (output ? outputBytes(output) : 0);
  }, 0);
  return chunkBytes + cssBytes;
}

function findStaticCycles(
  chunks: AuditedChunk[],
  chunksByFile: Map<string, AuditedChunk>,
) {
  const state = new Map<string, "visiting" | "visited">();
  const stack: AuditedChunk[] = [];
  const cycles = new Map<string, AuditedChunk[]>();

  const visit = (chunk: AuditedChunk) => {
    state.set(chunk.fileName, "visiting");
    stack.push(chunk);
    for (const importFile of chunk.imports) {
      const imported = chunksByFile.get(importFile);
      if (!imported) {
        continue;
      }
      if (state.get(importFile) === "visiting") {
        const cycleStart = stack.findIndex(
          (candidate) => candidate.fileName === importFile,
        );
        const cycle = [...stack.slice(cycleStart), imported];
        cycles.set(canonicalCycleKey(cycle), cycle);
        continue;
      }
      if (!state.has(importFile)) {
        visit(imported);
      }
    }
    stack.pop();
    state.set(chunk.fileName, "visited");
  };

  chunks.forEach((chunk) => {
    if (!state.has(chunk.fileName)) {
      visit(chunk);
    }
  });
  return Array.from(cycles.values());
}

function canonicalCycleKey(cycle: AuditedChunk[]) {
  const names = cycle.slice(0, -1).map((chunk) => chunk.fileName);
  if (names.length === 0) {
    return "";
  }
  const rotations = names.map((_, index) => [
    ...names.slice(index),
    ...names.slice(0, index),
  ]);
  return rotations.map((parts) => parts.join("\0")).sort()[0];
}

function chunkMatchesToken(chunk: AuditedChunk, token: string) {
  return [chunk.name, chunk.fileName, ...chunk.moduleIds]
    .map(normalizeModuleID)
    .some((candidate) => candidate.includes(token));
}

function describeChunk(chunk: AuditedChunk) {
  return chunk.name || chunk.fileName;
}

function enforceMaximum(
  errors: string[],
  label: string,
  metric: string,
  actual: number,
  maximum: number,
) {
  if (actual > maximum) {
    errors.push(
      `[dever bundle] ${label}: ${metric} ${actual} exceeds max ${maximum}`,
    );
  }
}

function formatBytes(bytes: number) {
  if (bytes < 1024) {
    return `${bytes}B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)}KiB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(2)}MiB`;
}

function safePluginName(value: string) {
  return value.replace(/[^a-zA-Z0-9_-]+/g, "-");
}

function normalizeModuleID(value: string) {
  return value.replaceAll("\\", "/");
}
